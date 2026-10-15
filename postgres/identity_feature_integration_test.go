//go:build integration

package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/faustbrian/go-authorization/v3"
	"github.com/faustbrian/go-clock"
	identity "github.com/faustbrian/go-identity"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
	pgdatabase "github.com/faustbrian/go-postgres"
	"github.com/jackc/pgx/v5"
)

// Native and outbox SQL seams isolate the real audit contributor's composition
// with public Coordinator/Work. This is not a production session issuer.
type auditNativePhase struct {
	operation identity.FeatureOperation
	fail      bool
}

func (*auditNativePhase) Descriptor() identitypostgres.ContributorDescriptor {
	return identitypostgres.ContributorDescriptor{Owner: "identity/session/postgres", ContractID: "session.domain", Version: 1}
}
func (p *auditNativePhase) FeatureOperations() []identity.FeatureOperation {
	return []identity.FeatureOperation{p.operation}
}
func (p *auditNativePhase) ValidateFeatureCommand(command identity.Command, plan identitypostgres.CommandPlan) error {
	s, ok := command.Feature()
	if !ok || s.Operation != p.operation || len(s.Fields) != 1 || s.Fields[0].Name != "value" || s.Fields[0].Value.Kind != identity.FeatureText || plan.SubjectType != "session" || len(plan.Audits) != 2 || plan.Lifecycle == nil || plan.Lifecycle.BeforeVersion != 0 || plan.Lifecycle.AfterVersion != 1 {
		return identity.InvalidInputError{Field: "test_native"}
	}
	return nil
}
func (p *auditNativePhase) ApplyFeature(ctx context.Context, w identitypostgres.Work, command identity.Command) (identity.FeatureReceipt, error) {
	s, _ := command.Feature()
	if _, e := w.Exec(ctx, `INSERT INTO public.audit_test_native VALUES($1,$2)`, string(s.Binding.CommandID), s.Binding.RecordID); e != nil {
		return identity.FeatureReceipt{}, e
	}
	if p.fail {
		return identity.FeatureReceipt{}, identity.ForbiddenError{}
	}
	return identity.FeatureReceipt{Operation: s.Operation, AfterVersion: 1}, nil
}
func (*auditNativePhase) RecoverParticipant(ctx context.Context, w identitypostgres.RecoveryWork) (identitypostgres.RecoveryStatus, error) {
	return auditCompositionEvidence(ctx, w, "public.audit_test_native")
}

type auditLifecyclePhase struct{}

func (*auditLifecyclePhase) Descriptor() identitypostgres.ContributorDescriptor {
	return identitypostgres.ContributorDescriptor{Owner: "outbox/postgres", ContractID: "identity.lifecycle", Version: 1}
}
func (*auditLifecyclePhase) PersistLifecycle(ctx context.Context, w identitypostgres.Work) error {
	entries, e := w.StagedLifecycle()
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if _, e := w.Exec(ctx, `INSERT INTO public.audit_test_outbox VALUES($1)`, string(entry.CommandID)); e != nil {
			return e
		}
	}
	return nil
}
func (*auditLifecyclePhase) RecoverParticipant(ctx context.Context, w identitypostgres.RecoveryWork) (identitypostgres.RecoveryStatus, error) {
	return auditCompositionEvidence(ctx, w, "public.audit_test_outbox")
}
func auditCompositionEvidence(ctx context.Context, w identitypostgres.RecoveryWork, table string) (identitypostgres.RecoveryStatus, error) {
	b, e := w.Scope()
	if e != nil {
		return identitypostgres.RecoveryUnknown, e
	}
	// table is a fixed fixture constant, never caller input.
	rows, e := w.Query(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE command_id=$1)`, string(b.CommandID))
	if e != nil {
		return identitypostgres.RecoveryUnknown, e
	}
	defer rows.Close()
	var found bool
	if !rows.Next() || rows.Scan(&found) != nil || rows.Next() || rows.Err() != nil {
		return identitypostgres.RecoveryUnknown, identity.UnavailableError{}
	}
	if found {
		return identitypostgres.RecoveryCommitted, nil
	}
	return identitypostgres.RecoveryNotCommitted, nil
}

type auditCompositionPolicy struct{}

func (auditCompositionPolicy) Decide(context.Context, authorization.DecisionContext) (authorization.Decision, error) {
	return authorization.Decision{Outcome: authorization.Allow, Revision: 9}, nil
}

func testIdentityFeatureComposition(t *testing.T, ctx context.Context, config *pgx.ConnConfig, conn *pgx.Conn) {
	t.Helper()
	// Install the owned kernel schema in this task's fresh database. The kernel's
	// separate migration-ledger tests own populated-state and retry evidence.
	history, e := (identitypostgres.MigrationSource{}).Load(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, migration := range history {
		if _, e := conn.Exec(ctx, migration.UpSQL()); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := conn.Exec(ctx, `CREATE TABLE public.audit_test_native(command_id text PRIMARY KEY,record_id text);CREATE TABLE public.audit_test_outbox(command_id text PRIMARY KEY)`); e != nil {
		t.Fatal(e)
	}
	// ConnString retains its original database even after ConnConfig.Database
	// changes. Apply the selected disposable database to the pool explicitly.
	pool, e := pgdatabase.Connect(ctx, pgdatabase.Config{DSN: config.ConnString(), MaxConns: 2, Configure: func(native *pgdatabase.PoolConfig) error { native.ConnConfig.Database = config.Database; return nil }})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if pool.Close(cleanup) != nil {
			t.Error("composition pool cleanup")
		}
	}()
	repo, e := identitypostgres.NewRepository(pool, identitypostgres.Config{MaximumDuration: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	auditing, e := NewIdentityContributor(Config{})
	if e != nil {
		t.Fatal(e)
	}
	_, template := featureAuditEntries(t)
	native := &auditNativePhase{operation: template[0].Feature}
	registry, e := identitypostgres.NewContributorRegistry([]identitypostgres.Contributor{repo, auditing, &auditLifecyclePhase{}, native})
	if e != nil {
		t.Fatal(e)
	}
	coordinator, e := identitypostgres.NewCoordinator(pool, registry, identitypostgres.CoordinatorConfig{MaximumDuration: time.Second, LeaseDuration: time.Minute, CurrentFingerprintKeyVersion: 1, RetainedFingerprintKeys: map[uint32][32]byte{1: {1}}, QueryPolicy: &identitypostgres.CommandQueryPolicy{Clock: clock.System{}, PolicyVersion: 9, Authorization: auditCompositionPolicy{}}})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"commit", "abort", "recovery"} {
		t.Run(mode, func(t *testing.T) {
			binding, _ := featureAuditEntries(t)
			// Seed records are used only to compute their scope digest; coordinator
			// creates fresh exact records, event IDs and primary timestamps.
			binding.CommandID, e = identity.NewCommandID()
			if e != nil {
				t.Fatal(e)
			}
			command, e := identity.NewFeatureCommand(identity.FeatureCommandSpec{Binding: binding, Operation: native.operation, Fields: []identity.FeatureField{{Name: "value", Value: identity.FeatureValue{Kind: identity.FeatureText, Text: "synthetic"}}}})
			if e != nil {
				t.Fatal(e)
			}
			plan := identitypostgres.CommandPlan{Contributors: registry.Descriptors(), Feature: &identitypostgres.FeatureRegistration{Contributor: native.Descriptor(), Operation: native.operation}, SubjectType: "session", Lifecycle: &identitypostgres.LifecyclePlan{EventType: "session.created.v1", AfterVersion: 1}}
			scoped := featureAuditEntriesForBinding(t, binding)
			for _, entry := range scoped {
				plan.Audits = append(plan.Audits, entry.Plan)
			}
			plan, e = coordinator.PrepareCommandPlan(binding, command, plan)
			if e != nil {
				t.Fatal(e)
			}
			reservation, _, e := coordinator.ReserveCommand(ctx, binding, command, plan)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "recovery" {
				id, _ := base64.RawURLEncoding.DecodeString(string(binding.CommandID))
				if _, e := conn.Exec(ctx, `UPDATE identity.commands SET lease_until=clock_timestamp()-interval '1 second' WHERE command_id=$1`, id); e != nil {
					t.Fatal(e)
				}
				query := identity.RecoveryQuery{CommandID: binding.CommandID, TenantID: binding.TenantID, UserID: binding.UserID, ActorID: binding.ActorID, ActorKind: binding.ActorKind, OperationID: binding.OperationID, RecordID: binding.RecordID}
				caller := authorization.DecisionContext{PolicyRevision: 9, Request: authorization.Request{Subject: authorization.Subject{ID: authorization.SubjectID(binding.ActorID), Kind: binding.ActorKind}, Tenant: authorization.TenantID(binding.TenantID), Action: authorization.Action(binding.OperationID), Resource: authorization.Resource{Type: "identity.user", ID: authorization.ResourceID(binding.UserID)}}}
				result, e := coordinator.RecoverCommand(ctx, query, caller)
				var unavailable identity.UnavailableError
				if !errors.As(e, &unavailable) || result.Outcome != identity.CommitOutcomeNotCommitted {
					t.Fatal("real multi-audit recovery", e)
				}
			} else {
				w, e := coordinator.Begin(ctx, reservation)
				if e != nil {
					t.Fatal(e)
				}
				defer coordinator.Abort(w, identity.UnavailableError{})
				native.fail = mode == "abort"
				defer func() { native.fail = false }()
				result, e := coordinator.ApplyFeature(ctx, w, command)
				if mode == "abort" {
					result, e = coordinator.Abort(w, e)
					if result.Outcome != identity.CommitOutcomeNotCommitted || e == nil {
						t.Fatal("real audit abort", e)
					}
				} else {
					if e != nil {
						t.Fatal(e)
					}
					result, e = coordinator.Commit(w, result)
					if e != nil || result.Outcome != identity.CommitOutcomeCommitted {
						t.Fatal("real audit commit", e)
					}
				}
			}
			var audits, domain, outbox int
			id, _ := base64.RawURLEncoding.DecodeString(string(binding.CommandID))
			e = conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM audit.identity_command_bindings WHERE command_id=$1),(SELECT count(*) FROM public.audit_test_native WHERE command_id=$2),(SELECT count(*) FROM public.audit_test_outbox WHERE command_id=$2)`, id, string(binding.CommandID)).Scan(&audits, &domain, &outbox)
			want := 0
			if mode == "commit" {
				want = 1
			}
			if e != nil || audits != 2 || domain != want || outbox != want {
				t.Fatal("real composition atomicity", audits, domain, outbox, e)
			}
		})
	}
}
