# audit

[![CI](https://github.com/faustbrian/go-audit/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-audit/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-audit/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-audit.svg)](https://pkg.go.dev/github.com/faustbrian/go-audit)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-audit?sort=semver)](https://github.com/faustbrian/go-audit/releases)
[![Go](https://img.shields.io/badge/go-1.26.6-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`audit` is an infrastructure-neutral Go library for immutable-by-contract,
security-relevant and business-relevant records. It records who did what, to
which stable resource, when, from where, and with what outcome.

Audit records are deliberately separate from logs, traces, metrics, domain
events, and event-sourcing history. Using an event store does not make that
store a compliant audit trail, and using this library does not by itself
establish legal or regulatory compliance.

## Packages

- `github.com/faustbrian/go-audit`: records, validation, delivery policy,
  privacy, querying, export, integrity, retention, and safe observation hooks.
- `github.com/faustbrian/go-audit/memory`: bounded process-local adapter
  for tests; it is not durable.
- `github.com/faustbrian/go-audit/postgres`: separately releasable
  PostgreSQL durable adapter and caller-owned transaction writer.

## Quick start

```go
builder, err := audit.NewBuilder(audit.BuilderConfig{})
if err != nil {
    return err
}
record, err := builder.Build(audit.RecordInput{
    OccurredAt: time.Now(),
    Action:     "invoice.approved",
    Outcome:    audit.OutcomeSucceeded,
    Actor:      audit.ActorInput{Kind: audit.ActorHuman, ID: userID},
    Subject:    audit.SubjectInput{Type: "invoice", ID: invoiceID},
    Context:    audit.ContextInput{TenantID: tenantID},
    Changes:    audit.ChangeSetInput{NoChange: true},
})
if err != nil {
    return err
}
redactor, err := audit.NewRedactor(audit.RedactionRules{})
if err != nil {
    return err
}
recorder, err := audit.NewRecorder(audit.RecorderConfig{
    Sink: durableSink, Redactor: redactor, Mode: audit.DeliveryFailClosed,
})
if err != nil {
    return err
}
_, err = recorder.Submit(ctx, record)
return err
```

The caller must select fail-closed, fail-open-with-alert, or durable-buffer
delivery. The library never silently discards a record and performs no hidden
retry. Repeating the same record ID and canonical bytes is idempotent.

See the [documentation index](docs/README.md), [threat model](docs/threat-model.md),
[delivery semantics](docs/delivery.md), [privacy policy boundary](docs/privacy.md),
and [PostgreSQL operations](docs/postgresql.md).

The versioned [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and [package-family selection guidance](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection)
describe the shared design language, related packages, and composition rules.

## License

MIT. See [LICENSE](LICENSE).
