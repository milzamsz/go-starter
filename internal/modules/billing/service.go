package billing

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v81"
	stripeSession "github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	billingPortalSession "github.com/stripe/stripe-go/v81/billingportal/session"
	"github.com/stripe/stripe-go/v81/subscription"

	"github.com/milzam/go-starter/internal/config"
	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/sqlc"
)

// Service coordinates all billing integrations with Stripe.
type Service struct {
	queries *sqlc.Queries
	db      *pgxpool.Pool
	logger  *slog.Logger
	cfg     config.StripeConfig
}

// NewService creates a new billing Service and sets the Stripe API key.
func NewService(queries *sqlc.Queries, db *pgxpool.Pool, logger *slog.Logger, cfg config.StripeConfig) *Service {
	stripe.Key = cfg.SecretKey
	return &Service{
		queries: queries,
		db:      db,
		logger:  logger.With("component", "billing.Service"),
		cfg:     cfg,
	}
}

// CreateOrGetCustomer retrieves the Stripe customer ID for a user or creates a new one.
func (s *Service) CreateOrGetCustomer(ctx context.Context, userID uuid.UUID, email string) (string, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for customer ID check", "user_id", userID, "error", err)
		return "", appErrors.NewInternal("failed to check billing status", err)
	}

	if user.StripeCustomerID != nil && *user.StripeCustomerID != "" {
		return *user.StripeCustomerID, nil
	}

	// Create a new customer in Stripe.
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(user.Name),
		Metadata: map[string]string{
			"user_id": userID.String(),
		},
	}

	stripeCust, err := customer.New(params)
	if err != nil {
		s.logger.ErrorContext(ctx, "creating Stripe customer", "user_id", userID, "error", err)
		return "", appErrors.NewInternal("failed to register customer on billing gateway", err)
	}

	// Sync Stripe customer ID back to DB.
	err = s.queries.UpdateUserBilling(ctx, sqlc.UpdateUserBillingParams{
		ID:                 userID,
		StripeCustomerID:   &stripeCust.ID,
		Plan:               user.Plan,
		SubscriptionStatus: user.SubscriptionStatus,
		SubscriptionID:     user.SubscriptionID,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "saving Stripe customer ID to DB", "user_id", userID, "error", err)
		return "", appErrors.NewInternal("failed to synchronize billing records", err)
	}

	s.logger.InfoContext(ctx, "Stripe customer synchronized", "user_id", userID, "customer_id", stripeCust.ID)
	return stripeCust.ID, nil
}

// CreateCheckoutSession launches a Stripe Checkout flow for a user.
func (s *Service) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, req CreateCheckoutRequest) (*CheckoutResponse, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, appErrors.NewNotFound("user not found")
	}

	custID, err := s.CreateOrGetCustomer(ctx, userID, user.Email)
	if err != nil {
		return nil, err
	}

	successURL := req.SuccessURL
	if successURL == "" {
		successURL = s.cfg.SuccessURL
	}
	cancelURL := req.CancelURL
	if cancelURL == "" {
		cancelURL = s.cfg.CancelURL
	}

	// Configure Stripe Checkout parameters
	params := &stripe.CheckoutSessionParams{
		Customer:   stripe.String(custID),
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(req.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"user_id": userID.String(),
			},
		},
		Metadata: map[string]string{
			"user_id": userID.String(),
		},
	}

	sess, err := stripeSession.New(params)
	if err != nil {
		s.logger.ErrorContext(ctx, "creating Stripe checkout session", "user_id", userID, "error", err)
		return nil, appErrors.NewInternal("failed to initiate checkout flow", err)
	}

	return &CheckoutResponse{
		CheckoutURL: sess.URL,
		SessionID:   sess.ID,
	}, nil
}

// CreatePortalSession constructs a Stripe Customer Portal link.
func (s *Service) CreatePortalSession(ctx context.Context, userID uuid.UUID) (*PortalSessionResponse, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, appErrors.NewNotFound("user not found")
	}

	if user.StripeCustomerID == nil || *user.StripeCustomerID == "" {
		return nil, appErrors.NewBadRequest("user has no active billing registration")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*user.StripeCustomerID),
		ReturnURL: stripe.String(s.cfg.CancelURL), // redirect back on exit
	}

	sess, err := billingPortalSession.New(params)
	if err != nil {
		s.logger.ErrorContext(ctx, "creating Stripe portal session", "user_id", userID, "error", err)
		return nil, appErrors.NewInternal("failed to launch customer portal", err)
	}

	return &PortalSessionResponse{
		URL: sess.URL,
	}, nil
}

// GetSubscription fetches the subscription status for a user.
func (s *Service) GetSubscription(ctx context.Context, userID uuid.UUID) (*SubscriptionInfo, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, appErrors.NewNotFound("user not found")
	}

	if user.SubscriptionID == nil || *user.SubscriptionID == "" {
		return &SubscriptionInfo{
			Plan:   user.Plan,
			Status: string(user.SubscriptionStatus),
		}, nil
	}

	sub, err := subscription.Get(*user.SubscriptionID, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "fetching subscription from Stripe", "sub_id", *user.SubscriptionID, "error", err)
		// Return denormalized state if Stripe fails
		return &SubscriptionInfo{
			Plan:   user.Plan,
			Status: string(user.SubscriptionStatus),
		}, nil
	}

	var currentPeriodEnd time.Time
	if sub.CurrentPeriodEnd > 0 {
		currentPeriodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
	}

	return &SubscriptionInfo{
		Plan:              user.Plan,
		Status:            string(sub.Status),
		CurrentPeriodEnd:  currentPeriodEnd,
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}, nil
}

// CancelSubscription requests cancelation of a user's subscription at period end.
func (s *Service) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return appErrors.NewNotFound("user not found")
	}

	if user.SubscriptionID == nil || *user.SubscriptionID == "" {
		return appErrors.NewBadRequest("user has no active subscription to cancel")
	}

	// Set cancel_at_period_end = true
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}

	_, err = subscription.Update(*user.SubscriptionID, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "canceling subscription", "sub_id", *user.SubscriptionID, "error", err)
		return appErrors.NewInternal("failed to schedule subscription cancellation", err)
	}

	s.logger.InfoContext(ctx, "subscription cancellation scheduled", "user_id", userID, "sub_id", *user.SubscriptionID)
	return nil
}

// SyncSubscriptionStatus synchronizes a subscription's state from Stripe into the local database.
func (s *Service) SyncSubscriptionStatus(ctx context.Context, stripeSubID string) error {
	sub, err := subscription.Get(stripeSubID, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "getting subscription for sync", "sub_id", stripeSubID, "error", err)
		return err
	}

	// Retrieve user ID from metadata
	userIDStr, ok := sub.Metadata["user_id"]
	if !ok {
		// Fallback: check customer metadata or lookup customer
		s.logger.WarnContext(ctx, "user_id metadata missing in subscription", "sub_id", stripeSubID)
		cust, err := customer.Get(sub.Customer.ID, nil)
		if err != nil {
			s.logger.ErrorContext(ctx, "fetching customer for metadata fallback", "customer_id", sub.Customer.ID, "error", err)
			return err
		}
		userIDStr, ok = cust.Metadata["user_id"]
		if !ok {
			return errors.New("cannot match Stripe records to local user ID: user_id metadata is missing")
		}
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.logger.ErrorContext(ctx, "parsing user ID from metadata", "user_id_str", userIDStr, "error", err)
		return err
	}

	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Map Stripe status to local database status
	status := sqlc.SubscriptionStatusNone
	switch sub.Status {
	case stripe.SubscriptionStatusActive:
		status = sqlc.SubscriptionStatusActive
	case stripe.SubscriptionStatusCanceled:
		status = sqlc.SubscriptionStatusCanceled
	case stripe.SubscriptionStatusPastDue:
		status = sqlc.SubscriptionStatusPastDue
	case stripe.SubscriptionStatusTrialing:
		status = sqlc.SubscriptionStatusTrialing
	case stripe.SubscriptionStatusIncomplete:
		status = sqlc.SubscriptionStatusIncomplete
	}

	// Use price ID to determine the plan
	plan := "free"
	if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		plan = sub.Items.Data[0].Price.ID
	}

	err = s.queries.UpdateUserBilling(ctx, sqlc.UpdateUserBillingParams{
		ID:                 userID,
		StripeCustomerID:   user.StripeCustomerID,
		Plan:               plan,
		SubscriptionStatus: status,
		SubscriptionID:     &stripeSubID,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "updating synced billing records in DB", "user_id", userID, "error", err)
		return err
	}

	s.logger.InfoContext(ctx, "subscription synchronized successfully", "user_id", userID, "status", status, "plan", plan)
	return nil
}
