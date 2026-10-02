# Compatibility Policy

Each releasable directory is an independent Go module and follows semantic
versioning. Root-module releases use `v<version>` tags. The separately
releasable PostgreSQL module uses `postgres/v<version>` tags. A directory prefix
is never added to the root module's tag.

The published root v2 and PostgreSQL v1 modules are stable libraries. The
`github.com/faustbrian/go-audit/v2` root module introduces bounded integrity
verification and earlier record-byte rejection, which change
previously accepted behavior and validation error precedence.
The independently released PostgreSQL module continues to use root v1;
local `replace` directives must not bridge the major-version boundary.

To adopt v2, change root and memory imports to include `/v2`, split
integrity operations into at most 1,000 records, and retain trusted checkpoints
between chain-verification segments. A Merkle root is defined for each bounded
ordered batch; splitting does not preserve the root of a larger batch. Record
byte ceilings apply even when field budgets are configured higher. Canonical
bytes for accepted records and exported error sentinel names are unchanged.
Sentinel values are distinct across module majors: use v2 sentinels with v2
errors rather than comparing them with v1 sentinels through `errors.Is`.
Oversized input is rejected as `ErrInvalidArgument` before text or
privacy validation, even when the same input also contains a prohibited key.
The PostgreSQL adapter remains a root-v1 consumer and cannot receive root-v2
record values without a separately reviewed adapter migration.

The modules require Go 1.27.0, and repository verification currently tests
exactly Go 1.27.0. The PostgreSQL adapter supports PostgreSQL 14 through 18
according to the digest-pinned matrix in
`postgres/testdata/postgres-images.tsv`.

Before `v1`, minor releases MAY contain reviewed breaking changes, but every
break MUST be documented with migration guidance. Patch releases MUST remain
backward compatible. At and after `v1`, incompatible exported API or documented
behavior changes require a new major version.

Compatibility includes exported Go APIs, error classification, serialization,
protocol behavior, persistence schemas, environment variables, command output,
resource ownership, ordering, retry/idempotency semantics, and documented
defaults. A compile-compatible change can still be behaviorally breaking.

Specification-backed modules MUST NOT diverge from their declared standards.
Ambiguities require documented decisions and stable tests. Deprecated APIs
follow [`DEPRECATION.md`](DEPRECATION.md).
