# Threat model and compliance boundary

The library treats every caller, record field, sink result, database response,
and export consumer as untrusted input. Validation and defensive ownership
limit field injection and alias mutation. Explicit tenant scopes reduce tenant
confusion but do not authenticate a tenant. Actor kinds make anonymous, unknown,
system, human, service, and delegated identities explicit, but a compromised
writer can still forge actors, tenants, occurrence times, and business facts.

Redaction and default-deny field policies address secret leakage before
persistence and diagnostics. Bounded records, batches, queries, exports, and
buffer capacities provide backpressure for storage outage and disk-exhaustion
conditions; they cannot manufacture durable capacity. Fail-closed,
fail-open-with-alert, and durable-buffer modes make omission visible. Stable
record IDs and canonical bytes make identical retries idempotent and conflicting
duplicates rejectable. Commit errors remain unknown until reconciled.

Record validation checks text byte limits before UTF-8 scanning or allocating
privacy-normalization buffers. Attribute and change-map bytes and descriptions
are capped by the record-byte ceiling even when a caller configures larger
field budgets. Caller-owned input allocation and aggregate concurrent request
admission remain outside this per-operation bound.

Canonical encoding plus optional chains, external checkpoints, and Merkle roots
detect alteration, duplication, reordering, missing links, truncation, and
backdated records only relative to independently retained ordering evidence.
Each integrity verification or Merkle-root operation accepts at most 1,000
records before allocating or invoking a key provider. Cancellation is checked
between verification records. A key provider or observer that ignores context
can still block its calling goroutine because the library does not detach
caller callbacks into leak-prone background goroutines.
They do not prevent a compromised writer from omitting a record or a privileged
operator from replacing both the database and its co-located checkpoints.
Readers and export consumers can exfiltrate everything they are authorized to
read; malicious exports must therefore be tenant-bounded, access-controlled by
the caller, streamed to restricted storage, and verified before use.

It does not defend against a fully privileged database administrator, stolen
application and integrity keys, compromised writers or readers, compromised
caller policy, a caller that omits an action, false actor or tenant inputs,
malicious archive operators, or destruction of both records and independently
held checkpoints. Hashing alone does not prove who created a record and does
not provide non-repudiation.

The module does not decide authentication, authorization, read privileges,
business policy, action vocabularies, transport middleware, tenancy, legal
holds, erasure exceptions, or regulatory applicability. Deployment-specific
controls and evidence remain necessary for any compliance claim.

## Conditional residual risks

| Surface | Owner and rationale | Mitigation | Review condition |
| --- | --- | --- | --- |
| Actor facts, tenant scope, and omitted events | Application owner; values and explicit scopes are not authentication or proof an action occurred. | Authorize writes and reads, derive identities from trusted context, and reconcile required events. | Before changing identity, tenant routing, or fail-open policy. |
| Permitted record contents | Application privacy owner; syntactically valid identity fields and explicitly allowed descriptions or attributes can contain secrets. | Use default-deny redaction, restrict allowances, and avoid credentials in all identity fields. | Before adding an allowed field or changing a redactor. |
| Aggregate resource use | Application operator; per-record and batch bounds do not limit concurrent callers or backend capacity. | Bound admission, memory-store capacity, query costs, and durable-buffer capacity. | Before increasing limits, concurrency, or backend workloads. |
| Synchronous collaborators | Callback owner; clocks, ID generators, key providers, observers, and sinks are caller implementations, not preemptible library workers. | Keep local callbacks bounded; honor context and use dependency deadlines for blocking work. | Before replacing callbacks or changing latency budgets. |
| PostgreSQL driver input | Database and adapter operator; the independent v1 adapter receives driver-allocated row values before canonical validation and hashing. | Retain schema byte constraints, least privilege, row-byte and connection limits, and driver deadlines. | Before changing schema privileges, drivers, or accepting an untrusted database. |
| Integrity evidence and archives | Key and archive custodian; a writer can omit events or replace colocated checkpoints. | Retain independent trusted checkpoints and keys; bound and authorize exports; verify ordered archives. | Before changing custody, archive formats, or retention/legal-hold policy. |
| Supported dependencies and disclosure | Maintainers; vulnerabilities can arise in dependencies or deployment-specific inputs. | Review advisories and pinned sources, reproduce reported contracts, and coordinate fixes and disclosure. | On a dependency advisory, public release, or new credible report. |
