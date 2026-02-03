package main

import "testing"

func TestBuildRootCommand(t *testing.T) {
	cmd := buildRootCommand()
	if cmd == nil {
		t.Fatalf("expected command")
	}
	if cmd.Use != "kubestep" {
		t.Fatalf("expected use kubestep, got %s", cmd.Use)
	}

	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}

	expected := []string{"record", "replay", "analyze", "sessions", "verify"}
	for i := 0; i < len(expected); i++ {
		if !names[expected[i]] {
			t.Fatalf("expected subcommand %s", expected[i])
		}
	}
}
