# Billing

Billing is integrated with Stripe and synced into user state.

## Included capabilities

- Checkout session creation
- Billing portal session creation
- Subscription retrieval and cancellation paths
- Webhook verification and idempotent event handling
- User billing fields synchronized from Stripe events

## Route surface

- API: `/api/v1/billing/*`
- Web: `/pricing`, `/billing/checkout-web`, `/billing/portal-web`
- Webhook: `/webhooks/stripe`
