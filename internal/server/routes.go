package server

import (
	"encoding/json"
	"net/http"

	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/milzam/go-starter/internal/app"
	"github.com/milzam/go-starter/internal/auth"
	"github.com/milzam/go-starter/internal/authz"
	"github.com/milzam/go-starter/internal/docs"
	"github.com/milzam/go-starter/internal/middleware"
	"github.com/milzam/go-starter/internal/modules/billing"
	"github.com/milzam/go-starter/internal/modules/users"
	"github.com/milzam/go-starter/internal/observability"
	"github.com/milzam/go-starter/web/templates/pages"
	"github.com/milzam/go-starter/web/templates/showcase"
	"github.com/templui/templui/components"
)

// RegisterRoutes mounts all application routes onto the chi router.
func RegisterRoutes(r chi.Router, application *app.App) {
	// Global Validator instance
	validate := validator.New()

	// Apply global Auth Context extraction (places Principal in context if present)
	r.Use(middleware.AuthContext(application.Config.Auth.JWTSecret, application.Queries, application.Logger))

	// Instantiate Handlers
	authHandlers := auth.NewHandlers(application.AuthService, validate, application.Logger, application.Config.Auth)
	usersHandlers := users.NewHandlers(application.UsersService, validate, application.Logger)
	billingHandlers := billing.NewHandlers(application.BillingService, validate, application.Logger, application.Config.Stripe)

	// Health check (no auth)
	r.Get("/healthz", observability.NewHealthHandler(application.DB))
	r.Get("/readyz", observability.NewHealthHandler(application.DB))

	// Stripe webhook (raw body, no auth/CSRF middleware)
	r.Post("/webhooks/stripe", billingHandlers.HandleStripeWebhook)

	// API v1 routes (JSON API)
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Group(func(r chi.Router) {
			r.Post("/auth/signup", authHandlers.HandleSignup)
			r.Post("/auth/login", authHandlers.HandleLogin)
			r.Post("/auth/refresh", authHandlers.HandleRefresh)
			r.Post("/auth/forgot-password", authHandlers.HandleForgotPassword)
			r.Post("/auth/reset-password", authHandlers.HandleResetPassword)
			r.Post("/auth/verify-email", authHandlers.HandleVerifyEmail)
		})

		// Auth routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Post("/auth/logout", authHandlers.HandleLogout)
			r.Get("/auth/me", authHandlers.HandleGetMe)
			r.Post("/auth/2fa/enable", authHandlers.HandleEnable2FA)
			r.Post("/auth/2fa/verify", authHandlers.HandleVerify2FA)
			r.Post("/auth/2fa/disable", authHandlers.HandleDisable2FA)
		})

		// User profile & management routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Get("/users/{id}", usersHandlers.HandleGetUser)
			r.Put("/users/{id}", usersHandlers.HandleUpdateUser)

			// Admin-only user management (Casbin Authorize middleware checks global RBAC)
			r.Group(func(r chi.Router) {
				r.Use(authz.Authorize(application.Enforcer, application.Logger))
				r.Get("/users", usersHandlers.HandleListUsers)
				r.Patch("/users/{id}/role", usersHandlers.HandleUpdateUserRole)
				r.Delete("/users/{id}", usersHandlers.HandleDeleteUser)
			})
		})

		// Billing routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Post("/billing/checkout", billingHandlers.HandleCreateCheckout)
			r.Get("/billing/subscription", billingHandlers.HandleGetSubscription)
			r.Post("/billing/portal", billingHandlers.HandleCreatePortalSession)
			r.Post("/billing/cancel", billingHandlers.HandleCancelSubscription)
		})
	})

	// OAuth callbacks
	r.Get("/auth/callback/{provider}", authHandlers.HandleOAuthCallback)

	// Serve static files from web/static directory
	fs := http.FileServer(http.Dir("web/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Serve templui component scripts dynamically from embedded FS
	r.Get("/templui/js/*", func(w http.ResponseWriter, r *http.Request) {
		pathParam := chi.URLParam(r, "*")
		if pathParam == "" || strings.Contains(pathParam, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		if application.Config.App.IsDev() {
			w.Header().Set("Cache-Control", "no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000")
		}
		fileName := path.Base(pathParam)
		component := strings.TrimSuffix(fileName, ".min.js")
		component = strings.TrimSuffix(component, ".js")

		file, err := components.TemplFiles.ReadFile(path.Join(component, fileName))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(file)
	})

	// Web routes (templ + HTMX, HTML responses)
	r.Group(func(r chi.Router) {
		// Marketing pages (public)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.Home().Render(r.Context(), w)
		})
		r.Get("/showcase", func(w http.ResponseWriter, r *http.Request) {
			_ = showcase.SidebarDefault().Render(r.Context(), w)
		})
		r.Get("/pricing", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.Pricing(application.Config.Stripe.PriceID).Render(r.Context(), w)
		})
		r.Get("/features", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.Features().Render(r.Context(), w)
		})
		r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.DocsIndex(docs.LandingGroups()).Render(r.Context(), w)
		})
		// Backward-compatible redirects from legacy docs URLs.
		r.Get("/docs/getting-started", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs/introduction", http.StatusMovedPermanently)
		})
		r.Get("/docs/setup", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs/installation", http.StatusMovedPermanently)
		})
		r.Get("/docs/{slug}", func(w http.ResponseWriter, r *http.Request) {
			path := "/docs/" + chi.URLParam(r, "slug")
			article, found, err := docs.BuildArticle(path)
			if err != nil {
				http.Error(w, "failed to load docs page", http.StatusInternalServerError)
				return
			}
			if !found {
				http.NotFound(w, r)
				return
			}
			_ = pages.DocsArticle(article).Render(r.Context(), w)
		})
		r.Get("/docs/{section}/{slug}", func(w http.ResponseWriter, r *http.Request) {
			path := "/docs/" + chi.URLParam(r, "section") + "/" + chi.URLParam(r, "slug")
			article, found, err := docs.BuildArticle(path)
			if err != nil {
				http.Error(w, "failed to load docs page", http.StatusInternalServerError)
				return
			}
			if !found {
				http.NotFound(w, r)
				return
			}
			_ = pages.DocsArticle(article).Render(r.Context(), w)
		})
		r.Get("/docs/components", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs/features/components", http.StatusMovedPermanently)
		})
		r.Get("/docs/components/{name}", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.DocsComponent(chi.URLParam(r, "name")).Render(r.Context(), w)
		})

		// Auth pages (public)
		r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := middleware.GetPrincipal(r.Context()); ok {
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
			_ = pages.Login().Render(r.Context(), w)
		})
		r.Get("/signup", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := middleware.GetPrincipal(r.Context()); ok {
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
			_ = pages.Signup().Render(r.Context(), w)
		})
		r.Get("/forgot-password", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.ForgotPassword().Render(r.Context(), w)
		})
		r.Get("/reset-password", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.ResetPassword(r.URL.Query().Get("token")).Render(r.Context(), w)
		})
		r.Get("/verify-email", func(w http.ResponseWriter, r *http.Request) {
			_ = pages.VerifyEmail(r.URL.Query().Get("token")).Render(r.Context(), w)
		})
		r.Post("/auth/forgot-password-web", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
				return
			}
			email := r.FormValue("email")
			body, _ := json.Marshal(map[string]string{"email": email})
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			rr := newResponseRecorder()
			authHandlers.HandleForgotPassword(rr, req)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
		r.Post("/auth/reset-password-web", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				http.Redirect(w, r, "/reset-password", http.StatusSeeOther)
				return
			}
			body, _ := json.Marshal(map[string]string{
				"token":        r.FormValue("token"),
				"new_password": r.FormValue("new_password"),
			})
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			authHandlers.HandleResetPassword(newResponseRecorder(), req)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
		r.Post("/auth/verify-email-web", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
				return
			}
			body, _ := json.Marshal(map[string]string{"token": r.FormValue("token")})
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			authHandlers.HandleVerifyEmail(newResponseRecorder(), req)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
		clearSessionCookie := func(w http.ResponseWriter, r *http.Request) {
			secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				Expires:  time.Unix(0, 0),
				Secure:   secure,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}
		r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
			clearSessionCookie(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
		r.Post("/auth/logout-web", func(w http.ResponseWriter, r *http.Request) {
			clearSessionCookie(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
		r.Post("/billing/checkout-web", func(w http.ResponseWriter, r *http.Request) {
			p, ok := middleware.GetPrincipal(r.Context())
			if !ok {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			userID, err := uuid.Parse(p.UserID)
			if err != nil {
				http.Redirect(w, r, "/pricing", http.StatusSeeOther)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Redirect(w, r, "/pricing", http.StatusSeeOther)
				return
			}
			res, err := application.BillingService.CreateCheckoutSession(r.Context(), userID, billing.CreateCheckoutRequest{
				PriceID:    r.FormValue("price_id"),
				SuccessURL: application.Config.Stripe.SuccessURL,
				CancelURL:  application.Config.Stripe.CancelURL,
			})
			if err != nil {
				http.Redirect(w, r, "/pricing", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, res.CheckoutURL, http.StatusSeeOther)
		})
		r.Post("/billing/portal-web", func(w http.ResponseWriter, r *http.Request) {
			p, ok := middleware.GetPrincipal(r.Context())
			if !ok {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			userID, err := uuid.Parse(p.UserID)
			if err != nil {
				http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
				return
			}
			res, err := application.BillingService.CreatePortalSession(r.Context(), userID)
			if err != nil {
				http.Redirect(w, r, "/settings/billing", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, res.URL, http.StatusSeeOther)
		})

		// App pages (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
				p, _ := middleware.GetPrincipal(r.Context())
				userID, err := uuid.Parse(p.UserID)
				name := "Developer"
				email := p.Email
				if err == nil {
					userInfo, err := application.AuthService.GetCurrentUser(r.Context(), userID)
					if err == nil && userInfo != nil {
						name = userInfo.Name
						email = userInfo.Email
					}
				}
				_ = pages.Dashboard(name, email).Render(r.Context(), w)
			})
			r.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
				_ = pages.SettingsIndex().Render(r.Context(), w)
			})
			r.Get("/settings/profile", func(w http.ResponseWriter, r *http.Request) {
				_ = pages.SettingsProfile().Render(r.Context(), w)
			})
			r.Get("/settings/billing", func(w http.ResponseWriter, r *http.Request) {
				p, _ := middleware.GetPrincipal(r.Context())
				userID, err := uuid.Parse(p.UserID)
				if err != nil {
					_ = pages.SettingsBilling(nil).Render(r.Context(), w)
					return
				}
				info, err := application.BillingService.GetSubscription(r.Context(), userID)
				if err != nil {
					_ = pages.SettingsBilling(nil).Render(r.Context(), w)
					return
				}
				_ = pages.SettingsBilling(info).Render(r.Context(), w)
			})
			r.Get("/settings/security", func(w http.ResponseWriter, r *http.Request) {
				_ = pages.SettingsSecurity().Render(r.Context(), w)
			})
		})

		// Admin pages (admin role required)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Use(authz.Authorize(application.Enforcer, application.Logger))
			r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
				_ = pages.AdminDashboard().Render(r.Context(), w)
			})
			r.Get("/admin/users", func(w http.ResponseWriter, r *http.Request) {
				list, err := application.UsersService.ListUsers(r.Context(), 1, 50)
				if err != nil {
					_ = pages.AdminUsers(nil).Render(r.Context(), w)
					return
				}
				_ = pages.AdminUsers(list).Render(r.Context(), w)
			})
			r.Get("/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
				_ = pages.AdminUserDetail(chi.URLParam(r, "id")).Render(r.Context(), w)
			})
		})
	})
}

type responseRecorder struct {
	header http.Header
	status int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header)}
}

func (r *responseRecorder) Header() http.Header         { return r.header }
func (r *responseRecorder) WriteHeader(code int)        { r.status = code }
func (r *responseRecorder) Write(b []byte) (int, error) { return len(b), nil }

func (r *responseRecorder) StatusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}
