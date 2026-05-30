package queue

// Task type constants for all background jobs processed by the worker.
const (
	TaskSendVerificationEmail  = "email:send_verification"
	TaskSendResetEmail         = "email:send_reset"
	TaskSendWelcomeEmail       = "email:send_welcome"
	TaskCleanupExpiredTokens   = "cleanup:expired_tokens"
	TaskCleanupExpiredSessions = "cleanup:expired_sessions"
	TaskProcessStripeWebhook   = "billing:process_stripe_webhook"
)
