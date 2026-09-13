# Security Policy

Report vulnerabilities through this repository's
[private security advisory](https://github.com/faustbrian/go-audit/security/advisories/new)
form. Never put credentials, raw authorization headers, request or response
bodies, tenant data, or exploit details in a public issue.

The root module and the separately releasable PostgreSQL module each have a
published v1 line. The latest v1 patch release for each module is supported
unless announced otherwise. Fixes land on the default branch before the
affected module is released independently.

The default branch currently prepares an unpublished, non-releasable v2 root
module for bounded integrity verification. Production consumers must remain on
released root v1 until v2 is published; the PostgreSQL module continues to use
root v1 independently.

Deployers own authentication, authorization, tenancy discovery, transport
security, database credentials, redaction policy, retention, legal holds,
pseudonymization, privileged reads, key custody, alert delivery, and incident
response. Restrict ordinary application roles from update and delete access.
Hash chains detect selected integrity failures but do not provide
non-repudiation without independent trusted key and checkpoint custody.
Caller-provided key providers and observers execute synchronously. Deployers
must bound them, honor cancellation, make them concurrency-safe, and review
their behavior before changing backends or latency budgets.
