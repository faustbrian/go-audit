package audit_test

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/faustbrian/go-audit/v2"
)

func TestBuilderOversizedInputDoesNotAllocateForNormalizationOrEncoding(t *testing.T) {
	// The caller-owned payload is outside the measured operation. Rejection
	// must not allocate another input-sized normalization or encoding buffer.
	oversized := strings.Repeat("A_", 1<<20)
	for _, name := range []string{"attributes", "before", "after", "record-capped attributes", "record-capped description"} {
		t.Run(name, func(t *testing.T) {
			limits := audit.DefaultLimits()
			input := securityInput(audit.IntegrityInput{}, nil)
			switch name {
			case "attributes":
				input.Attributes = map[string]string{oversized: "value"}
			case "before":
				input.Changes = audit.ChangeSetInput{Before: map[string]string{oversized: "value"}}
			case "after":
				input.Changes = audit.ChangeSetInput{After: map[string]string{oversized: "value"}}
			case "record-capped attributes":
				limits.MaxAttributeBytes = 4 * len(oversized)
				input.Attributes = map[string]string{oversized: "value"}
			case "record-capped description":
				limits.MaxDescriptionBytes = 4 * len(oversized)
				input.Description = oversized
			}
			builder, err := audit.NewBuilder(audit.BuilderConfig{
				Limits: limits, Clock: func() time.Time { return time.Unix(1, 0) },
				IDGenerator: func() (string, error) { return "record", nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err = builder.Build(input)
			runtime.ReadMemStats(&after)
			if !errors.Is(err, audit.ErrInvalidArgument) {
				t.Fatalf("oversized input error = %v", err)
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 256<<10 {
				t.Fatalf("oversized rejection allocated %d bytes for a %d-byte input", allocated, len(oversized))
			}
		})
	}
}

func TestBuilderByteLimitsPrecedeTextAndPrivacyValidation(t *testing.T) {
	input := securityInput(audit.IntegrityInput{}, nil)
	input.Action = strings.Repeat("a", audit.DefaultLimits().MaxFieldBytes) + string([]byte{0xff})
	_, err := securityBuilder(t).Build(input)
	if !errors.Is(err, audit.ErrInvalidArgument) || !strings.Contains(err.Error(), "exceeds byte limit") {
		t.Fatalf("oversized invalid text error = %v", err)
	}
	input = securityInput(audit.IntegrityInput{}, nil)
	input.Attributes = map[string]string{strings.Repeat("a", audit.DefaultLimits().MaxAttributeBytes) + ".password": "value"}
	_, err = securityBuilder(t).Build(input)
	if !errors.Is(err, audit.ErrInvalidArgument) || errors.Is(err, audit.ErrSensitiveData) {
		t.Fatalf("oversized prohibited key error = %v", err)
	}
}

func TestBuilderMapByteBudgetIncludesEveryKeyAndValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		budget      int
		attributes  map[string]string
		wantInvalid bool
	}{
		{name: "exact key-only ceiling", budget: 16, attributes: map[string]string{strings.Repeat("a", 16): ""}},
		{name: "exact cumulative ceiling", budget: 15, attributes: map[string]string{"a": "xxxx", "b": "xxxx", "c": "xxxx"}},
		{name: "cumulative entries exceed ceiling", budget: 12, attributes: map[string]string{"a": "xxxx", "b": "xxxx", "c": "xxxx"}, wantInvalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits := audit.DefaultLimits()
			limits.MaxAttributeBytes = tc.budget
			builder, err := audit.NewBuilder(audit.BuilderConfig{Limits: limits})
			if err != nil {
				t.Fatal(err)
			}
			input := securityInput(audit.IntegrityInput{}, nil)
			input.Attributes = tc.attributes
			_, err = builder.Build(input)
			if tc.wantInvalid {
				if !errors.Is(err, audit.ErrInvalidArgument) {
					t.Fatalf("over-budget map error = %v, want ErrInvalidArgument", err)
				}
			} else if err != nil {
				t.Fatalf("exact-budget map error = %v", err)
			}
		})
	}
}
