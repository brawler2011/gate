package sandbox

import (
	"testing"
)

func TestOrchestratorCreation(t *testing.T) {
	client := &Client{}
	orchestrator := NewOrchestrator(client)

	if orchestrator == nil {
		t.Error("NewOrchestrator returned nil")
	}

	if orchestrator.client == nil {
		t.Error("Orchestrator client is nil")
	}

	if orchestrator.compiler == nil {
		t.Error("Orchestrator compiler is nil")
	}

	if orchestrator.executor == nil {
		t.Error("Orchestrator executor is nil")
	}
}

func TestTestCaseStructure(t *testing.T) {
	testCase := TestCase{
		Input:  []byte("1 2 3"),
		Answer: []byte("6"),
	}

	if len(testCase.Input) == 0 {
		t.Error("TestCase Input is empty")
	}

	if len(testCase.Answer) == 0 {
		t.Error("TestCase Answer is empty")
	}
}
