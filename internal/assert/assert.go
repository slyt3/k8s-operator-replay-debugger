package assert

import (
	"fmt"
	"runtime"
)

const (
	maxStackDepth = 10
)

// Assert checks condition and returns error with context if false.
// Follows Rule 5: side-effect free boolean test with error recovery.
func Assert(condition bool, message string) error {
	if !condition {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			return fmt.Errorf("assertion failed: %s (unknown location)", message)
		}
		return fmt.Errorf("assertion failed at %s:%d: %s", file, line, message)
	}
	return nil
}

// AssertNotNil checks pointer is not nil.
func AssertNotNil(ptr interface{}, name string) error {
	if ptr == nil {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			return fmt.Errorf("nil pointer assertion failed: %s", name)
		}
		return fmt.Errorf("nil pointer at %s:%d: %s", file, line, name)
	}
	return nil
}

// AssertInRange checks value is within bounds.
func AssertInRange(value, min, max int, name string) error {
	if value < min || value > max {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			return fmt.Errorf("range assertion failed: %s value %d not in [%d, %d]",
				name, value, min, max)
		}
		return fmt.Errorf("range error at %s:%d: %s value %d not in [%d, %d]",
			file, line, name, value, min, max)
	}
	return nil
}

// AssertStringNotEmpty checks string is not empty.
func AssertStringNotEmpty(s, name string) error {
	if len(s) == 0 {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			return fmt.Errorf("empty string assertion failed: %s", name)
		}
		return fmt.Errorf("empty string at %s:%d: %s", file, line, name)
	}
	return nil
}
