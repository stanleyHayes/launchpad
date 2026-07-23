# ADR 0001 — MongoDB and Redis for Phase 0

## Status

Accepted

## Context

The original PRD recommended PostgreSQL + Redis. The team chose MongoDB + Redis for the initial build to move faster on document-shaped onboarding data (journeys, steps, config blobs) while keeping Redis for sessions and short-lived coordination.

## Decision

- MongoDB is the primary application datastore.
- Redis stores refresh/session tokens, rate-limit counters, and cache keys.
- Custom Go authentication issues JWTs and Redis-backed sessions.
- Tenant isolation is enforced in repository queries via `organizationId` (no SQL RLS).

## Consequences

- Index bootstrap scripts replace SQL migrations.
- Vector search will use MongoDB Atlas Vector Search (or equivalent) instead of pgvector.
- Relational reporting may require aggregation pipelines or a later analytics store.
