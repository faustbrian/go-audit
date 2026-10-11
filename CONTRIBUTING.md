# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected module's docs.
2. Run the standalone Go baseline below for the affected module.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.

Full scheduled and release mutation gates must finish with zero surviving viable
mutants. Local mutation is optional, not a universal contribution prerequisite.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Use the checksum-pinned public Tools v2.1.3 release declared in `.golib.yaml`.
Download its released `golib` binary rather than building the verifier from source.

Use the declared Go 1.27.2 producer; both module minima remain Go 1.27.0.
Test each affected module independently of the workspace and sibling checkouts:

```bash
GOWORK=off go test ./...
(cd postgres && GOWORK=off go test ./...)
```

These ordinary tests run deterministic fuzz seeds and saved corpus regressions,
not active exploration. Both modules declare `required_services: []`, but the
PostgreSQL module's manifest selects the `integration` tag. Its tagged tests
start digest-pinned PostgreSQL fixtures through `postgrestest` and require
Docker, including for `make check` and `make ci`. The untagged adapter command
above does not run that live boundary. Root v2 and the independently released
PostgreSQL v1 adapter remain separate type and compatibility contracts.

With the checksum-pinned public verifier selected by `.golib.yaml`, use:

```bash
make inventory
make check
make ci
```

Both Make routes retain the offline required-doc/link and false-compliance claim
refusal guard, then run bounded local checks, including one race command for the
memory adapter's prompt append/query cancellation while its store lock is held,
count one with a two-minute bound. Base-free checks run seeds/corpora;
changed-source CI selects bounded active smoke for causally affected targets.
Ordinary CI consumes the public verifier without building it from source.
Hosted CI also runs the checkout-only documentation/claim refusal boundary
with a two-minute limit; Required needs both that boundary and the shared
workflow to succeed.

Short active exploration selects one target; optional local mutation selects
one package:

```bash
GOMAXPROCS=2 GOWORK=off go test -run '^$' -fuzz '^FuzzCanonicalRecord$' -fuzztime=2s -parallel=1 -timeout=2m .
golib mutation --module . --package ./memory
```

`make check-full` retains all four native 10000x fuzz campaigns, 100-count
duplicate stress, 25-count high-cardinality soak, tagged PostgreSQL writer and
idempotency qualification, both module benchmarks, package documentation and
published clean consumers, plus full coverage, race, mutation, API and security
qualification. Persistence, delivery, retention and integrity changes still
need their affected fault and real-database tests; API changes need compatibility
and direct consumers. Release rehearsal remains
`golib release dry-run --module .` (or `--module postgres` for that independent
adapter). Report skipped, unavailable and failed boundaries; local checks are
not hosted-CI or release proof.

Repository verification is provided by the pinned `go-library-tools` release.
Use `golib check --local --module postgres` for a bounded module-scoped run when investigating
the PostgreSQL adapter; do not replace the repository-owned configuration or
verification evidence with local tooling.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.
