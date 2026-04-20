package unified

import "testing"

func TestTokenItemsDeriveStructuredUsage(t *testing.T) {
	tokens := TokenItems{
		{Kind: TokenKindInputNew, Count: 3},
		{Kind: TokenKindInputCacheRead, Count: 4},
		{Kind: TokenKindInputCacheWrite, Count: 5},
		{Kind: TokenKindOutput, Count: 9},
		{Kind: TokenKindOutputReasoning, Count: 2},
	}
	input := tokens.InputTokens()
	if got, want := input.New, 3; got != want {
		t.Fatalf("new = %d, want %d", got, want)
	}
	if got, want := input.CacheRead, 4; got != want {
		t.Fatalf("cache_read = %d, want %d", got, want)
	}
	if got, want := input.CacheWrite, 5; got != want {
		t.Fatalf("cache_write = %d, want %d", got, want)
	}
	if got, want := input.Total, 12; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	output := tokens.OutputTokens()
	if got, want := output.Total, 9; got != want {
		t.Fatalf("output total = %d, want %d", got, want)
	}
	if got, want := output.Reasoning, 2; got != want {
		t.Fatalf("output reasoning = %d, want %d", got, want)
	}
}
