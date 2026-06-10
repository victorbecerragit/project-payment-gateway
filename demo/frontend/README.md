# Payment Gateway Demo Frontend

This directory contains the planned frontend for the Payment Gateway.

## Purpose

The demo frontend is an educational and validation tool that visually demonstrates the gateway's capabilities, especially verifying the state changes when using a real provider like **Stripe**. 

While the gateway's generic architecture is strictly **provider-agnostic**, the frontend is designed to show how provider-backed status changes (such as webhooks triggered via the Stripe CLI) seamlessly affect the application's domain state.

## Planned Capabilities

- **Create a Payment**: A simple UI to interact with `POST /api/v1/payments`, generating an idempotency key and sending the request to the gateway.
- **Payment Lifecycle Monitor**: A visual dashboard observing the transitions from `PENDING` -> `PROCESSING` -> `COMPLETED` or `FAILED`.
- **Provider References**: Display of underlying metadata, such as the Gateway Payment ID linked to the Stripe Provider Reference (e.g. `pi_...`), if exposed for demo purposes.
- **Webhook Visualization**: A side-panel or guided step documentation showing how simulated webhook completions are processed behind the scenes using `stripe trigger` or automated load generation.

## Future Tech Stack

To align with modern but simple demo environments (similar to Bank of Anthos components), this frontend may be implemented in React or plain HTML/JS and packaged as a lightweight container.
