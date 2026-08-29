#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
required=(
    README.md CHANGELOG.md CONTRIBUTING.md LICENSE SECURITY.md SUPPORT.md
    docs/README.md docs/api.md docs/adoption.md docs/delivery.md
    docs/threat-model.md docs/privacy.md docs/integrity.md
    docs/query-export.md docs/postgresql.md docs/retention.md
    docs/incident-use.md docs/faq.md
    postgres/README.md postgres/CHANGELOG.md postgres/LICENSE
    scripts/check-clean-consumer.sh
)

cd "${root}"
for path in "${required[@]}"; do
    test -s "${path}" || {
        printf 'missing required documentation: %s\n' "${path}" >&2
        exit 1
    }
done

packages="$(go list ./...)"
while IFS= read -r package; do
	go doc "${package}" >/dev/null
done <<< "${packages}"

source_list="$(mktemp)"
trap 'rm -f "${source_list}"' EXIT
if ! git ls-files --cached --others --exclude-standard -z -- \
    '*.md' '*.go' \
    ':(exclude).golib-tooling/**' \
    ':(exclude).verification/**' > "${source_list}"; then
    printf 'failed to enumerate repository-owned source files\n' >&2
    exit 1
fi

while IFS= read -r -d '' path; do
    test -f "${path}" || {
        printf 'source file disappeared during documentation scan: %s\n' "${path}" >&2
        exit 1
    }

    if grep -niE 'automatically compliant|provides non.repudiation|event store is (a |an )?compliant audit' -- "${path}"; then
        printf 'forbidden compliance or integrity claim detected\n' >&2
        exit 1
    else
        status=$?
        test "${status}" -eq 1 || {
            printf 'failed to scan source file: %s\n' "${path}" >&2
            exit 1
        }
    fi
done < "${source_list}"

printf 'required audit documentation and package docs are present\n'
