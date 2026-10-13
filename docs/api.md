# API and record semantics

[Documentation map](index.md) · [Repository README](../README.md)
[Next: Adoption patterns](adoption.md)

This source tree documents the root-v2 API. PostgreSQL references describe
the independent root-v1 adapter's storage conventions, not a v2 integration.
No currently released PostgreSQL adapter accepts v2 records; v2 consumers need
application-owned storage integration pending a separate adapter migration.

`Builder` constructs new records; `ParseCanonicalJSON` restores records from
schema-v1 canonical bytes. Construction validates required identities, bounds,
maps, privacy namespaces, changes and integrity metadata shape, then owns mutable
inputs. Neither path proves identity truth, ID uniqueness or digest authenticity.
Map and digest accessors return copies.

An empty optional string means absent. Unknown is explicit through
`ActorUnknown` or `OutcomeUnknown`; it is not inferred from an empty value.
Anonymous actors use `ActorAnonymous` and cannot have an ID. Human, service,
and system actors require stable IDs. Deleted subjects keep their stable type and
ID and set `Deleted`; deletion never erases identity from an existing record.
Zero actor kinds, outcomes, times, tenant scopes, queries, and limits are
invalid where the contract requires an explicit choice.

Record IDs are caller-independent cryptographically random UUID-shaped values
by default. An injected generator may provide another globally unique stable
ID; the library validates its shape but does not enforce uniqueness. Default
identity does not depend on map iteration, process counters, database ordering
or a database-generated sequence; injected generation is application-owned.
Recording time comes from the builder clock and occurrence time from the
caller; both are canonical UTC at microsecond precision so ordering is stable
across the PostgreSQL adapter.

`CanonicalJSON` is package-owned schema-v1 deterministic encoding, not RFC 8785
canonicalization. It encodes a value without validating or redacting it.
`ParseCanonicalJSON` strictly rejects alternate byte representations, unknown
fields, trailing values, malformed times and digests, unsupported schema
versions, and values
outside the configured limits. Parsing does not assert that redaction ran in
the current process; adapters restore that trusted state only after verifying
their persisted canonical digest.
`MaxIntegrityBytes` must be exactly 32 because every supported digest is
SHA-256. `MaxRecordBytes` and `MaxFieldBytes` may be tightened but may not
exceed their defaults, which are the durable PostgreSQL and cursor ceilings;
all other configurable collection and byte ceilings must be positive.

`Record`, `Query`, `Cursor`, `Checkpoint`, and retention values are safe to
share after construction because mutable inputs and byte/map accessors are
copied. `Builder`, `Recorder`, and `Chain` own no mutable counters and start no
goroutines; concurrent use additionally requires injected clocks, generators,
sinks, redactors, observers, alerters, buffers, and key providers to support the
same use. Every synchronous collaborator must bound its own latency and resource
use; context-aware collaborators must stop on cancellation. Builder clocks and
ID generators receive no context and must remain constant-time and local. The
memory adapter serializes mutation with its own context-aware gate. PostgreSQL
adapters rely on the supplied caller-owned pool or transaction and never close
them.

Builder clock and ID-generator panics are contained. ID-generator failures are
reported only as `ErrRecordIDUnavailable`; clock panics are reported only as
`ErrClockUnavailable`. Arbitrary dependency diagnostics are not retained.

`MaxIntegrityRecords` limits each `Chain.Verify`,
`Chain.VerifyFromCheckpoint`, and `MerkleRoot` operation to 1,000 records. The
limit bounds local hashing, canonical encoding, allocation, and HMAC key
lookups. Verification checks cancellation between records. Key providers and
observers remain caller-owned synchronous dependencies; they must honor the
supplied context, bound their own latency, and permit concurrent calls.
