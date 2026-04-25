// Package parse turns raw HTTP responses into structured data the
// analysers can consume. One parse pass per page; analysers share the
// parsed form.
package parse

import (
	"strings"

	"github.com/temoto/robotstxt"
)

// AIUserAgents is the list of AI-crawler user-agents that SageScore
// scores against. Order is significant — it matches the scoring table
// in docs/methodology.md §2.
var AIUserAgents = []string{
	"GPTBot",
	"PerplexityBot",
	"ClaudeBot",
	"Google-Extended",
	"Applebot-Extended",
	"Bytespider",
	"Amazonbot",
}

// RobotsSummary captures domain-level robots.txt facts.
type RobotsSummary struct {
	Fetched             bool   // true if /robots.txt returned 200
	Raw                 []byte // raw file bytes (may be large; capped upstream)
	Data                *robotstxt.RobotsData
	Allowed             map[string]bool // per-UA allow decision for the homepage path "/"
	HasLLMSTxt          bool            // set externally after /llms.txt fetch
	HasLLMSFullTxt      bool            // set externally after /llms-full.txt fetch
	HasSageScoreBotRule bool
	HasWildcardDisallow bool
}

// ParseRobots parses a robots.txt body. An empty body or parse error
// returns a summary with Fetched=false and everything defaulting to
// allowed — robots.txt is opt-in restrictive, so absence = allow.
func ParseRobots(body []byte) *RobotsSummary {
	sum := &RobotsSummary{
		Raw:     body,
		Allowed: map[string]bool{},
	}
	for _, ua := range AIUserAgents {
		sum.Allowed[ua] = true
	}
	sum.Allowed["SageScoreBot"] = true

	if len(body) == 0 {
		return sum
	}
	data, err := robotstxt.FromBytes(body)
	if err != nil {
		return sum
	}
	sum.Fetched = true
	sum.Data = data

	// Determine allow/disallow for "/" per user-agent.
	for _, ua := range AIUserAgents {
		grp := data.FindGroup(ua)
		if grp == nil {
			continue
		}
		sum.Allowed[ua] = grp.Test("/")
	}
	if grp := data.FindGroup("SageScoreBot"); grp != nil {
		sum.Allowed["SageScoreBot"] = grp.Test("/")
	}

	// Heuristic flags — we lift these from the raw text because the
	// robotstxt library normalises away the exact form.
	raw := string(body)
	lower := strings.ToLower(raw)
	sum.HasSageScoreBotRule = strings.Contains(lower, "sagescorebot")

	// Wildcard disallow of root ("User-agent: *" followed by "Disallow: /"
	// on its own, with no path).
	sum.HasWildcardDisallow = hasWildcardDisallow(raw)

	return sum
}

// AllowedForSageScoreBot reports whether SageScoreBot may crawl the path.
// Returns true if no robots.txt or no matching rule.
func (r *RobotsSummary) AllowedForSageScoreBot(path string) bool {
	if r == nil || r.Data == nil {
		return true
	}
	grp := r.Data.FindGroup("SageScoreBot")
	if grp == nil {
		grp = r.Data.FindGroup("*")
	}
	if grp == nil {
		return true
	}
	return grp.Test(path)
}

// hasWildcardDisallow scans the raw text for a "Disallow: /\n" line
// immediately inside a "User-agent: *" block.
func hasWildcardDisallow(raw string) bool {
	lines := strings.Split(raw, "\n")
	inWildcard := false
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		low := strings.ToLower(ln)
		if strings.HasPrefix(low, "user-agent:") {
			val := strings.TrimSpace(ln[len("user-agent:"):])
			inWildcard = val == "*"
			continue
		}
		if !inWildcard {
			continue
		}
		if strings.HasPrefix(low, "disallow:") {
			val := strings.TrimSpace(ln[len("disallow:"):])
			if val == "/" {
				return true
			}
		}
	}
	return false
}
