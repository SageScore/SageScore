package analyse

import (
	"fmt"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// AICrawlers scores the AI-crawler access dimension. Domain-level.
// Point allocations from docs/methodology.md §2.
func AICrawlers(r *parse.RobotsSummary) Sub {
	if r == nil {
		return Sub{Score: 0}
	}
	findings := []Finding{}

	type uaWeight struct {
		UA     string
		Points int
		// When a UA is blocked, emit this code.
		BlockCode string
	}

	uas := []uaWeight{
		{"GPTBot", 14, "AI_CRAWLER_BLOCKED_GPTBOT"},
		{"PerplexityBot", 14, "AI_CRAWLER_BLOCKED_PERPLEXITY"},
		{"ClaudeBot", 14, "AI_CRAWLER_BLOCKED_CLAUDE"},
		{"Google-Extended", 14, "AI_CRAWLER_BLOCKED_GOOGLE_EXTENDED"},
		{"Applebot-Extended", 10, "AI_CRAWLER_BLOCKED_APPLEBOT_EXTENDED"},
		{"Bytespider", 4, "AI_CRAWLER_BLOCKED_BYTESPIDER"},
		{"Amazonbot", 0, "AI_CRAWLER_BLOCKED_AMAZONBOT"},
	}

	score := 0
	for _, u := range uas {
		allowed, present := r.Allowed[u.UA]
		if !present {
			allowed = true
		}
		if allowed {
			score += u.Points
		} else if u.BlockCode != "" {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				Code:     u.BlockCode,
				Message:  fmt.Sprintf("%s is disallowed by robots.txt.", u.UA),
			})
		}
	}

	if r.HasLLMSTxt {
		score += 10
	} else {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "LLMS_TXT_MISSING",
			Message:  "/llms.txt is not present. This is an emerging signal; adoption remains low but non-zero.",
		})
	}
	if r.HasLLMSFullTxt {
		score += 5
	}
	if r.HasSageScoreBotRule {
		score += 2
	}

	if r.HasWildcardDisallow {
		score -= 25
		findings = append(findings, Finding{
			Severity: SeverityCritical,
			Code:     "WILDCARD_DISALLOW",
			Message:  "robots.txt disallows all crawlers at the root path. This blocks both AI and traditional search engines.",
		})
	}

	return Sub{Score: clamp(score, 0, 100), Findings: findings}
}
