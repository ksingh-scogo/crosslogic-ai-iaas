# Billing Engine

The Billing Engine ensures that all usage is accurately tracked and charged. It integrates tightly with Stripe for metering and invoicing.

## Responsibilities

-   **Usage Tracking**: Records input and output tokens for every request.
-   **Aggregation**: Aggregates usage per tenant and billing cycle.
-   **Stripe Sync**: Pushes usage records to Stripe Metered Billing.
-   **Invoicing**: Generates invoices based on usage and subscription plans.

## Architecture

### Data Flow

1.  **Request Completion**: When a request finishes (in Scheduler), a `UsageRecord` is generated.
2.  **Async Processing**: The record is pushed to a buffered channel or queue (to avoid blocking the response).
3.  **Persistance**: The record is saved to the `usage_records` table in PostgreSQL.
4.  **Aggregation Job**: A background job runs periodically to aggregate unbilled usage.
5.  **Stripe Push**: Aggregated usage is sent to Stripe API.

### Pricing Models

Pricing is defined in the `models` table:

-   `price_input_per_million`: Cost per 1M input tokens.
-   `price_output_per_million`: Cost per 1M output tokens.

### Webhooks

The Billing Engine listens for Stripe webhooks to handle:

-   `invoice.payment_succeeded`: Mark usage as paid.
-   `invoice.payment_failed`: Alert admin / suspend tenant.
-   `customer.subscription.created`: Provision new tenant resources.

## Configuration

-   `STRIPE_SECRET_KEY`: Secret key for Stripe API.
-   `STRIPE_WEBHOOK_SECRET`: Secret for verifying webhook signatures.
-   `BILLING_AGGREGATION_INTERVAL`: How often to aggregate usage (e.g., `1h`).
