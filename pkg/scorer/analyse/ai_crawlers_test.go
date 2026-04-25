package analyse

import (
	"testing"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

func TestAICrawlers_AllAllowed(t *testing.T) {
	r := &parse.RobotsSummary{
		Allowed: map[string]bool{
			"GPTBot": true, "PerplexityBot": true, "ClaudeBot": true,
			"Google-Extended": true, "Applebot-Extended": true,
			"Bytespider": true, "Amazonbot": true,
		},
		HasLLMSTxt: true,
	}
	sub := AICrawlers(r)
	if sub.Score < 70 {
		t.Fatalf("expected ≥70 with all allowed + llms.txt, got %d", sub.Score)
	}
}

func TestAICrawlers_WildcardDisallow(t *testing.T) {
	r := &parse.RobotsSummary{
		Allowed: map[string]bool{
			"GPTBot": false, "PerplexityBot": false, "ClaudeBot": false,
			"Google-Extended": false, "Applebot-Extended": false,
			"Bytespider": false, "Amazonbot": false,
		},
		HasWildcardDisallow: true,
	}
	sub := AICrawlers(r)
	if sub.Score != 0 {
		t.Fatalf("expected 0 on wildcard disallow+all blocked, got %d", sub.Score)
	}
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "WILDCARD_DISALLOW" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("WILDCARD_DISALLOW finding not emitted")
	}
}

func TestAICrawlers_ClaudeBlocked(t *testing.T) {
	r := &parse.RobotsSummary{
		Allowed: map[string]bool{
			"GPTBot": true, "PerplexityBot": true, "ClaudeBot": false,
			"Google-Extended": true, "Applebot-Extended": true,
			"Bytespider": true, "Amazonbot": true,
		},
	}
	sub := AICrawlers(r)
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "AI_CRAWLER_BLOCKED_CLAUDE" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("AI_CRAWLER_BLOCKED_CLAUDE not emitted")
	}
}
