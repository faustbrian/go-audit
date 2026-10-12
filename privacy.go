package audit

import (
	"context"
	"fmt"
)

// Redactor removes or transforms policy-disallowed fields before persistence
// and before any diagnostic observer receives an operation result.
type Redactor interface {
	// Redact returns a policy-safe record without changing identity or introducing
	// attribute/change keys. Recorder validates allowed transformations and contains
	// panics.
	// Direct calls remain responsible for the implementation's errors and cancellation.
	Redact(context.Context, Record) (Record, error)
}

// RedactorFunc adapts a function to Redactor.
type RedactorFunc func(context.Context, Record) (Record, error)

// Redact invokes the adapted redaction policy and marks a nil-error result as
// explicitly processed. It rejects a nil function/context but does not contain
// panics, sanitize errors or validate record identity; Recorder adds those checks.
func (redactor RedactorFunc) Redact(ctx context.Context, record Record) (Record, error) {
	if redactor == nil || ctx == nil {
		return Record{}, invalid("redactor", "must be assigned")
	}
	redacted, err := redactor(ctx, record)
	if err == nil {
		redacted.redactionApplied = true
	}
	return redacted, err
}

// RedactionRules are deny-by-default allowlists for extensible data and
// explicit removals for privacy-sensitive contextual fields.
type RedactionRules struct {
	// DropDescription removes the optional summary when true; false retains it.
	DropDescription bool
	// DropNetworkOrigin removes network context when true.
	DropNetworkOrigin bool
	// DropUserAgent removes user-agent context when true.
	DropUserAgent bool
	// AllowedAttributes lists exact case-sensitive keys to retain; empty removes all
	// attributes.
	AllowedAttributes []string
	// AllowedChanges lists exact case-sensitive keys to retain in Before and After.
	// Empty removes all structured fields, preserving the explicit Redacted state.
	AllowedChanges []string
}

type ruleRedactor struct {
	attributes, changes                               map[string]struct{}
	dropDescription, dropNetworkOrigin, dropUserAgent bool
}

// NewRedactor compiles deterministic redaction rules. Empty allowlists remove
// every attribute or structured change key. Allowlists are copied into sets;
// duplicates collapse. Identity fields and other context are left unchanged,
// and allowed values are not inspected for credentials.
func NewRedactor(rules RedactionRules) (Redactor, error) {
	limits := DefaultLimits()
	attributes, err := allowlist("allowed_attributes", rules.AllowedAttributes, limits.MaxAttributeEntries)
	if err != nil {
		return nil, err
	}
	changes, err := allowlist("allowed_changes", rules.AllowedChanges, limits.MaxChangeEntries)
	if err != nil {
		return nil, err
	}
	return &ruleRedactor{attributes: attributes, changes: changes, dropDescription: rules.DropDescription, dropNetworkOrigin: rules.DropNetworkOrigin, dropUserAgent: rules.DropUserAgent}, nil
}

func (redactor *ruleRedactor) Redact(ctx context.Context, record Record) (Record, error) {
	if redactor == nil || ctx == nil {
		return Record{}, invalid("redactor", "must be assigned")
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	result := record
	if redactor.dropDescription {
		result.description = ""
	}
	if redactor.dropNetworkOrigin {
		result.context.networkOrigin = ""
	}
	if redactor.dropUserAgent {
		result.context.userAgent = ""
	}
	result.attributes = retain(record.attributes, redactor.attributes)
	result.changes.before = retain(record.changes.before, redactor.changes)
	result.changes.after = retain(record.changes.after, redactor.changes)
	if !record.changes.noChange && len(result.changes.before) == 0 && len(result.changes.after) == 0 {
		result.changes.redacted = true
	}
	result.redactionApplied = true
	return result, nil
}

func allowlist(name string, values []string, maximum int) (map[string]struct{}, error) {
	if len(values) > maximum {
		return nil, invalid(name, "has too many entries")
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := boundedRequired(name, value, DefaultLimits().MaxFieldBytes); err != nil {
			return nil, fmt.Errorf("%w: %s contains an invalid value", ErrInvalidArgument, name)
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func retain(values map[string]string, allowed map[string]struct{}) map[string]string {
	result := make(map[string]string)
	for key, value := range values {
		if _, exists := allowed[key]; exists {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
