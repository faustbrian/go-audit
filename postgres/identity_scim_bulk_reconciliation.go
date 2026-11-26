package postgres

import (
	"context"

	"github.com/faustbrian/go-audit"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
)

// PersistSCIMBulkReconciliationAudit appends only the kernel-custodied event
// exposed to this registered contributor during a SCIM Bulk reconciliation.
// It uses the existing append writer grant from PrivilegeSQL and leaves the
// original command's audit binding unchanged. Exact event replay is idempotent;
// any error requires the coordinator to abort the complete Work. The contributor
// owns no transaction, retains no Work and starts no independent callbacks.
func (c *IdentityContributor) PersistSCIMBulkReconciliationAudit(ctx context.Context, w identitypostgres.Work) error {
	if c == nil || c.maximum == 0 || ctx == nil {
		return audit.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return audit.NewAppendError(audit.AppendRejected, err)
	}
	record, err := w.StagedSCIMBulkReconciliationAudit(c)
	if err != nil {
		return err
	}
	prepared, err := c.preparer.prepare([]audit.Record{record})
	if err != nil {
		return err
	}
	value := prepared[0]
	record = value.record
	status, err := identityAuditStatus(w.Query(ctx, appendRecordSQL,
		record.ID(), record.OccurredAt(), record.RecordedAt(), record.Context().TenantID(),
		int16(record.Actor().Kind()), record.Actor().ID(), record.Subject().Type(), record.Subject().ID(),
		record.Action(), int16(record.Outcome()), record.Context().CorrelationID(), value.canonical, value.digest))
	if err != nil {
		return audit.NewAppendError(audit.AppendRejected, &databaseError{operation: "SCIM bulk reconciliation staging", cause: err})
	}
	switch status {
	case 1, 2:
		return nil
	case 3:
		return audit.NewAppendError(audit.AppendRejected, audit.ErrDuplicateConflict)
	default:
		return audit.NewAppendError(audit.AppendRejected, &databaseError{operation: "SCIM bulk reconciliation status", cause: audit.ErrInvalidArgument})
	}
}

var _ identitypostgres.SCIMBulkReconciliationAuditContributor = (*IdentityContributor)(nil)
