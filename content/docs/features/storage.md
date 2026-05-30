# Storage

GoStarter supports storage through an abstraction so local and object-backed strategies can be used behind one interface.

## Typical usage

- local storage for development
- object storage for production deployments

## Configuration

Use storage driver and provider env vars from `.env.example` and validate at startup.
