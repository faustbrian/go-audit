# Changelog

All notable changes to this module will be documented here. The format follows
Keep a Changelog, and releases follow Semantic Versioning.

## [Unreleased]

### Changed

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

## [1.0.0] - 2026-08-25

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-audit` identity while preserving its documented API and behavior.

### Documentation

- Link the package README to the repository-wide Golib documentation portal.

### Added

- Immutable-by-contract bounded audit record construction and canonical JSON.
- Explicit actor, subject, context, outcome, change, privacy, delivery,
  integrity, query, export, retention, and observation contracts.
- Bounded concurrency-safe in-memory test adapter.
- Separately releasable PostgreSQL append-only adapter with least-privilege
  roles, stable pagination, transaction staging, and legal-hold-aware retention.
- SHA-256 and HMAC-SHA-256 chain verification, checkpoints, and Merkle anchors
  using standard-library primitives and external key-provider contracts.
- Explicit finite record, batch, and persisted-byte limits for durable buffers,
  plus golden integrity, leak, stress, benchmark, and PostgreSQL version-matrix
  release evidence.
- Canonical UTC microsecond timestamps and mandatory stable IDs for system
  actors, preserving ordering across durable adapters.

### Security

- Reject secret-bearing namespaces and unrestricted request or response bodies.
- Redact before persistence and expose identifier-free observation events.
- Keep arbitrary redactor failures opaque and give PostgreSQL writers only the
  append-function privilege rather than direct table read access.
- Reject redactor field injection and post-validation aliasing, sanitize every
  dependency boundary including builder clocks and ID generators, and require
  explicit bounded recovery for fail-open and durable-buffer policies.
- Verify canonical bytes, persisted digests, historical HMAC key IDs, stable
  acceptance snapshots, deterministic retention plans, and cross-adapter field
  ceilings before use.
- Reject arbitrary and credential-shaped authentication-method values; the
  field accepts only the closed `AuthenticationMethod*` vocabulary.
- Reject NUL in every durable text field and map entry so every accepted core
  record remains representable by the PostgreSQL adapter.
- Reject separator-obfuscated credential and API-key field names at every
  validated persistence boundary.

### Fixed

- Preserve cursor round trips for newline-bearing durable record IDs and reject
  timestamps whose UTC canonicalization leaves the supported year range.

[Unreleased]: https://github.com/faustbrian/go-audit/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/faustbrian/go-audit/releases/tag/v1.0.0
