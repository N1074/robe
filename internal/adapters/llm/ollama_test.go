package llm

import "testing"

func TestStripThinkingRemovesThinkBlock(t *testing.T) {
	got := stripThinking("<think>private reasoning</think>\nFinal answer.")
	if got != "Final answer." {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestStripThinkingRemovesMultipleThinkBlocks(t *testing.T) {
	got := stripThinking("<think>one</think>\nFinal <think>two</think>answer.")
	if got != "Final answer." {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestStripThinkingDropsUnclosedThinkBlock(t *testing.T) {
	got := stripThinking("Final answer.\n<think>unfinished")
	if got != "Final answer." {
		t.Fatalf("unexpected content: %q", got)
	}
}
