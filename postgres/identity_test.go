package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	identity "github.com/faustbrian/go-identity"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
)

func TestIdentityContributorDoesNotOwnTransactions(t *testing.T) {
	c, e := NewIdentityContributor(Config{})
	if e != nil || c == nil {
		t.Fatal(e)
	}
	var _ identitypostgres.AuditContributor = c
	var _ identitypostgres.RecoveryContributor = c
	d := c.Descriptor()
	if d.Owner != "audit/postgres" || d.ContractID != "identity.security_audit" || d.Version != 1 {
		t.Fatal("wrong contributor identity")
	}
	if e := c.PersistAudits(context.Background(), identitypostgres.Work{}); e == nil {
		t.Fatal("zero Work accepted")
	}
	// This is deliberate invalid-input coverage, not an omitted live context.
	var missingContext context.Context
	if e := c.PersistAudits(missingContext, identitypostgres.Work{}); e == nil {
		t.Fatal("nil context accepted")
	}
	if e := (*IdentityContributor)(nil).PersistAudits(context.Background(), identitypostgres.Work{}); e == nil {
		t.Fatal("nil contributor accepted")
	}
	if c, e := NewIdentityContributor(Config{MaxBatchBytes: -1}); e == nil || c != nil {
		t.Fatal("invalid config admitted")
	}
}

func TestIdentityRecoveryStatusAndFailureMapping(t *testing.T) {
	c, err := NewIdentityContributor(Config{})
	if err != nil {
		t.Fatal(err)
	}
	binding := identity.CommandBinding{CommandID: "AAAAAAAAAAAAAAAAAAAABA", TenantID: "tenant-a", OperationID: "identity.user.create"}
	plan := identitypostgres.AuditPlan{EventType: "identity.user.create", SchemaVersion: 1, EventIndex: 65535, BindingDigest: [32]byte{1}, RetentionPolicyVersion: ^uint64(0), RequiredRecordCount: 1, Required: true}
	for _, test := range []struct {
		name      string
		value     int16
		want      identitypostgres.RecoveryStatus
		wantError bool
	}{
		{"absent", 0, identitypostgres.RecoveryNotCommitted, false},
		{"exact", 1, identitypostgres.RecoveryCommitted, false},
		{"conflicting", 2, identitypostgres.RecoveryUnknown, false},
		{"negative unknown status", -1, identitypostgres.RecoveryUnknown, true},
		{"positive unknown status", 3, identitypostgres.RecoveryUnknown, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, err := c.recover(context.Background(), func(_ context.Context, sql string, args ...any) (int16, error) {
				if sql != identityRecoverySQL || len(args) != 5 || args[0] != "tenant-a" || len(args[1].([]byte)) != 16 || args[2] != int32(65535) || args[3] != "18446744073709551615" || len(args[4].([]byte)) != 32 {
					t.Fatal("changed frozen evidence input")
				}
				return test.value, nil
			}, binding, plan)
			if status != test.want || (err != nil) != test.wantError {
				t.Fatal("incorrect recovery classification", status, err)
			}
		})
	}
	cause := errors.New("private driver details")
	status, err := c.recover(context.Background(), func(context.Context, string, ...any) (int16, error) { return 0, cause }, binding, plan)
	if status != identitypostgres.RecoveryUnknown || err == nil || strings.Contains(err.Error(), cause.Error()) {
		t.Fatal("database failure became conclusive or leaked details")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	status, err = c.recover(ctx, func(context.Context, string, ...any) (int16, error) {
		t.Fatal("cancelled recovery queried")
		return 0, nil
	}, binding, plan)
	if status != identitypostgres.RecoveryUnknown || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	status, err = c.RecoverParticipant(ctx, identitypostgres.RecoveryWork{})
	if status != identitypostgres.RecoveryUnknown || !errors.Is(err, context.Canceled) {
		t.Fatal("public cancellation lost", err)
	}
	for _, mutate := range []func(*identitypostgres.AuditPlan){
		func(p *identitypostgres.AuditPlan) { p.Required = false },
		func(p *identitypostgres.AuditPlan) { p.RequiredRecordCount = 0 },
		func(p *identitypostgres.AuditPlan) { p.RequiredRecordCount = 2 },
		func(p *identitypostgres.AuditPlan) { p.RetentionPolicyVersion = 0 },
		func(p *identitypostgres.AuditPlan) { p.BindingDigest = [32]byte{} },
		func(p *identitypostgres.AuditPlan) { p.SchemaVersion = 2 },
		func(p *identitypostgres.AuditPlan) { p.EventType = "" },
	} {
		invalid := plan
		mutate(&invalid)
		status, err := c.recover(context.Background(), func(context.Context, string, ...any) (int16, error) { t.Fatal("invalid plan queried"); return 0, nil }, binding, invalid)
		if status != identitypostgres.RecoveryUnknown || err == nil {
			t.Fatal("invalid plan conclusive")
		}
	}
	for _, command := range []identity.CommandID{"invalid!", "AA"} {
		invalid := binding
		invalid.CommandID = command
		status, err := c.recover(context.Background(), func(context.Context, string, ...any) (int16, error) {
			t.Fatal("invalid command queried")
			return 0, nil
		}, invalid, plan)
		if status != identitypostgres.RecoveryUnknown || err == nil {
			t.Fatal("invalid command conclusive")
		}
	}
}

func TestIdentityRecoveryRejectsInvalidAuthority(t *testing.T) {
	c, err := NewIdentityContributor(Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		contributor *IdentityContributor
		ctx         context.Context
	}{
		{"zero recovery work", c, context.Background()},
		{"nil context", c, nil},
		{"nil contributor", nil, context.Background()},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, err := test.contributor.RecoverParticipant(test.ctx, identitypostgres.RecoveryWork{})
			if status != identitypostgres.RecoveryUnknown || err == nil {
				t.Fatal("invalid authority produced conclusive recovery")
			}
		})
	}
}

func TestIdentityRecoveryAcceptsFrozenProfileAuditAlias(t *testing.T) {
	c, err := NewIdentityContributor(Config{})
	if err != nil {
		t.Fatal(err)
	}
	binding := identity.CommandBinding{CommandID: "AAAAAAAAAAAAAAAAAAAABA", TenantID: "tenant-a", OperationID: "identity.profile.update"}
	plan := identitypostgres.AuditPlan{EventType: "identity.user.update", SchemaVersion: 1, BindingDigest: [32]byte{1}, RetentionPolicyVersion: 1, RequiredRecordCount: 1, Required: true}
	// RecoveryWork supplies the coordinator's already validated frozen mapping.
	// This seam observes the participant result, not a reconstructed command.
	status, err := c.recover(context.Background(), func(context.Context, string, ...any) (int16, error) { return 0, nil }, binding, plan)
	if err != nil || status != identitypostgres.RecoveryNotCommitted {
		t.Fatal("frozen profile audit alias rejected", status, err)
	}
}
