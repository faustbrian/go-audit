# Source documentation audit

[Documentation map](index.md) · [Repository README](../README.md)
[Next: API and record semantics](api.md)

## Scope and compact plan

Base: `54c4e6f2b30ab94fb854d26440ece1803f8a5023` (`origin/main`).
This is a Tier A source-contract documentation pass, not a runtime, compliance,
release, or adoption-portal change. The owner is this standalone repository.
Direct consumers are record builders, recorder collaborators, memory-store
callers, integrity/archive custodians, and independently versioned PostgreSQL
writers/readers/retention operators.

Inspect every tracked Go file and public declaration/field/interface method;
repair missing contracts and concrete prose contradictions, preserving executable
ASTs, canonical bytes, fixture pins, SQL and API signatures. Verify formatting,
parsed and rendered docs, local links, markers and changed-content secrets, then
inspect the complete diff. Execute only the four materially changed runnable examples.
No routine runtime campaign is needed for unchanged library behavior.

## Reproducible module/package inventory

Run from this worktree: `git ls-files '*.go' 'go.mod' '**/go.mod'`;
`git ls-files '*.go' | xargs rg -n '^(package |//go:build|//go:generate|//go:embed)|Code generated|TODO|FIXME|HACK|XXX'`.
Use `.golib.yaml`, `modules.json`, `packages.json`, the Makefile and
`scripts/check-docs.sh` to reproduce package, native-generation and check scope.

| Module | Public packages | Go producer / minimum | API baseline |
| --- | --- | --- | --- |
| `github.com/faustbrian/go-audit/v2` (`.`) | `audit`, `memory` | 1.27.2 / 1.27.0 | `api/v2.txt`, apidiff |
| `github.com/faustbrian/go-audit/postgres` (`postgres`) | `postgres` | 1.27.2 / 1.27.0 | `postgres/api/baseline.txt`, apidiff |

The adapter consumes published root **v1**, not local root v2. No replace or
major-path bridge is introduced. `api/baseline.txt` is historical root-v1 API
evidence. Native scripts render package docs, but neither configured baseline
is a prose-bearing go-doc snapshot; no signature artifact regeneration is needed.

## All tracked Go files

| File | Baseline lines | Investigation |
| --- | --- | --- |
| `benchmark_test.go` | 388 | comparative benchmark helpers; no capacity promise; 14 named entry points |
| `canonical.go` | 236 | public production contracts; 0 named entry points |
| `delivery.go` | 469 | public production contracts; 0 named entry points |
| `delivery_policy_test.go` | 456 | test/helper and assertion audit; 10 named entry points |
| `delivery_test.go` | 241 | test/helper and assertion audit; 4 named entry points |
| `examples_test.go` | 96 | test/helper and assertion audit; 4 named entry points |
| `fuzz_test.go` | 129 | test/helper and assertion audit; 3 named entry points |
| `golden_test.go` | 135 | test/helper and assertion audit; 2 named entry points |
| `integrity.go` | 349 | public production contracts; 0 named entry points |
| `integrity_test.go` | 547 | test/helper and assertion audit; 10 named entry points |
| `internal_contract_test.go` | 451 | test/helper and assertion audit; 12 named entry points |
| `leak_test.go` | 11 | goleak TestMain; unchanged lifecycle harness; 1 named entry points |
| `memory/internal_test.go` | 152 | test/helper and assertion audit; 5 named entry points |
| `memory/mutation_boundaries_test.go` | 116 | test/helper and assertion audit; 3 named entry points |
| `memory/store.go` | 317 | public production contracts; 0 named entry points |
| `memory/store_test.go` | 435 | test/helper and assertion audit; 9 named entry points |
| `memory/stress_test.go` | 104 | test/helper and assertion audit; 2 named entry points |
| `mutation_boundaries_test.go` | 412 | test/helper and assertion audit; 10 named entry points |
| `observation_test.go` | 143 | test/helper and assertion audit; 3 named entry points |
| `observe.go` | 148 | public production contracts; 0 named entry points |
| `postgres/benchmark_test.go` | 29 | comparative benchmark helpers; no capacity promise; 1 named entry points |
| `postgres/fault_test.go` | 623 | test/helper and assertion audit; 11 named entry points |
| `postgres/integration_test.go` | 2191 | integration-tagged SQL, roles, restore, retention; digest-pinned service fixtures; 16 named entry points |
| `postgres/leak_test.go` | 11 | goleak TestMain; unchanged lifecycle harness; 1 named entry points |
| `postgres/migrations.go` | 32 | public production contracts; 0 named entry points |
| `postgres/privileges.go` | 50 | public production contracts; 0 named entry points |
| `postgres/retention.go` | 206 | public production contracts; 0 named entry points |
| `postgres/retention_fault_test.go` | 299 | test/helper and assertion audit; 3 named entry points |
| `postgres/store.go` | 502 | public production contracts; 0 named entry points |
| `postgres/store_test.go` | 155 | test/helper and assertion audit; 3 named entry points |
| `privacy.go` | 111 | public production contracts; 0 named entry points |
| `query.go` | 292 | public production contracts; 0 named entry points |
| `query_retention_test.go` | 258 | test/helper and assertion audit; 4 named entry points |
| `record.go` | 778 | public production contracts; 0 named entry points |
| `record_contract_test.go` | 357 | test/helper and assertion audit; 7 named entry points |
| `record_test.go` | 213 | test/helper and assertion audit; 4 named entry points |
| `retention.go` | 184 | public production contracts; 0 named entry points |
| `scripts_test.go` | 36 | test/helper and assertion audit; 1 named entry points |
| `security_bounds_test.go` | 102 | test/helper and assertion audit; 3 named entry points |
| `security_test.go` | 453 | test/helper and assertion audit; 10 named entry points |
| `sink.go` | 132 | public production contracts; 0 named entry points |
| `sink_test.go` | 46 | test/helper and assertion audit; 2 named entry points |

## Findings and contract evidence

Missing documentation: public configuration/input/result fields, interface
methods, `ErrKeyUnavailable`, delivery-failure sentinels and PostgreSQL batch
constants. Existing canonical package comments are present in `record.go`,
`memory/store.go` and `postgres/store.go`; extend these rather than duplicate.
Private wire structs/test helpers need no public member filler.

Investigated rationale and accuracy repairs:

- Construction and privacy: `record.go`, `canonical.go`, `privacy.go`,
  `record_contract_test.go`, `record_test.go`, `security_bounds_test.go`,
  `security_test.go`. Only default IDs are random; injected IDs are validated
  as bounded text, not proven unique. UTC microseconds, closed mechanism labels,
  copy ownership and field-name privacy checks do not establish truthful facts
  or detect secrets in arbitrary values. Parsing is another public record path,
  requires byte-exact schema-v1 encoding and does not mark redaction or verify a
  chain digest. Default limits differ from explicit zero limits in the parser.
- Delivery: `sink.go`, `delivery.go`, `delivery_policy_test.go`,
  `delivery_test.go`, `observation_test.go`, `sink_test.go`. No hidden retry or
  background lifetime. Recovery deliberately detaches cancellation with a
  bounded timeout while retaining values. Confirmed-commit errors bypass fallback
  and ordinary success acknowledgement validation; returned result still has an
  error. Inspectable `errors.Is` classification is not cause unwrapping.
- Integrity: `integrity.go`, `integrity_test.go`, `golden_test.go`,
  `mutation_boundaries_test.go`. Hash the schema-v1 canonical record with only
  its current digest removed; HMAC key IDs remain included. Unkeyed hashes and
  Merkle roots are not authenticity, omission detection or signatures. `Verify`
  alone accepts an internally consistent prefix; suffix verification binds an
  independently trusted final checkpoint. Keys are copied, not erased. Existing
  OpenSSL HMAC fixtures and golden record are unchanged implementation evidence,
  not a new external-standard or conformance claim.
- Queries and process memory: `query.go`, `memory/store.go`, all four memory
  test files. Tenant scope is selection, not authorization; cursors are unsigned,
  adapter-owned watermarks and do not bind query filters. Stable pages exclude
  later acceptances. Memory capacity counts canonical payload bytes rather than
  heap overhead; cancellation is cooperative. Export callbacks run outside the
  gate, stop at one page and sanitize non-cancellation callback failures.
- PostgreSQL v1: all production and fault/integration files, four embedded
  migrations, [operations](postgresql.md), [retention](retention.md) and
  [privacy](privacy.md). Pool/transaction concurrency remains resource-owned.
  Savepoint failures may force outer rollback; rollback attempts use a detached
  five-second context, not a guarantee of cleanup success. Ordinary append is
  atomic, commit errors unknown. Export retains open rows/connection until it
  returns. Role generation does not execute grants or audit existing privileges.
  Retention plans are sorted by ID for lock ordering, digest-bound but not proof
  of archive custody; apply does not perform archive verification. Immutable ID
  tombstones and accepted-order hold state remain defined by unchanged SQL.

No decision register/specification directory or additional normative standard
version was found. The authoritative format is the package-owned schema-v1
canonical encoding and frozen fixtures, not RFC 8785 JSON canonicalization.
Historical migrations, published checksums, release records and source pins
must remain intact. No runtime defect is repaired by this pass.

## Variants, markers, tests and examples

All 42 tracked Go files are inventoried, including external/internal tests and
helpers. Only `postgres/integration_test.go` has a build constraint (`integration`).
No generated Go files, go:generate directives or unresolved TODO/FIXME/HACK/XXX
markers were found. `postgres/migrations.go` embeds four immutable SQL migrations;
it is not a generated source file and needs no generator directive.

Four fuzz targets: canonical record, hostile construction, cursor, PostgreSQL
config batch bounds. Sixteen benchmarks: fourteen root/reference workload
comparisons, bounded SQL query construction, tagged PostgreSQL workloads.
Four examples cover Builder, Recorder, Chain and Exporter; this pass makes each
fallible step explicit without changing their documented output. Both modules
have goleak TestMain harnesses. Tests, fuzz seeds, benchmark workloads and
integration helpers are investigated as contract evidence, not rewritten to
manufacture documentation counts. Active fuzz, race, leak, database, release,
clean-consumer, multi-major conformance and aggregate campaigns are omitted:
no production, SQL, dependency or capacity behavior changes justify them.

## Verification and remaining questions

Completed with the catalog-pinned Go 1.27.2 producer and `GOWORK=off`:

- `gofmt -w` on the 15 changed Go files.
- A disposable standard-library parser/scanner/AST comparison read all 42 Go
  files, including tagged tests: 427 public declarations/fields/methods and
  exactly one canonical comment per public package. All library executable
  ASTs are unchanged. Tokens are identical except implicit-semicolon insertion
  from expanding `DeliveryAlert`, `PolicyMetadata` and `RetentionApplyResult`
  to document their public fields; normalized tokens and ASTs remain equal.
- Configured and historical API baselines, module manifests/checksums, both
  canonical/integrity fixtures and all four embedded migration bytes match base.
  Build/embed directives are unchanged; no generation output needs updating.
- `golib source-docs check`: 2 module boundaries and 42 Go files, passed.
- `golib docs check --local` and `bash scripts/check-docs.sh --local`: passed,
  including the repository-owned forbidden-compliance-claim guard.
- `go test -mod=readonly . -run '^Example' -count=1 -timeout=2m`: all four
  changed examples passed with unchanged documented output. No unchanged
  runtime suite or integration-tagged service test was executed.
- `go doc -all .`, `go doc -all ./memory`, and nested-module
  `go doc -all .`: all three public package renders passed against their own
  module paths; no root-v2 substitution into the v1 adapter was used.

All three session-owned caches and parser/render resources were removed,
including read-only downloaded modules. Existing `.verification` evidence was
preserved. Final local links, changed-content redacted secret scan, ordinary
complete-diff inspection and Intent/ACK commit hygiene complete the handoff.
The locally built coordinator-owned CLI/scanner are retained by their owner;
their use here is not a hosted CI or public release claim.

Real external boundaries remain caller-owned identity truth/uniqueness,
cooperative dependencies, authorization, key/checkpoint/archive custody, pgx
resource ownership, privileges and retention policy. This source pass does not
qualify deployed PostgreSQL behavior, current hosted CI or a public release.

## Objective Enforcement

The qualified public Tools v2.4.0 binary and immutable reusable CI source
provide the strict tracked-source check through `make source-docs-check`,
the existing local check/CI routes and Required aggregation. Release and
selected security/assurance routes retain their existing controls. This
structural status supplements the rendered semantic audit; it does not
certify prose meaning, generator execution or runtime conformance.
