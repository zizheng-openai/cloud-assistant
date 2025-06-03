package ai

import (
	"context"
	"testing"

	"github.com/jlewi/cloud-assistant/protos/gen/cassie"
)

func TestShellRequiredFlag_Pass(t *testing.T) {
	asserter := shellRequiredFlag{}
	assertion := &cassie.Assertion{
		Name: "test-pass",
		Type: cassie.Assertion_TYPE_SHELL_REQUIRED_FLAG,
		Payload: &cassie.Assertion_ShellRequiredFlag_{
			ShellRequiredFlag: &cassie.Assertion_ShellRequiredFlag{
				Command: "kubectl",
				Flags:   []string{"--context", "-n"},
			},
		},
	}
	blocks := map[string]*cassie.Block{
		"1": {
			Kind:     cassie.BlockKind_CODE,
			Contents: "kubectl get pods --context test -n default",
		},
	}
	err := asserter.Assert(context.Background(), assertion, blocks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assertion.Result != cassie.Assertion_RESULT_TRUE {
		t.Errorf("expected PASSED, got %v", assertion.Result)
	}
}

func TestShellRequiredFlag_Fail(t *testing.T) {
	asserter := shellRequiredFlag{}
	assertion := &cassie.Assertion{
		Name: "test-fail",
		Type: cassie.Assertion_TYPE_SHELL_REQUIRED_FLAG,
		Payload: &cassie.Assertion_ShellRequiredFlag_{
			ShellRequiredFlag: &cassie.Assertion_ShellRequiredFlag{
				Command: "kubectl",
				Flags:   []string{"--context", "-n"},
			},
		},
	}
	blocks := map[string]*cassie.Block{
		"1": {
			Kind:     cassie.BlockKind_CODE,
			Contents: "kubectl get pods --context test",
		},
	}
	err := asserter.Assert(context.Background(), assertion, blocks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assertion.Result != cassie.Assertion_RESULT_FALSE {
		t.Errorf("expected FAILED, got %v", assertion.Result)
	}
}
