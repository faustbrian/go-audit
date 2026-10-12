package audit_test

import (
	"context"
	"fmt"
	"time"

	"github.com/faustbrian/go-audit/v2"
	"github.com/faustbrian/go-audit/v2/memory"
)

func ExampleBuilder() {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	builder, err := audit.NewBuilder(audit.BuilderConfig{
		Clock:       func() time.Time { return now },
		IDGenerator: func() (string, error) { return "record-1", nil },
	})
	if err != nil {
		panic(err)
	}
	record, err := builder.Build(audit.RecordInput{
		OccurredAt: now,
		Action:     "invoice.approved",
		Outcome:    audit.OutcomeSucceeded,
		Actor:      audit.ActorInput{Kind: audit.ActorHuman, ID: "user-1"},
		Subject:    audit.SubjectInput{Type: "invoice", ID: "invoice-1"},
		Changes:    audit.ChangeSetInput{NoChange: true},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(record.ID(), record.Action(), err)
	// Output: record-1 invoice.approved <nil>
}

func ExampleRecorder() {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	builder, err := audit.NewBuilder(audit.BuilderConfig{
		Clock:       func() time.Time { return now },
		IDGenerator: func() (string, error) { return "record-1", nil },
	})
	if err != nil {
		panic(err)
	}
	record, err := builder.Build(audit.RecordInput{
		OccurredAt: now, Action: "invoice.viewed", Outcome: audit.OutcomeSucceeded,
		Actor:   audit.ActorInput{Kind: audit.ActorService, ID: "billing-api"},
		Subject: audit.SubjectInput{Type: "invoice", ID: "invoice-1"},
		Changes: audit.ChangeSetInput{NoChange: true},
	})
	if err != nil {
		panic(err)
	}
	sink, err := memory.New(memory.Config{MaxRecords: 10, MaxBytes: 1 << 20, MaxBatchRecords: 10})
	if err != nil {
		panic(err)
	}
	redactor, err := audit.NewRedactor(audit.RedactionRules{})
	if err != nil {
		panic(err)
	}
	recorder, err := audit.NewRecorder(audit.RecorderConfig{Sink: sink, Redactor: redactor, Mode: audit.DeliveryFailClosed})
	if err != nil {
		panic(err)
	}
	result, err := recorder.Submit(context.Background(), record)
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Disposition == audit.DeliveryPersisted, err)
	// Output: true <nil>
}

func ExampleChain() {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	builder, err := audit.NewBuilder(audit.BuilderConfig{
		Clock:       func() time.Time { return now },
		IDGenerator: func() (string, error) { return "record-1", nil },
	})
	if err != nil {
		panic(err)
	}
	record, err := builder.Build(audit.RecordInput{
		OccurredAt: now, Action: "invoice.approved", Outcome: audit.OutcomeSucceeded,
		Actor:   audit.ActorInput{Kind: audit.ActorService, ID: "billing"},
		Subject: audit.SubjectInput{Type: "invoice", ID: "invoice-1"},
		Changes: audit.ChangeSetInput{NoChange: true},
	})
	if err != nil {
		panic(err)
	}
	chain, err := audit.NewChain(audit.ChainConfig{Algorithm: audit.IntegritySHA256})
	if err != nil {
		panic(err)
	}
	sealed, err := chain.Seal(context.Background(), record, audit.ChainLink{Partition: "tenant-1", Sequence: 1})
	if err != nil {
		panic(err)
	}
	verifyErr := chain.Verify(context.Background(), []audit.Record{sealed})
	if verifyErr != nil {
		panic(verifyErr)
	}
	fmt.Println(sealed.Integrity().Enabled(), len(sealed.Integrity().Digest()), err, verifyErr)
	// Output: true 32 <nil> <nil>
}

func ExampleExporter() {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	builder, err := audit.NewBuilder(audit.BuilderConfig{
		Clock:       func() time.Time { return now },
		IDGenerator: func() (string, error) { return "record-1", nil },
	})
	if err != nil {
		panic(err)
	}
	record, err := builder.Build(audit.RecordInput{
		OccurredAt: now, Action: "invoice.viewed", Outcome: audit.OutcomeSucceeded,
		Actor:   audit.ActorInput{Kind: audit.ActorHuman, ID: "user-1"},
		Subject: audit.SubjectInput{Type: "invoice", ID: "invoice-1"},
		Context: audit.ContextInput{TenantID: "tenant-1"},
		Changes: audit.ChangeSetInput{NoChange: true},
	})
	if err != nil {
		panic(err)
	}
	store, err := memory.New(memory.Config{MaxRecords: 1, MaxBytes: 1 << 20, MaxBatchRecords: 1})
	if err != nil {
		panic(err)
	}
	redactor, err := audit.NewRedactor(audit.RedactionRules{})
	if err != nil {
		panic(err)
	}
	record, err = redactor.Redact(context.Background(), record)
	if err != nil {
		panic(err)
	}
	_, err = store.Append(context.Background(), record)
	if err != nil {
		panic(err)
	}
	tenant, err := audit.Tenant("tenant-1")
	if err != nil {
		panic(err)
	}
	query, err := audit.NewQuery(audit.QueryInput{Tenant: tenant, Limit: 1})
	if err != nil {
		panic(err)
	}
	err = store.Export(context.Background(), query, func(record audit.Record) error {
		fmt.Println(record.ID(), record.Action())
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(err)
	// Output:
	// record-1 invoice.viewed
	// <nil>
}
