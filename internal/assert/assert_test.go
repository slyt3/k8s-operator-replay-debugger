package assert

import "testing"

func TestAssert(t *testing.T) {
	if err := Assert(true, "ok"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := Assert(false, "fail"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAssertNotNil(t *testing.T) {
	if err := AssertNotNil(struct{}{}, "obj"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := AssertNotNil(nil, "nil"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAssertInRange(t *testing.T) {
	if err := AssertInRange(5, 1, 10, "val"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := AssertInRange(0, 1, 10, "val"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAssertStringNotEmpty(t *testing.T) {
	if err := AssertStringNotEmpty("ok", "s"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := AssertStringNotEmpty("", "s"); err == nil {
		t.Fatalf("expected error")
	}
}
