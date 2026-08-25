#!/usr/bin/env bash
set -euo pipefail

temporary="$(mktemp -d "${TMPDIR:-/tmp}/golib-audit-consumer.XXXXXX")"
cleanup() {
    case "${temporary}" in
        "${TMPDIR:-/tmp}"/golib-audit-consumer.*)
            chmod -R u+w "${temporary}" 2>/dev/null || true
            find "${temporary}" -depth -delete
            ;;
    esac
}
trap cleanup EXIT HUP INT TERM

mkdir "${temporary}/core"
cd "${temporary}/core"
GOWORK=off go mod init example.com/audit-consumer
GOWORK=off go mod edit \
    -require=github.com/faustbrian/go-audit@v1.0.0
mkdir consumer
printf '%s\n' \
    'package consumer' \
    'import (' \
    '  "context"' \
    '  "github.com/faustbrian/go-audit"' \
    '  "github.com/faustbrian/go-audit/memory"' \
    ')' \
    'var _ = context.Background' \
    'var _ = audit.NewBuilder' \
    'var _ = audit.NewRecorder' \
    'var _ = audit.NewChain' \
    'var _ = audit.NewQuery' \
    'var _ = audit.NewRetentionPlan' \
    'var _ = memory.New' > consumer/consumer.go
GOWORK=off go mod tidy
GOWORK=off go test ./...

mkdir "${temporary}/postgres"
cd "${temporary}/postgres"
GOWORK=off go mod init example.com/audit-postgres-consumer
GOWORK=off go mod edit \
    -require=github.com/faustbrian/go-audit@v1.0.0 \
    -require=github.com/faustbrian/go-audit/postgres@v1.0.0 \
    -require=github.com/faustbrian/go-postgres@v1.0.1
mkdir consumer
printf '%s\n' \
    'package consumer' \
    'import (' \
    '  "github.com/faustbrian/go-audit/postgres"' \
    ')' \
    'var _ = postgres.New' \
    'var _ = postgres.NewTx' \
    'var _ = postgres.NewRetentionAdmin' \
    'var _ = postgres.Migrations' \
    'var _ = postgres.PrivilegeSQL' > consumer/consumer.go
GOWORK=off go mod tidy
GOWORK=off go test ./...

printf '%s\n' 'clean audit consumers compile'
