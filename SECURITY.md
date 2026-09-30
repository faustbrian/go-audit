# Security Policy

Report vulnerabilities through this repository's
[private security advisory](https://github.com/faustbrian/go-audit/security/advisories/new)
form. Never put credentials, raw authorization headers, request or response
bodies, tenant data, or exploit details in a public issue.

The root module and the separately releasable PostgreSQL module each have a
published v1 line. The latest v1 patch release for each module is supported
unless announced otherwise. Fixes land on the default branch before the
affected module is released independently.

The default branch prepares the v2 root module with bounded integrity and
record-validation work. Use v2 only after its public release gates pass and a
`v2.0.0` tag is published. The PostgreSQL module continues to use root v1
independently; it does not accept v2 record types.

Maintainers assess reports privately, reproduce the affected public contract,
and coordinate a fix, affected-version range, migration advice, and disclosure
with the reporter. Severity reflects exploit prerequisites and deployment
impact, not a compliance claim. Do not disclose exploit details before the
coordinated advisory and patched release are available. If private reporting
is unavailable, open a public issue requesting a private contact without
including sensitive evidence.

Deployers own authentication, authorization, tenancy discovery, transport
security, database credentials, redaction policy, retention, legal holds,
pseudonymization, privileged reads, key custody, alert delivery, and incident
response. Restrict ordinary application roles from update and delete access.
Hash chains detect selected integrity failures but do not provide
non-repudiation without independent trusted key and checkpoint custody.
Caller-provided key providers and observers execute synchronously. Deployers
must bound them, honor cancellation, make them concurrency-safe, and review
their behavior before changing backends or latency budgets.
