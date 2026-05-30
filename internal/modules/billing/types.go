package billing

import "time"

// CreateCheckoutRequest is the payload for creating a checkout session.
type CreateCheckoutRequest struct {
	PriceID    string `json:"price_id" validate:"required"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

// CheckoutResponse contains the URL to redirect the user to Stripe Checkout.
type CheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
	SessionID   string `json:"session_id"`
}

// SubscriptionInfo holds read-only denormalized billing status.
type SubscriptionInfo struct {
	Plan              string    `json:"plan"`
	Status            string    `json:"status"`
	CurrentPeriodEnd  time.Time `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd bool      `json:"cancel_at_period_end"`
}

// PortalSessionResponse contains the Stripe customer billing portal URL.
type PortalSessionResponse struct {
	URL string `json:"url"`
}
