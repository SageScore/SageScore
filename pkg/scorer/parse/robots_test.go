package parse

import "testing"

func TestParseRobots_WildcardDisallow(t *testing.T) {
	body := []byte("User-agent: *\nDisallow: /\n")
	sum := ParseRobots(body)
	if !sum.HasWildcardDisallow {
		t.Fatal("wildcard disallow not detected")
	}
}

func TestParseRobots_AIUAsDisallowed(t *testing.T) {
	body := []byte("User-agent: GPTBot\nDisallow: /\n\nUser-agent: ClaudeBot\nDisallow: /private\n")
	sum := ParseRobots(body)
	if sum.Allowed["GPTBot"] {
		t.Fatal("GPTBot should be disallowed at /")
	}
	if !sum.Allowed["ClaudeBot"] {
		t.Fatal("ClaudeBot only disallowed on /private — should be allowed at /")
	}
}

func TestParseRobots_EmptyReturnsAllAllowed(t *testing.T) {
	sum := ParseRobots(nil)
	for _, ua := range AIUserAgents {
		if !sum.Allowed[ua] {
			t.Fatalf("%s should be allowed when robots.txt absent", ua)
		}
	}
}

func TestParseRobots_SageScoreBotDisallowAtRoot(t *testing.T) {
	body := []byte("User-agent: SageScoreBot\nDisallow: /\n")
	sum := ParseRobots(body)
	if sum.AllowedForSageScoreBot("/") {
		t.Fatal("SageScoreBot should be disallowed at root")
	}
	if !sum.HasSageScoreBotRule {
		t.Fatal("SageScoreBot rule detection failed")
	}
}
