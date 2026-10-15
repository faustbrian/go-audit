package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/faustbrian/go-audit"
	"github.com/faustbrian/go-authorization/v3"
	identity "github.com/faustbrian/go-identity"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
)

// Risk plan: native scope must not be coerced to kernel scope; every staged
// slot persists; recovery requires unanimous retained absence, not a partial
// prefix. Existing real PostgreSQL identity SQL tests prove routine semantics.
func featureAuditEntries(t *testing.T) (identity.CommandBinding, []identitypostgres.StagedAudit) {
	t.Helper()
	// This participant fixture is not authenticated session issuance. The real
	// session-issue operation requires signed-original final proof at reservation.
	binding := identity.CommandBinding{CommandID: "AAAAAAAAAAAAAAAAAAAABA", TenantID: "tenant-a", UserID: "user-a", ActorID: "actor-a", ActorKind: authorization.SubjectUser, OperationID: "identity.session.audit-fixture", PolicyID: "policy", PolicyVersion: 1, SchemaID: "session-issue", SchemaVersion: 1, CorrelationID: "correlation", RecordID: "session-a"}
	return binding, featureAuditEntriesForBinding(t, binding)
}

func featureAuditEntriesForBinding(t *testing.T, binding identity.CommandBinding) []identitypostgres.StagedAudit {
	t.Helper()
	feature := identity.FeatureOperation{Owner: "identity/session", OperationID: binding.OperationID, SchemaID: binding.SchemaID, SchemaVersion: 1, RecordRequirement: identity.FeatureRecordRequired}
	entries := make([]identitypostgres.StagedAudit, 2)
	for i, class := range []string{"standard", "protected"} {
		id := []string{"event-a", "event-b"}[i]
		builder, e := audit.NewBuilder(audit.BuilderConfig{IDGenerator: func() (string, error) { return id, nil }})
		if e != nil {
			t.Fatal(e)
		}
		action := []string{"session.issue", "session.authority"}[i]
		record, e := builder.Build(audit.RecordInput{Action: action, OccurredAt: time.Now().UTC(), Outcome: audit.OutcomeSucceeded, ReasonCode: "completed", Actor: audit.ActorInput{Kind: audit.ActorHuman, ID: string(binding.ActorID)}, Subject: audit.SubjectInput{Type: "session", ID: binding.RecordID}, Context: audit.ContextInput{TenantID: string(binding.TenantID), CorrelationID: binding.CorrelationID}, Changes: audit.ChangeSetInput{NoChange: true}, Policy: audit.PolicyMetadata{PolicyID: binding.PolicyID, Version: "1"}, Attributes: map[string]string{"command.id": string(binding.CommandID), "identity.profile": "identity.security.v1", "retention.class": class, "aggregate.type": "session", "aggregate.id": binding.RecordID, "aggregate.before_version": "0", "aggregate.after_version": "1"}})
		if e != nil {
			t.Fatal(e)
		}
		redactor, e := audit.NewRedactor(audit.RedactionRules{AllowedAttributes: []string{"command.id", "identity.profile", "retention.class", "aggregate.type", "aggregate.id", "aggregate.before_version", "aggregate.after_version"}})
		if e != nil {
			t.Fatal(e)
		}
		record, e = redactor.Redact(context.Background(), record)
		if e != nil {
			t.Fatal(e)
		}
		digest, e := identitypostgres.FeatureAuditBindingDigest(binding, feature, "session", record)
		if e != nil {
			t.Fatal(e)
		}
		entries[i] = identitypostgres.StagedAudit{Feature: feature, SubjectType: "session", Record: record, Plan: identitypostgres.AuditPlan{EventType: action, RetentionClass: class, SchemaVersion: 1, EventIndex: uint16(i), BindingDigest: digest, RetentionPolicyVersion: 1, RequiredRecordCount: 1, MaxRecordBytes: 16384, MaxAggregateBytes: 16384, Required: true}}
	}
	return entries
}

func TestIdentityNativeFeatureAuditsPreserveScope(t *testing.T) {
	c, e := NewIdentityContributor(Config{})
	if e != nil {
		t.Fatal(e)
	}
	binding, entries := featureAuditEntries(t)
	calls := 0
	e = c.persist(context.Background(), func(_ context.Context, sql string, args ...any) (int16, error) {
		calls++
		if sql != identityAppendSQL || args[2] != int32(calls-1) {
			t.Fatal("audit slot lost")
		}
		return 1, nil
	}, binding, entries)
	if e != nil || calls != 2 {
		t.Fatal("native audit rejected", e, calls)
	}
	for _, alter := range []func(*identitypostgres.StagedAudit){func(e *identitypostgres.StagedAudit) { e.Feature.Owner = "identity/password" }, func(e *identitypostgres.StagedAudit) { e.SubjectType = "credential" }, func(e *identitypostgres.StagedAudit) { e.Plan.RetentionClass = "standard" }} {
		changed := append([]identitypostgres.StagedAudit(nil), entries...)
		alter(&changed[1])
		e = c.persist(context.Background(), func(context.Context, string, ...any) (int16, error) {
			t.Fatal("substituted scope wrote prefix")
			return 0, nil
		}, binding, changed)
		if e == nil {
			t.Fatal("substituted native scope accepted")
		}
	}
}

func TestIdentityMultiSlotRecoveryRequiresUnanimousEvidence(t *testing.T) {
	c, e := NewIdentityContributor(Config{})
	if e != nil {
		t.Fatal(e)
	}
	binding, entries := featureAuditEntries(t)
	plans := []identitypostgres.AuditPlan{entries[0].Plan, entries[1].Plan}
	for _, statuses := range [][]int16{{0, 0}, {1, 1}, {0, 1}, {1, 0}, {0, 2}, {2, 0}} {
		calls := 0
		got, e := c.recoverPlans(context.Background(), func(context.Context, string, ...any) (int16, error) { v := statuses[calls]; calls++; return v, nil }, binding, plans)
		want := identitypostgres.RecoveryUnknown
		if statuses[0] == 0 && statuses[1] == 0 {
			want = identitypostgres.RecoveryNotCommitted
		}
		if statuses[0] == 1 && statuses[1] == 1 {
			want = identitypostgres.RecoveryCommitted
		}
		if e != nil || got != want || calls != 2 {
			t.Fatal("incorrect multi-slot evidence", statuses, got, e, calls)
		}
	}
}
