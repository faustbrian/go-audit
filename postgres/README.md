# audit/postgres

`audit/postgres` is the separately releasable PostgreSQL persistence adapter
for the core [`audit`](https://pkg.go.dev/github.com/faustbrian/go-audit)
contracts. It owns append-only storage, bounded queries and exports,
caller-owned transaction staging, and legal-hold-aware retention. It does not
own PostgreSQL pools, commit caller transactions, manage deployment role
lifecycles, replace the core record and delivery policy, or establish
regulatory compliance.

## Status and compatibility

The module is stable and released independently with `postgres/v*` repository
tags. Its canonical module path is
`github.com/faustbrian/go-audit/postgres`.

The minimum supported Go version is **Go 1.26.6**, matching this module's
`go.mod` and the repository support policy. Repository verification currently
tests Go 1.26.6; other toolchain versions are not part of the recorded support
claim. The adapter supports PostgreSQL 14 through 18. See
[compatibility and assurance](../docs/assurance.md) for the exact evidence and
deployment-owned boundaries.

Add the latest compatible release to an application:

```sh
go get github.com/faustbrian/go-audit/postgres@v1.0.0
```

Applications import the package using its conventional identifier:

```go
import auditpostgres "github.com/faustbrian/go-audit/postgres"
```

## Five-minute quick start

The example assumes the embedded migrations and deployment-specific grants
described in [PostgreSQL operations](../docs/postgresql.md) have already been
applied and `DATABASE_URL` contains writer credentials.

```go
package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/faustbrian/go-audit"
	auditpostgres "github.com/faustbrian/go-audit/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(context.Background()); err != nil {
		// Report through an application-owned, credential-safe diagnostic path.
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dsn, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		return errors.New("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	sink, err := auditpostgres.New(pool, auditpostgres.Config{})
	if err != nil {
		return err
	}
	redactor, err := audit.NewRedactor(audit.RedactionRules{})
	if err != nil {
		return err
	}
	recorder, err := audit.NewRecorder(audit.RecorderConfig{
		Sink: sink, Redactor: redactor, Mode: audit.DeliveryFailClosed,
	})
	if err != nil {
		return err
	}

	builder, err := audit.NewBuilder(audit.BuilderConfig{})
	if err != nil {
		return err
	}
	record, err := builder.Build(audit.RecordInput{
		OccurredAt: time.Now(),
		Action:     "invoice.approved",
		Outcome:    audit.OutcomeSucceeded,
		Actor:      audit.ActorInput{Kind: audit.ActorService, ID: "billing"},
		Subject:    audit.SubjectInput{Type: "invoice", ID: "invoice-42"},
		Changes:    audit.ChangeSetInput{NoChange: true},
	})
	if err != nil {
		return err
	}

	_, err = recorder.Submit(ctx, record)
	return err
}
```

Empty `Config` selects the bounded core defaults. Construction validates the
pool and all non-zero overrides before use. The recorder applies explicit
caller-owned redaction before the adapter receives a record; empty redaction
allowlists remove all extensible attributes and change values.

## Package map and selection

This module contains one public `postgres` package:

- `Store` implements durable append, bounded stable query, and streaming
  export over a caller-owned `pgxpool.Pool`.
- `TxWriter` stages an atomic bounded batch in a caller-owned `pgx.Tx`; only
  the caller commits or rolls back the business transaction.
- `RetentionAdmin` provides separately privileged legal-hold and
  archive-before-delete operations.
- `Migrations`, `FreshInstallPreflightSQL`, and `PrivilegeSQL` expose the
  deployment inputs without performing hidden I/O or role discovery.

Use this module when audit records require durable PostgreSQL persistence and
the deployment can own migrations, credentials, backup, recovery, and role
separation. Use the core `audit` module without this adapter for storage-neutral
contracts, or its `memory` package for bounded process-local tests. Do not use
this adapter as a general event store, log database, migration runner, pool
manager, authorization layer, or partition-management framework.

## Construction, ownership, and lifecycle

`New`, `NewTx`, and `NewRetentionAdmin` validate configuration and retain no
caller-mutable collection aliases. They start no goroutines. Pools and
transactions remain caller-owned, so the adapter has no `Close` or `Shutdown`
method: callers close pools and finish transactions after all adapter calls
return. Values may be used concurrently where their caller-owned pgx resource
permits it; a `TxWriter` remains bound to the ownership and concurrency rules
of its single transaction.

Application-facing database work takes `context.Context` first and observes
caller cancellation and deadlines. Rollback cleanup is the deliberate
exception: once a transaction or savepoint must be aborted, the adapter uses
`context.WithoutCancel` and a fresh five-second timeout so an already-canceled
operation cannot strand database resources. That detached context performs
cleanup only; it does not retry or continue the canceled operation. The
adapter otherwise performs no hidden retry. An unknown append outcome must be
reconciled by record ID and canonical bytes before a caller decides whether to
retry. Stable categories support `errors.Is`, while `audit.AppendOutcomeOf`
distinguishes rejected, committed, and unknown durability outcomes.
`errors.Is(err, auditpostgres.ErrRetryableTransaction)` identifies a
transaction that must be retried in full. Database diagnostics are deliberately
sanitized.

`NewTx` stages each bounded batch behind a savepoint. Ordinary failures roll
back that batch; deadlock or serialization failure aborts the whole caller
transaction. Query results and export callbacks are bounded, ordered, and
closed before return. See the [package API](https://pkg.go.dev/github.com/faustbrian/go-audit/postgres)
and [adapter documentation index](docs/README.md) for the detailed contracts.

## PostgreSQL setup and operations

For a fresh database, execute `FreshInstallPreflightSQL()` and every embedded
migration in one outer transaction with the deployment's migration owner. The
preflight reserves the three fixed roles as `NOLOGIN`; a collision fails
instead of granting temporary access. For an existing installation, commit
migration 3's role neutralization before migration 4 so poisoned legacy
history cannot roll the role safety change back.

Create distinct deployment-specific writer, reader, and retention roles, then
generate and review their grants:

```go
sql, err := auditpostgres.PrivilegeSQL(auditpostgres.RoleNames{
	Writer:    "billing_audit_writer",
	Reader:    "security_audit_reader",
	Retention: "privacy_audit_retention",
})
```

The writer can execute the idempotent append function but cannot directly
select, update, or delete audit rows or retained identity tombstones. Reserve
`RetentionAdmin` for an independently controlled retention pool. The database
encoding must be UTF-8, and migration 4 requires an exclusive enough window to
validate legacy rows and install the durable acceptance boundary within its
30-second lock-acquisition bound.

## Security, performance, and operational boundaries

Redact records before persistence; never store credentials, unrestricted
request or response bodies, or secret-bearing fields. Deployers own transport
security, database credentials, role membership, authorization, tenancy,
backup and restore, legal holds, archive verification, alerting, and incident
response. Report vulnerabilities through the repository's private
[security process](../SECURITY.md).

Queries, exports, batches, decoded records, callbacks, and retention plans are
bounded. The supported schema is intentionally unpartitioned so PostgreSQL can
enforce global record-ID uniqueness; partition rollover is not a supported
tuning option. Benchmark results are comparative evidence, not deployment
capacity promises. Review the [operations guide](../docs/postgresql.md),
[assurance evidence and caveats](../docs/assurance.md), and
[FAQ and troubleshooting](../docs/faq.md) before production adoption.

## Documentation and ecosystem navigation

The adapter exposes no separate testing-helper package. Use the core `memory`
package for process-local unit tests and a deployment-owned PostgreSQL instance
for integration evidence.

- [Adapter documentation index](docs/README.md)
- [PostgreSQL operations and migration guidance](../docs/postgresql.md)
- [Core adoption examples](../docs/adoption.md)
- [API reference](https://pkg.go.dev/github.com/faustbrian/go-audit/postgres)
- [Testing and contributor guidance](../CONTRIBUTING.md)
- [FAQ and troubleshooting](../docs/faq.md)
- [Changelog](CHANGELOG.md)
- [Support](../SUPPORT.md)
- [Security reporting](../SECURITY.md)
- [License](LICENSE)

The versioned [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
explains how independently released modules compose. The
[persistence and durability family guidance](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection)
places this adapter alongside its storage, migration, idempotency, and outbox
companions without introducing a framework or umbrella dependency.
