# Email

Email delivery is handled through a sender abstraction and queued tasks.

## Included capabilities

- SMTP sender implementation
- verification/reset/welcome-style transactional flows
- async delivery through worker tasks

## Operational note

Run the worker process in all environments where email should be delivered.
