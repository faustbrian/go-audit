package postgres

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/faustbrian/go-audit"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
)

// PersistSCIMBulkSkipAudits appends only the coordinator-built protected cutoff
// events exposed to this registered contributor during the native failure Work.
// Each event uses its skipped child's command binding at index zero; it does not
// consume or change the triggering command's frozen audit plan. Exact replay is
// idempotent. Any error requires the coordinator to abort the complete Work.
// It owns no transaction, retains no Work and makes no independent callbacks.
func (c *IdentityContributor) PersistSCIMBulkSkipAudits(ctx context.Context, w identitypostgres.Work) error {
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
	entries, err := w.StagedSCIMBulkSkipAudits(c)
	if err != nil {
		return err
	}
	// SCIM admits at most 1,000 children, one of which triggered the cutoff.
	if len(entries) == 0 || len(entries) > 999 {
		return audit.ErrBatchTooLarge
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return audit.NewAppendError(audit.AppendRejected, err)
		}
		id, err := base64.RawURLEncoding.DecodeString(string(entry.Child.CommandID))
		if err != nil || len(id) != 16 || entry.RetentionPolicyVersion == 0 || entry.BindingDigest == ([32]byte{}) {
			return audit.ErrInvalidArgument
		}
		// Use the existing bounded canonical preparation and append routine one
		// record at a time rather than inventing a second bulk storage protocol.
		prepared, err := c.preparer.prepare([]audit.Record{entry.Record})
		if err != nil {
			return err
		}
		status, err := identityAuditStatus(w.Query(ctx, identityAppendSQL, string(binding.TenantID), id, int32(0),
			strconv.FormatUint(entry.RetentionPolicyVersion, 10), entry.BindingDigest[:], prepared[0].canonical))
		if err != nil {
			return audit.NewAppendError(audit.AppendRejected, &databaseError{operation: "SCIM bulk skip staging", cause: err})
		}
		if status != 1 && status != 2 {
			return audit.NewAppendError(audit.AppendRejected, audit.ErrDuplicateConflict)
		}
	}
	return nil
}

var _ identitypostgres.SCIMBulkSkipAuditContributor = (*IdentityContributor)(nil)
