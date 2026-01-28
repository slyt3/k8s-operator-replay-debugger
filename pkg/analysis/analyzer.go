package analysis

import (
	"fmt"
	"time"

	"github.com/operator-replay-debugger/internal/assert"
	"github.com/operator-replay-debugger/pkg/storage"
)

const (
	maxAnalysisOperations  = 100000
	loopDetectionWindow    = 100
	slowOperationThreshold = 1000
)

// LoopDetection identifies potential infinite loops in operations.
// Rule 2: All loops bounded with explicit limits.
type LoopDetection struct {
	Operations []storage.Operation
	WindowSize int
}

// Pattern represents a detected pattern in operations.
type Pattern struct {
	StartIndex    int
	EndIndex      int
	RepeatCount   int
	OperationKind string
	Description   string
}

// DetectLoops finds repeating operation patterns.
// Rule 4: Function under 60 lines with clear logic.
func DetectLoops(ops []storage.Operation, windowSize int) ([]Pattern, error) {
	err := assert.AssertInRange(
		len(ops),
		0,
		maxAnalysisOperations,
		"operation count",
	)
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(
		windowSize,
		2,
		loopDetectionWindow,
		"window size",
	)
	if err != nil {
		return nil, err
	}

	patterns := make([]Pattern, 0, 100)
	maxPatterns := 100

	i := 0
	opCount := len(ops)

	for i < opCount-windowSize && len(patterns) < maxPatterns {
		pattern := checkPatternAt(ops, i, windowSize, opCount)
		if pattern != nil {
			patterns = append(patterns, *pattern)
			i = pattern.EndIndex + 1
		} else {
			i = i + 1
		}
	}

	return patterns, nil
}

// checkPatternAt checks for repeating pattern starting at index.
// Rule 2: Bounded iteration with explicit limit.
func checkPatternAt(
	ops []storage.Operation,
	startIdx int,
	windowSize int,
	maxIdx int,
) *Pattern {
	if startIdx+windowSize*2 > maxIdx {
		return nil
	}

	matchCount := 0
	currentIdx := startIdx
	maxMatches := 10

	for matchCount < maxMatches && currentIdx+windowSize*2 <= maxIdx {
		isMatch := compareWindows(ops, currentIdx, currentIdx+windowSize, windowSize)
		if !isMatch {
			break
		}
		matchCount = matchCount + 1
		currentIdx = currentIdx + windowSize
	}

	if matchCount < 2 {
		return nil
	}

	return &Pattern{
		StartIndex:    startIdx,
		EndIndex:      currentIdx - 1,
		RepeatCount:   matchCount,
		OperationKind: ops[startIdx].ResourceKind,
		Description: fmt.Sprintf(
			"Repeated %s operations %d times",
			ops[startIdx].ResourceKind,
			matchCount,
		),
	}
}

// compareWindows checks if two operation windows match.
// Rule 2: Bounded comparison with explicit limit.
func compareWindows(
	ops []storage.Operation,
	idx1 int,
	idx2 int,
	size int,
) bool {
	if idx1+size > len(ops) || idx2+size > len(ops) {
		return false
	}

	matchCount := 0
	i := 0

	for i < size {
		op1 := &ops[idx1+i]
		op2 := &ops[idx2+i]

		if op1.OperationType != op2.OperationType {
			return false
		}
		if op1.ResourceKind != op2.ResourceKind {
			return false
		}
		if op1.Namespace != op2.Namespace {
			return false
		}
		if op1.Name != op2.Name {
			return false
		}

		matchCount = matchCount + 1
		i = i + 1
	}

	return matchCount == size
}

// SlowOperation represents an operation exceeding threshold.
type SlowOperation struct {
	Index      int
	Operation  storage.Operation
	DurationMs int64
}

// FindSlowOperations identifies operations exceeding duration threshold.
// Rule 2: Bounded loop over operations.
func FindSlowOperations(
	ops []storage.Operation,
	thresholdMs int64,
) ([]SlowOperation, error) {
	err := assert.AssertInRange(
		len(ops),
		0,
		maxAnalysisOperations,
		"operation count",
	)
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(
		int(thresholdMs),
		1,
		1000000,
		"threshold milliseconds",
	)
	if err != nil {
		return nil, err
	}

	slowOps := make([]SlowOperation, 0, 100)
	maxSlowOps := 100

	i := 0
	opCount := len(ops)

	for i < opCount && len(slowOps) < maxSlowOps {
		op := &ops[i]
		if op.DurationMs >= thresholdMs {
			slowOps = append(slowOps, SlowOperation{
				Index:      i,
				Operation:  *op,
				DurationMs: op.DurationMs,
			})
		}
		i = i + 1
	}

	return slowOps, nil
}

// ErrorSummary summarizes errors found in operations.
type ErrorSummary struct {
	TotalErrors  int
	ErrorsByType map[string]int
	FirstError   *storage.Operation
	LastError    *storage.Operation
}

// AnalyzeErrors summarizes all errors in operations.
// Rule 2: Bounded loop with clear termination.
func AnalyzeErrors(ops []storage.Operation) (*ErrorSummary, error) {
	err := assert.AssertInRange(
		len(ops),
		0,
		maxAnalysisOperations,
		"operation count",
	)
	if err != nil {
		return nil, err
	}

	summary := &ErrorSummary{
		ErrorsByType: make(map[string]int, 20),
	}

	maxErrorTypes := 20
	i := 0
	opCount := len(ops)

	for i < opCount {
		op := &ops[i]
		if len(op.Error) > 0 {
			summary.TotalErrors = summary.TotalErrors + 1

			if summary.FirstError == nil {
				summary.FirstError = op
			}
			summary.LastError = op

			if len(summary.ErrorsByType) < maxErrorTypes {
				errorType := string(op.OperationType)
				summary.ErrorsByType[errorType] =
					summary.ErrorsByType[errorType] + 1
			}
		}
		i = i + 1
	}

	return summary, nil
}

// ResourceAccessPattern tracks how resources are accessed.
type ResourceAccessPattern struct {
	ResourceKey string
	ReadCount   int
	WriteCount  int
	FirstAccess time.Time
	LastAccess  time.Time
}

// AnalyzeResourceAccess tracks resource access patterns.
// Rule 2: Bounded loop and bounded map size.
func AnalyzeResourceAccess(
	ops []storage.Operation,
) (map[string]*ResourceAccessPattern, error) {
	err := assert.AssertInRange(
		len(ops),
		0,
		maxAnalysisOperations,
		"operation count",
	)
	if err != nil {
		return nil, err
	}

	patterns := make(map[string]*ResourceAccessPattern, 1000)
	maxPatterns := 1000

	i := 0
	opCount := len(ops)

	for i < opCount {
		op := &ops[i]
		key := fmt.Sprintf("%s/%s/%s",
			op.ResourceKind, op.Namespace, op.Name)

		if len(patterns) >= maxPatterns {
			break
		}

		pattern, exists := patterns[key]
		if !exists {
			pattern = &ResourceAccessPattern{
				ResourceKey: key,
				FirstAccess: op.Timestamp,
			}
			patterns[key] = pattern
		}

		if isReadOperation(op.OperationType) {
			pattern.ReadCount = pattern.ReadCount + 1
		} else if isWriteOperation(op.OperationType) {
			pattern.WriteCount = pattern.WriteCount + 1
		}

		pattern.LastAccess = op.Timestamp

		i = i + 1
	}

	return patterns, nil
}

// isReadOperation checks if operation is a read.
func isReadOperation(opType storage.OperationType) bool {
	return opType == storage.OperationGet || opType == storage.OperationList
}

// isWriteOperation checks if operation is a write.
func isWriteOperation(opType storage.OperationType) bool {
	return opType == storage.OperationCreate ||
		opType == storage.OperationUpdate ||
		opType == storage.OperationPatch ||
		opType == storage.OperationDelete
}
