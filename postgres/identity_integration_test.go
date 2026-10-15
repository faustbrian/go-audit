//go:build integration

package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faustbrian/go-audit"
	"github.com/faustbrian/go-authorization/v3"
	identity "github.com/faustbrian/go-identity"
	identitypostgres "github.com/faustbrian/go-identity/postgres"
	"github.com/faustbrian/go-postgres/postgrestest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func identityAuditFixture(t *testing.T, binding identity.CommandBinding) (audit.Record, identitypostgres.StagedAudit) {
	t.Helper()
	b, err := audit.NewBuilder(audit.BuilderConfig{IDGenerator: func() (string, error) { id, e := identity.NewSubjectID(); return string(id), e }})
	if err != nil {
		t.Fatal(err)
	}
	record, err := b.Build(audit.RecordInput{Action: "identity.user.create", OccurredAt: time.Now().UTC(), Outcome: audit.OutcomeSucceeded, ReasonCode: "completed", Actor: audit.ActorInput{Kind: audit.ActorHuman, ID: string(binding.ActorID)}, Subject: audit.SubjectInput{Type: "user", ID: string(binding.UserID)}, Context: audit.ContextInput{TenantID: string(binding.TenantID), CorrelationID: binding.CorrelationID}, Changes: audit.ChangeSetInput{NoChange: true}, Policy: audit.PolicyMetadata{PolicyID: binding.PolicyID, Version: "1"}, Attributes: map[string]string{"command.id": string(binding.CommandID), "identity.profile": "identity.security.v1", "retention.class": "standard", "aggregate.type": "user", "aggregate.id": string(binding.UserID), "aggregate.before_version": "0", "aggregate.after_version": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"command.id", "identity.profile", "retention.class", "aggregate.type", "aggregate.id", "aggregate.before_version", "aggregate.after_version"}
	redactor, err := audit.NewRedactor(audit.RedactionRules{AllowedAttributes: keys})
	if err != nil {
		t.Fatal(err)
	}
	record, err = redactor.Redact(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := identitypostgres.AuditBindingDigest(binding, record)
	if err != nil {
		t.Fatal(err)
	}
	return record, identitypostgres.StagedAudit{Record: record, Plan: identitypostgres.AuditPlan{EventType: record.Action(), SchemaVersion: 1, EventIndex: 0, BindingDigest: digest, RetentionPolicyVersion: ^uint64(0), RequiredRecordCount: 1, MaxRecordBytes: 16384, MaxAggregateBytes: 16384, Required: true}}
}

func TestIdentityAuditSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dsn := os.Getenv("IDENTITY_AUDIT_TEST_DSN")
	if dsn == "" {
		// Match the native suite's digest-pinned fixture rather than requiring
		// an out-of-band database in hosted integration runs. The explicit DSN
		// remains limited to the task-owned local runner below.
		version := os.Getenv("POSTGRES_VERSION")
		if version == "" {
			version = "18"
		}
		matrix, err := os.ReadFile("testdata/postgres-images.tsv")
		if err != nil {
			t.Fatal("PostgreSQL image matrix unavailable")
		}
		var image string
		for _, line := range strings.Split(string(matrix), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == version && strings.Contains(fields[1], "@sha256:") {
				image = fields[1]
			}
		}
		if image == "" {
			t.Fatal("selected PostgreSQL image is not pinned")
		}
		fixture, err := postgrestest.Start(ctx, postgrestest.Config{Image: image, Database: "identity_test", CleanupTimeout: 2 * time.Minute})
		if err != nil {
			t.Fatal("disposable PostgreSQL fixture unavailable")
		}
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 2*time.Minute)
			defer stop()
			if err := fixture.Close(cleanup); err != nil {
				t.Error("PostgreSQL fixture cleanup failed")
			}
		})
		dsn = fixture.DSN()
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil || config.Database != "identity_test" || config.Host != "127.0.0.1" && config.Host != "localhost" && config.Host != "::1" {
		t.Fatal("explicit disposable loopback identity_test database required")
	}
	admin, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("disposable cluster unavailable")
	}
	closeConnection := func(connection *pgx.Conn) {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if e := connection.Close(cleanup); e != nil {
			t.Error("test connection cleanup failed")
		}
	}
	defer closeConnection(admin)
	suffix, err := identity.NewSubjectID()
	if err != nil {
		t.Fatal(err)
	}
	name := "audit_identity_" + string(suffix)
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if _, e := admin.Exec(cleanup, "DROP DATABASE "+quoted); e != nil {
			t.Error("test database cleanup failed")
		}
		// The cluster is explicitly task-owned; these fixed NOLOGIN roles were
		// created by this test's fresh-install transaction, never user roles.
		if _, e := admin.Exec(cleanup, "DROP ROLE IF EXISTS audit_writer, audit_reader, audit_retention, identity_audit_writer, identity_audit_reader, identity_audit_retention"); e != nil {
			t.Error("test role cleanup failed")
		}
	}()
	config.Database = name
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer closeConnection(conn)
	files := Migrations()
	names, err := fs.ReadDir(files, ".")
	if err != nil {
		t.Fatal(err)
	}
	migration, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer migration.Rollback(ctx)
	if _, err = migration.Exec(ctx, FreshInstallPreflightSQL()); err != nil {
		t.Fatal(err)
	}
	for _, file := range names {
		contents, e := fs.ReadFile(files, file.Name())
		if e != nil {
			t.Fatal(e)
		}
		up := strings.Split(string(contents), "-- +migrations Down")[0]
		if _, e = migration.Exec(ctx, up); e != nil {
			t.Fatal("migration", file.Name(), e)
		}
		if file.Name() == "000004_durability_hardening.sql" {
			// Seed an old-binary record before migration 5; its payload and
			// existing writer semantics must remain unchanged on upgrade.
			legacyBinding := identity.CommandBinding{CommandID: "AAAAAAAAAAAAAAAAAAAACA", TenantID: "tenant-a", UserID: "legacy-user", ActorID: "actor-a", ActorKind: authorization.SubjectUser, OperationID: "identity.user.create", PolicyID: "policy", PolicyVersion: 1, SchemaID: "fields", SchemaVersion: 1, CorrelationID: "legacy"}
			legacy, _ := identityAuditFixture(t, legacyBinding)
			limits, _, _, e := validateConfig(Config{})
			if e != nil {
				t.Fatal(e)
			}
			prepared, e := (&Store{limits: limits}).prepare([]audit.Record{legacy})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = insert(ctx, migration, prepared[0]); e != nil {
				t.Fatal("legacy writer failed", e)
			}
		}
	}
	if err = migration.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "CREATE TABLE public.identity_domain_effects(id integer PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	binding := identity.CommandBinding{CommandID: "AAAAAAAAAAAAAAAAAAAABA", TenantID: "tenant-a", UserID: "user-a", ActorID: "actor-a", ActorKind: authorization.SubjectUser, OperationID: "identity.user.create", PolicyID: "policy", PolicyVersion: 1, SchemaID: "fields", SchemaVersion: 1, CorrelationID: "correlation"}
	record, entry := identityAuditFixture(t, binding)
	contributor, err := NewIdentityContributor(Config{})
	if err != nil {
		t.Fatal(err)
	}
	persist := func(tx pgx.Tx, entries []identitypostgres.StagedAudit) error {
		return contributor.persist(ctx, func(ctx context.Context, sql string, args ...any) (int16, error) {
			var status int16
			e := tx.QueryRow(ctx, sql, args...).Scan(&status)
			return status, e
		}, binding, entries)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO public.identity_domain_effects VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if err = persist(tx, []identitypostgres.StagedAudit{entry}); err != nil {
		t.Fatal("same-transaction audit missing", err)
	}
	if err = persist(tx, []identitypostgres.StagedAudit{entry}); err != nil {
		t.Fatal("exact replay rejected", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var records, bindings, effects int
	if err = conn.QueryRow(ctx, "SELECT (SELECT count(*) FROM audit.records),(SELECT count(*) FROM audit.identity_command_bindings),(SELECT count(*) FROM public.identity_domain_effects)").Scan(&records, &bindings, &effects); err != nil || records != 2 || bindings != 1 || effects != 1 {
		t.Fatal("committed atomic state missing", err)
	}
	var eventID, policyVersion string
	if err = conn.QueryRow(ctx, "SELECT event_id,retention_policy_version::text FROM audit.identity_command_bindings").Scan(&eventID, &policyVersion); err != nil || eventID != record.ID() || policyVersion != "18446744073709551615" {
		t.Fatal("opaque ID or full-width version changed", err)
	}
	recover := func(tx pgx.Tx, scope identity.CommandBinding, plan identitypostgres.AuditPlan) (identitypostgres.RecoveryStatus, error) {
		return contributor.recover(ctx, func(ctx context.Context, sql string, args ...any) (int16, error) {
			var status int16
			err := tx.QueryRow(ctx, sql, args...).Scan(&status)
			return status, err
		}, scope, plan)
	}
	t.Run("recovery SQL rejects malformed scope without effects", func(t *testing.T) {
		valid := []any{"tenant-a", make([]byte, 16), int32(0), "1", make([]byte, 32)}
		for _, test := range []struct {
			index int
			value any
		}{
			{0, nil}, {0, ""}, {0, strings.Repeat("x", 65)},
			{1, nil}, {1, []byte{1}}, {2, nil}, {2, int32(-1)}, {2, int32(65536)},
			{3, nil}, {3, "0"}, {3, "1.5"}, {3, "18446744073709551616"}, {3, "NaN"},
			{4, nil}, {4, []byte{1}},
		} {
			args := append([]any(nil), valid...)
			args[test.index] = test.value
			var status int16
			err := conn.QueryRow(ctx, identityRecoverySQL, args...).Scan(&status)
			var failure *pgconn.PgError
			if !errors.As(err, &failure) || failure.Code != "22023" {
				t.Fatal("malformed SQL scope not rejected safely")
			}
		}
	})
	t.Run("recovery uses retained exact bindings", func(t *testing.T) {
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		status, e := recover(tx, binding, entry.Plan)
		if e != nil || status != identitypostgres.RecoveryCommitted {
			t.Fatal("exact binding did not prove participant effect", e)
		}
		absent := binding
		absent.CommandID = "AAAAAAAAAAAAAAAAAAAAEA"
		status, e = recover(tx, absent, entry.Plan)
		if e != nil || status != identitypostgres.RecoveryNotCommitted {
			t.Fatal("absence not proven", e)
		}
		wrongTenant := binding
		wrongTenant.TenantID = "tenant-b"
		status, e = recover(tx, wrongTenant, entry.Plan)
		if e != nil || status != identitypostgres.RecoveryNotCommitted {
			t.Fatal("tenant isolation failed", e)
		}
		wrongIndex := entry.Plan
		wrongIndex.EventIndex++
		wrongPolicy := entry.Plan
		wrongPolicy.RetentionPolicyVersion--
		wrongDigest := entry.Plan
		wrongDigest.BindingDigest[0] ^= 1
		for _, plan := range []identitypostgres.AuditPlan{wrongIndex, wrongPolicy, wrongDigest} {
			status, e = recover(tx, binding, plan)
			if e != nil || status != identitypostgres.RecoveryUnknown {
				t.Fatal("conflicting binding became conclusive", e)
			}
		}
		var after int
		if e = tx.QueryRow(ctx, "SELECT count(*) FROM audit.identity_command_bindings").Scan(&after); e != nil || after != bindings {
			t.Fatal("recovery changed bindings", e)
		}
	})
	t.Run("pruned payload retains recovery evidence", func(t *testing.T) {
		scope := binding
		scope.CommandID = "AAAAAAAAAAAAAAAAAAAAGA"
		r, staged := identityAuditFixture(t, scope)
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		e = contributor.persist(ctx, func(ctx context.Context, sql string, args ...any) (int16, error) {
			var status int16
			err := tx.QueryRow(ctx, sql, args...).Scan(&status)
			return status, err
		}, scope, []identitypostgres.StagedAudit{staged})
		if e != nil {
			t.Fatal(e)
		}
		canonical, e := audit.CanonicalJSON(r)
		if e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256(canonical)
		var removed bool
		if e = tx.QueryRow(ctx, "SELECT audit.prune_record($1,$2)", r.ID(), digest[:]).Scan(&removed); e != nil || !removed {
			t.Fatal("prune failed", e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		tx, e = conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		status, e := recover(tx, scope, staged.Plan)
		if e != nil || status != identitypostgres.RecoveryCommitted {
			t.Fatal("pruning erased evidence", e)
		}
		var payloads int
		if e = tx.QueryRow(ctx, "SELECT count(*) FROM audit.records WHERE record_id=$1", r.ID()).Scan(&payloads); e != nil || payloads != 0 {
			t.Fatal("recovery resurrected payload", e)
		}
	})
	t.Run("rollback retains neither mutation nor audit", func(t *testing.T) {
		_, other := identityAuditFixture(t, binding)
		other.Plan.EventIndex = 1
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "INSERT INTO public.identity_domain_effects VALUES(2)"); e != nil {
			t.Fatal(e)
		}
		if e = persist(tx, []identitypostgres.StagedAudit{other}); e != nil {
			t.Fatal(e)
		}
		if e = tx.Rollback(ctx); e != nil {
			t.Fatal(e)
		}
		var count int
		if e = conn.QueryRow(ctx, "SELECT (SELECT count(*) FROM public.identity_domain_effects WHERE id=2)+(SELECT count(*) FROM audit.records WHERE record_id=$1)+(SELECT count(*) FROM audit.identity_command_bindings WHERE event_index=1)", other.Record.ID()).Scan(&count); e != nil || count != 0 {
			t.Fatal("audit or mutation escaped rollback", e)
		}
	})
	t.Run("only explicit writer grants permit the phase", func(t *testing.T) {
		if _, e := conn.Exec(ctx, "CREATE ROLE identity_audit_writer NOLOGIN; CREATE ROLE identity_audit_reader NOLOGIN; CREATE ROLE identity_audit_retention NOLOGIN"); e != nil {
			t.Fatal(e)
		}
		grants, e := PrivilegeSQL(RoleNames{Writer: "identity_audit_writer", Reader: "identity_audit_reader", Retention: "identity_audit_retention"})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = conn.Exec(ctx, grants); e != nil {
			t.Fatal(e)
		}
		_, other := identityAuditFixture(t, binding)
		other.Plan.EventIndex = 20
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "SET LOCAL ROLE identity_audit_writer"); e != nil {
			t.Fatal(e)
		}
		if e = persist(tx, []identitypostgres.StagedAudit{other}); e != nil {
			t.Fatal("explicit writer cannot persist", e)
		}
		status, e := recover(tx, binding, entry.Plan)
		if e != nil || status != identitypostgres.RecoveryUnknown {
			t.Fatal("writer cannot inspect conflicting extra bindings", e)
		}
		if _, e = tx.Exec(ctx, "SELECT * FROM audit.identity_command_bindings"); e == nil {
			t.Fatal("writer can bypass private binding routine")
		}
		if e = tx.Rollback(ctx); e != nil {
			t.Fatal(e)
		}
		tx, e = conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "SET LOCAL ROLE identity_audit_reader"); e != nil {
			t.Fatal(e)
		}
		if _, e = recover(tx, binding, entry.Plan); e == nil {
			t.Fatal("reader can invoke recovery phase")
		}
		if e = tx.Rollback(ctx); e != nil {
			t.Fatal(e)
		}
		tx, e = conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "SET LOCAL ROLE identity_audit_reader"); e != nil {
			t.Fatal(e)
		}
		if e = persist(tx, []identitypostgres.StagedAudit{other}); e == nil {
			t.Fatal("reader can invoke writer phase")
		}
		if e = tx.Rollback(ctx); e != nil {
			t.Fatal(e)
		}
		tx, e = conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, "SET LOCAL ROLE identity_audit_retention"); e != nil {
			t.Fatal(e)
		}
		if _, e = recover(tx, binding, entry.Plan); e == nil {
			t.Fatal("retention role can invoke recovery phase")
		}
	})
	_, changedID := identityAuditFixture(t, binding)
	changedIndex := entry
	changedIndex.Plan.EventIndex++
	changedPolicy := entry
	changedPolicy.Plan.RetentionPolicyVersion--
	for name, changed := range map[string]identitypostgres.StagedAudit{"changed ID at same index": changedID, "same ID at another index": changedIndex, "changed retained policy": changedPolicy} {
		t.Run(name, func(t *testing.T) {
			tx, e := conn.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			if _, e = tx.Exec(ctx, "INSERT INTO public.identity_domain_effects VALUES(3)"); e != nil {
				t.Fatal(e)
			}
			if e = persist(tx, []identitypostgres.StagedAudit{changed}); e == nil {
				t.Fatal("conflicting binding accepted")
			}
			if e = tx.Commit(ctx); e == nil {
				t.Fatal("conflicting audit allowed partial domain commit")
			}
			var count int
			if e = conn.QueryRow(ctx, "SELECT count(*) FROM public.identity_domain_effects WHERE id=3").Scan(&count); e != nil || count != 0 {
				t.Fatal("failed audit retained domain mutation", e)
			}
		})
	}
	t.Run("same ID with changed canonical bytes", func(t *testing.T) {
		canonical, e := audit.CanonicalJSON(record)
		if e != nil {
			t.Fatal(e)
		}
		changed := strings.Replace(string(canonical), `"aggregate.after_version":"1"`, `"aggregate.after_version":"2"`, 1)
		if changed == string(canonical) {
			t.Fatal("canonical change fixture ineffective")
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		var status int16
		if e = tx.QueryRow(ctx, identityAppendSQL, string(binding.TenantID), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4}, int32(0), "18446744073709551615", entry.Plan.BindingDigest[:], []byte(changed)).Scan(&status); e == nil {
			t.Fatal("changed canonical bytes accepted")
		}
	})
	t.Run("concurrent exact binding is singular", func(t *testing.T) {
		pool, e := pgxpool.NewWithConfig(ctx, mustIdentityPoolConfig(t, config))
		if e != nil {
			t.Fatal(e)
		}
		defer pool.Close()
		_, other := identityAuditFixture(t, binding)
		other.Plan.EventIndex = 10
		start := make(chan struct{})
		results := make(chan error, 2)
		for range 2 {
			go func() {
				<-start
				tx, e := pool.Begin(ctx)
				if e == nil {
					defer tx.Rollback(ctx)
					e = persist(tx, []identitypostgres.StagedAudit{other})
					if e == nil {
						e = tx.Commit(ctx)
					}
				}
				results <- e
			}()
		}
		close(start)
		for range 2 {
			if e = <-results; e != nil {
				t.Error("concurrent binding failed", e)
			}
		}
		var count int
		if e = conn.QueryRow(ctx, "SELECT count(*) FROM audit.identity_command_bindings WHERE event_index=10").Scan(&count); e != nil || count != 1 {
			t.Fatal("concurrent binding duplicated", e)
		}
	})
	t.Run("missing retention profile fails closed", func(t *testing.T) {
		fresh, _ := identityAuditFixture(t, binding)
		canonical, e := audit.CanonicalJSON(fresh)
		if e != nil {
			t.Fatal(e)
		}
		changed := strings.Replace(string(canonical), `,"retention.class":"standard"`, "", 1)
		if changed == string(canonical) {
			t.Fatal("missing retention fixture ineffective")
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		var status int16
		if e = tx.QueryRow(ctx, identityAppendSQL, string(binding.TenantID), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4}, int32(50), "1", entry.Plan.BindingDigest[:], []byte(changed)).Scan(&status); e == nil {
			t.Fatal("missing retention.class admitted")
		}
	})
	t.Run("payload pruning never resurrects on replay", func(t *testing.T) {
		canonical, e := audit.CanonicalJSON(record)
		if e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256(canonical)
		var removed bool
		if e = conn.QueryRow(ctx, "SELECT audit.prune_record($1,$2)", record.ID(), digest[:]).Scan(&removed); e != nil || !removed {
			t.Fatal("prune failed", e)
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if e = persist(tx, []identitypostgres.StagedAudit{entry}); e != nil {
			t.Fatal("retained replay failed", e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		var count int
		if e = conn.QueryRow(ctx, "SELECT count(*) FROM audit.records WHERE record_id=$1", record.ID()).Scan(&count); e != nil || count != 0 {
			t.Fatal("replay resurrected pruned event", e)
		}
	})
	t.Run("native feature public coordinator composition", func(t *testing.T) {
		testIdentityFeatureComposition(t, ctx, config, conn)
	})
}

func mustIdentityPoolConfig(t *testing.T, config *pgx.ConnConfig) *pgxpool.Config {
	t.Helper()
	c, e := pgxpool.ParseConfig("")
	if e != nil {
		t.Fatal(e)
	}
	c.ConnConfig = config.Copy()
	c.MaxConns = 2
	return c
}
