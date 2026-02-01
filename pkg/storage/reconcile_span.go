package storage

import (
	"fmt"
	"time"

	"github.com/slyt3/kubestep/internal/assert"
)

// ReconcileSpan represents a single reconciliation span.
type ReconcileSpan struct {
	ID                     string
	SessionID              string
	ActorID                string
	StartTime              time.Time
	EndTime                time.Time
	DurationMs             int64
	Kind                   string
	Namespace              string
	Name                   string
	TriggerUID             string
	TriggerResourceVersion string
	TriggerReason          string
	Error                  string
}

// ValidateReconcileSpan checks span data meets constraints.
func ValidateReconcileSpan(span *ReconcileSpan) error {
	if span == nil {
		return fmt.Errorf("reconcile span is nil")
	}

	err := assert.AssertNotNil(span, "reconcile span")
	if err != nil {
		return err
	}

	if len(span.ID) == 0 {
		return fmt.Errorf("span id is empty")
	}

	err = assert.AssertInRange(len(span.ID), 1, maxSpanIDLength, "span id length")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(span.SessionID, "session_id")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(span.ActorID, "actor_id")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(span.Kind, "kind")
	if err != nil {
		return err
	}

	if len(span.ActorID) > maxActorIDLength {
		err = assert.Assert(false, "actor_id exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.Kind) > maxResourceKindLength {
		err = assert.Assert(false, "kind exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.Namespace) > maxNamespaceLength {
		err = assert.Assert(false, "namespace exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.Name) > maxNameLength {
		err = assert.Assert(false, "name exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.TriggerUID) > maxUIDLength {
		err = assert.Assert(false, "trigger_uid exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.TriggerResourceVersion) > maxResourceVersionLen {
		err = assert.Assert(false, "trigger_resource_version exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.TriggerReason) > maxTriggerReasonLength {
		err = assert.Assert(false, "trigger_reason exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(span.Error) > maxErrorLength {
		err = assert.Assert(false, "error exceeds max length")
		if err != nil {
			return err
		}
	}

	if span.DurationMs < 0 {
		err = assert.Assert(false, "duration_ms must be non-negative")
		if err != nil {
			return err
		}
	}

	return nil
}
