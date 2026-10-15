package postgres

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/faustbrian/go-audit"
	identity "github.com/faustbrian/go-identity"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
)

// IdentityContributor supplies the identity coordinator's audit phase without
// owning a pool, transaction, Work, registry or goroutine. Configuration is
// immutable after construction, so one instance may serve independent Work
// values concurrently. Each Work itself remains serial-use and generation-bound.
// Existing standalone Store and TxWriter contracts remain separate and unchanged.
type IdentityContributor struct {
	preparer Store
	maximum  int
}

// NewIdentityContributor validates and copies record/batch limits without I/O
// or transaction ownership. Zero Config selects the existing audit defaults.
// Apply all Migrations, including the command-binding migration, before use;
// missing schema or incompatible storage fails closed at persistence.
func NewIdentityContributor(config Config) (*IdentityContributor, error) {
	limits, maximum, bytes, err := validateConfig(config)
	if err != nil {
		return nil, err
	}
	return &IdentityContributor{preparer: Store{limits: limits, maxBatchBytes: bytes}, maximum: maximum}, nil
}

// Descriptor is the exact stable open contract admitted at composition time.
// It is pure and independent of configuration or database state.
func (*IdentityContributor) Descriptor() identitypostgres.ContributorDescriptor {
	return identitypostgres.ContributorDescriptor{Owner: "audit/postgres", ContractID: "identity.security_audit", Version: 1}
}

// PersistAudits writes every complete staged event and its immutable command/
// index binding through this same Work. Exact replay is idempotent; changed
// canonical bytes, event-ID/index reuse or binding/policy mismatch fails closed.
// Event IDs are retained, never derived. A successful return is not a commit
// receipt: only the Coordinator closes the transaction. On any error it must
// abort/reconcile, never ignore a partial prefix. No sink, network, savepoint,
// independent transaction or caller callback is used, and Work is not retained.
// Errors are redacted. Nil receiver/context, cancelled context and invalid,
// incomplete, expired or closed Work fail without staging persistence.
func (c *IdentityContributor) PersistAudits(ctx context.Context, w identitypostgres.Work) error {
	if c == nil || c.maximum == 0 || ctx == nil {
		return audit.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return audit.NewAppendError(audit.AppendRejected, err)
	}
	binding, err := w.Scope()
	if err != nil {
		return err
	}
	entries, err := w.StagedAudits()
	if err != nil {
		return err
	}
	execute := func(ctx context.Context, sql string, args ...any) (int16, error) {
		return identityAuditStatus(w.Query(ctx, sql, args...))
	}
	return c.persist(ctx, execute, binding, entries)
}

const identityAppendSQL = `SELECT audit.append_identity_record($1::text,$2::bytea,$3::integer,$4::numeric,$5::bytea,$6::bytea)`
const identityRecoverySQL = `SELECT audit.identity_record_status($1::text,$2::bytea,$3::integer,$4::numeric,$5::bytea)`

// RecoverParticipant inspects retained command bindings through the sole
// coordinator's frozen, invocation-bound primary evidence capability. Absent
// bindings prove no audit effect; one exact binding with retained record identity
// proves only that participant's effect, never a command receipt. Conflicting,
// additional or incomplete bindings remain unknown. Legal payload pruning does
// not erase evidence. No writes, callbacks, network, private transaction or
// retained handles are used. Errors are redacted; invalid or cancelled authority
// returns RecoveryUnknown. Independent invocations may run concurrently.
func (c *IdentityContributor) RecoverParticipant(ctx context.Context, w identitypostgres.RecoveryWork) (identitypostgres.RecoveryStatus, error) {
	if c == nil || c.maximum == 0 || ctx == nil {
		return identitypostgres.RecoveryUnknown, audit.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return identitypostgres.RecoveryUnknown, audit.NewAppendError(audit.AppendRejected, err)
	}
	binding, err := w.Scope()
	if err != nil {
		return identitypostgres.RecoveryUnknown, err
	}
	plans, err := w.AuditPlans()
	if err != nil {
		return identitypostgres.RecoveryUnknown, err
	}
	return c.recoverPlans(ctx, func(ctx context.Context, sql string, args ...any) (int16, error) {
		return identityAuditStatus(w.Query(ctx, sql, args...))
	}, binding, plans)
}

// The retained routine deliberately treats extra bindings as unknown. Native
// multi-slot recovery may terminalize only when every slot proves absence;
// a partial prefix, pruned payload or any conflicting effect never suffices.
func (c *IdentityContributor) recoverPlans(ctx context.Context, execute identityAuditExecute, binding identity.CommandBinding, plans []identitypostgres.AuditPlan) (identitypostgres.RecoveryStatus, error) {
	if len(plans) == 0 || len(plans) > c.maximum || len(plans) > 64 {
		return identitypostgres.RecoveryUnknown, audit.ErrInvalidArgument
	}
	for i, p := range plans {
		if !validIdentityRecoveryPlan(p) || i > 0 && plans[i-1].EventIndex >= p.EventIndex {
			return identitypostgres.RecoveryUnknown, audit.ErrInvalidArgument
		}
	}
	var first identitypostgres.RecoveryStatus
	unanimous := true
	for i, p := range plans {
		status, err := c.recover(ctx, execute, binding, p)
		if err != nil {
			return identitypostgres.RecoveryUnknown, err
		}
		if i == 0 {
			first = status
		} else if status != first {
			unanimous = false
		}
	}
	if !unanimous {
		return identitypostgres.RecoveryUnknown, nil
	}
	return first, nil
}

func validIdentityRecoveryPlan(plan identitypostgres.AuditPlan) bool {
	return plan.Required && plan.RequiredRecordCount == 1 && plan.RetentionPolicyVersion != 0 && plan.BindingDigest != ([32]byte{}) && plan.SchemaVersion == 1 && plan.EventType != ""
}

// Both owned routines return exactly one smallint, with no payload disclosure.
func identityAuditStatus(rows *identitypostgres.WorkRows, err error) (int16, error) {
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var status int16
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, audit.ErrInvalidArgument
	}
	if err := rows.Scan(&status); err != nil {
		return 0, err
	}
	if rows.Next() {
		return 0, audit.ErrInvalidArgument
	}
	return status, rows.Err()
}

func (c *IdentityContributor) recover(ctx context.Context, execute identityAuditExecute, binding identity.CommandBinding, plan identitypostgres.AuditPlan) (identitypostgres.RecoveryStatus, error) {
	// RecoveryWork exposes only the coordinator's validated frozen audit plan.
	// That owner maps operation aliases (for example profile.update) to their
	// canonical audit action. Requiring literal equality here invents a second,
	// incompatible mapping and prevents valid commands from reconciling.
	if !validIdentityRecoveryPlan(plan) {
		return identitypostgres.RecoveryUnknown, audit.ErrInvalidArgument
	}
	id, err := base64.RawURLEncoding.DecodeString(string(binding.CommandID))
	if err != nil || len(id) != 16 {
		return identitypostgres.RecoveryUnknown, audit.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return identitypostgres.RecoveryUnknown, audit.NewAppendError(audit.AppendRejected, err)
	}
	status, err := execute(ctx, identityRecoverySQL, string(binding.TenantID), id, int32(plan.EventIndex), strconv.FormatUint(plan.RetentionPolicyVersion, 10), plan.BindingDigest[:])
	if err != nil {
		return identitypostgres.RecoveryUnknown, audit.NewAppendError(audit.AppendRejected, &databaseError{operation: "identity recovery", cause: err})
	}
	switch status {
	case 0:
		return identitypostgres.RecoveryNotCommitted, nil
	case 1:
		return identitypostgres.RecoveryCommitted, nil
	case 2:
		return identitypostgres.RecoveryUnknown, nil
	default:
		return identitypostgres.RecoveryUnknown, audit.ErrInvalidArgument
	}
}

// This private SQL seam permits real-database tests of the owned persistence
// algorithm without manufacturing public Work authority. Production supplies
// only the Work-backed extended-protocol executor above.
type identityAuditExecute func(context.Context, string, ...any) (int16, error)

func (c *IdentityContributor) persist(ctx context.Context, execute identityAuditExecute, binding identity.CommandBinding, entries []identitypostgres.StagedAudit) error {
	if len(entries) == 0 || len(entries) > c.maximum || len(entries) > 64 {
		return audit.ErrBatchTooLarge
	}
	records := make([]audit.Record, len(entries))
	for i, e := range entries {
		digest, err := identitypostgres.AuditBindingDigest(binding, e.Record)
		if e.Feature != (identity.FeatureOperation{}) {
			digest, err = identitypostgres.FeatureAuditBindingDigest(binding, e.Feature, e.SubjectType, e.Record)
			if e.Plan.RetentionClass != e.Record.Attributes()["retention.class"] {
				return audit.ErrInvalidArgument
			}
		} else if e.SubjectType != "" || e.Plan.RetentionClass != "" {
			return audit.ErrInvalidArgument
		}
		if err != nil || digest != e.Plan.BindingDigest || e.Record.Action() != e.Plan.EventType || !e.Plan.Required || e.Plan.RequiredRecordCount != 1 || e.Plan.RetentionPolicyVersion == 0 {
			return audit.ErrInvalidArgument
		}
		records[i] = e.Record
	}
	prepared, err := c.preparer.prepare(records)
	if err != nil {
		return err
	}
	id, err := base64.RawURLEncoding.DecodeString(string(binding.CommandID))
	if err != nil || len(id) != 16 {
		return audit.ErrInvalidArgument
	}
	for i, p := range prepared {
		e := entries[i]
		if err := ctx.Err(); err != nil {
			return audit.NewAppendError(audit.AppendRejected, err)
		}
		status, err := execute(ctx, identityAppendSQL, string(binding.TenantID), id, int32(e.Plan.EventIndex), strconv.FormatUint(e.Plan.RetentionPolicyVersion, 10), e.Plan.BindingDigest[:], p.canonical)
		if err != nil {
			return audit.NewAppendError(audit.AppendRejected, &databaseError{operation: "identity staging", cause: err})
		}
		if status != 1 && status != 2 {
			return audit.NewAppendError(audit.AppendRejected, audit.ErrDuplicateConflict)
		}
	}
	return nil
}

var _ identitypostgres.AuditContributor = (*IdentityContributor)(nil)
var _ identitypostgres.RecoveryContributor = (*IdentityContributor)(nil)
