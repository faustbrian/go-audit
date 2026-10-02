# Assurance and security review

The released-v1 inventory is preserved in `api/baseline.txt`; the active
v2 API baseline is `api/v2.txt`. The implementation inventory is the
record and canonical structures in `record.go` and `canonical.go`, the sink and
delivery contracts in `sink.go` and `delivery.go`, privacy in `privacy.go`,
query and export in `query.go`, integrity in `integrity.go`, retention in
`retention.go`, safe observations in `observe.go`, the bounded memory adapter,
and the separately versioned PostgreSQL store, transaction writer, retention
administrator, embedded migration, roles, indexes, triggers, and functions.
`modules.json` and `packages.json` are the release manifest authority.

The cryptographic surface uses only `crypto/sha256`, `crypto/hmac`,
`crypto/rand`, and constant-time digest comparison from the Go standard
library. HMAC keys are caller-provided, copied before use, never stored in a
record, and selected by explicit key ID and recording time. The package does not
generate, derive, wrap, rotate, persist, or attest keys. SHA-256 chains detect
corruption only; HMAC adds authenticity only while key custody remains trusted;
neither primitive alone supplies non-repudiation.

Canonical version 1 is frozen by a readable golden record and an independently
computed chain digest. Tests cover key rotation, checkpoints, missing,
reordered, duplicated, altered, truncated, partially archived, and restored
records. PostgreSQL fault tests classify validation and statement failures as
rejected and post-commit ambiguity as unknown, including deadlock and
serialization SQLSTATEs. Real-database tests cover transactional migration
interruption, published migration checksums, fail-safe reserved-role
neutralization and atomic fresh-install reservation, atomic caller-owned
writes, duplicate reconciliation, stable
pagination, cancellation, protocol-compatible rolling writes across migration,
backup and restore, two-phase retention, legal holds,
backend termination and pool reconnection, caller-search-path shadowing denial,
duplicate retention-order rejection, closed authentication-method validation,
and least-privilege read/update/delete denial.

The supported PostgreSQL matrix is the upstream-supported majors 14 through 18
using the digest-pinned current-minor images declared in
`postgres/testdata/postgres-images.tsv`. The integration-tagged PostgreSQL
module gate runs against the default pinned 18 image, while the version table
is validated by `TestPostgreSQLVersionMatrixUsesImmutableImages`. Race and
`goleak` checks cover all packages. Fuzz, stress, soak, fault, exact statement
coverage, viable mutation, benchmarks, security, dependency, documentation,
API, and clean-consumer gates are release requirements rather than optional
warnings.

Benchmarks measure canonical encoding, redaction, single and atomic batch
append, fully filtered pagination, streaming export, and chain verification.
Core workloads include explicit equivalent standard-library reference paths;
the PostgreSQL workloads execute against the selected real database image.
Results must be retained with Go version, machine, corpus, duration, latency,
throughput, and allocations; they are engineering evidence, not universal
service-level objectives.

## Owned release exclusions

These boundaries are not reported as passing adapter gates:

| Boundary | Owner | Reason | Risk | Expiry |
| --- | --- | --- | --- | --- |
| Partition rollover | audit maintainers | The supported schema is unpartitioned so global record-ID uniqueness remains enforceable. | A deployment-specific partitioned fork can lose global idempotency or bypass legal holds during rollover. | 2027-02-09, or before any partitioned adapter is released, whichever comes first. |
| Physical standby promotion | audit maintainers | The adapter owns no PostgreSQL topology; local evidence terminates a backend and proves pool reconnection and idempotent recovery. | A deployment's proxy, DNS, replication, or promotion policy may produce a longer or ambiguous outage. | 2027-02-09, or before claiming support for a managed failover topology, whichever comes first. |
| Previous-binary rolling deployment | audit maintainers | Version 1 has no previous released reader or writer binary; the frozen version-1 fixture is exercised through upgrade, backup, and restore instead. | The first post-v1 format could break old readers if released without a real two-binary matrix. | Before the first format or API release after v1. |

## Caller-owned synchronous dependency risks

| Boundary | Owner | Rationale | Mitigation | Review condition |
| --- | --- | --- | --- | --- |
| HMAC `KeyProvider` | Deployer | Key custody, historical rotation, and backend selection must remain outside the library. | The library passes context, caps each verification at `MaxIntegrityRecords`, holds no locks across lookup, copies returned key bytes, and converts panics or arbitrary failures to `ErrKeyUnavailable`. Providers must bound latency, honor cancellation, and support concurrency. | Review before changing the key backend, retry policy, timeout budget, or custody model. |
| `Observer` hooks | Deployer | Applications select their metrics and tracing backend, and synchronous delivery preserves operation ordering without hidden workers. | Observations contain no record identifiers, panics are contained, no library lock is held, and the caller must keep hooks non-blocking, context-aware, and concurrency-safe. | Review before changing telemetry backends, exporters, batching, or request latency budgets. |
| Export consumer callbacks | Deployer | Streaming keeps export memory bounded and lets the authorized caller own archive publication. | Queries cap record count, callbacks run without library locks, panics and arbitrary failures are sanitized, and context is propagated. Consumers must bound per-record work and stop on cancellation. | Review before changing archive destinations, retry behavior, authorization, or export size and latency budgets. |
| Delivery, redaction, storage, and retention collaborators | Deployer | Applications own durability, privacy policy, alerting, buffering, persistence, and legal-hold decisions. | Operations pass context, cap batches and queries, contain panics, sanitize arbitrary failures, and avoid library locks across calls. Implementations must bound latency and capacity, honor cancellation, and support the documented concurrency. | Review before changing a backend, redaction policy, capacity, retry behavior, timeout, or transaction boundary. |
| Builder clock and ID generator | Application owner | Applications may need deterministic time and externally allocated globally unique identifiers. | Each build invokes them once, contains panics, sanitizes failures, and validates the result. These callbacks receive no context and therefore must be constant-time, local, and concurrency-safe. | Review before installing any custom clock or generator, especially one using network, storage, locks, or retries. |
