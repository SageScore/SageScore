package analyse

import (
	"math"
	"strings"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// Content analyses structure of one page. 7 weighted sub-checks
// (methodology §3).
func Content(page *parse.ParsedPage) Sub {
	findings := []Finding{}
	total := 0

	// 1. BLUF / answer-first (20 pts).
	bluf := 0
	if len(page.Paragraphs) > 0 {
		first := page.Paragraphs[0]
		words := strings.Fields(first)
		firstSent := ""
		if len(page.Sentences) > 0 {
			firstSent = page.Sentences[0]
		}
		if len(words) <= 100 && len(firstSent) < 200 && looksAnswerShaped(firstSent) {
			bluf = 20
		} else if len(words) <= 100 && len(firstSent) < 200 {
			bluf = 10
		}
	}
	if bluf < 20 {
		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Code:     "CONTENT_BLUF_MISSING",
			Message:  "Opening paragraph does not answer the page's implicit question up-front.",
		})
	}
	total += bluf

	// 2. Chunk-size hygiene (20 pts): sections between same-level
	// headings 150–300 words on average.
	chunks := chunkWordCounts(page)
	chunkScore := 20
	if len(chunks) > 0 {
		mean := meanInt(chunks)
		switch {
		case mean > 300:
			chunkScore = 5
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				Code:     "CONTENT_CHUNKS_TOO_LONG",
				Message:  "Sections average more than 300 words; LLM retrieval drops attention mid-chunk.",
			})
		case mean < 100:
			chunkScore = 5
			findings = append(findings, Finding{
				Severity: SeverityMedium,
				Code:     "CONTENT_CHUNKS_TOO_SHORT",
				Message:  "Sections are very short; content feels fragmented.",
			})
		case mean >= 150 && mean <= 300:
			chunkScore = 20
		default:
			// 100-149 linear ramp
			chunkScore = 10
		}
	}
	total += chunkScore

	// 3. Structural-element ratio (20 pts): lists/tables/code vs. paragraphs.
	se := page.Lists + page.Tables + page.PreBlocks
	units := se + len(page.Paragraphs)
	seScore := 0
	if units > 0 {
		ratio := float64(se) / float64(units)
		switch {
		case ratio >= 0.25 && ratio <= 0.35:
			seScore = 20
		case ratio >= 0.15 && ratio < 0.25, ratio > 0.35 && ratio <= 0.5:
			seScore = 12
		case ratio > 0 && ratio < 0.15:
			seScore = 5
		}
	}
	if seScore < 20 {
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "CONTENT_LOW_STRUCTURAL_ELEMENTS",
			Message:  "Sparse use of lists, tables, or code blocks; these formats are extracted by AI-search at ~43% higher accuracy than prose.",
		})
	}
	total += seScore

	// 4. Paragraph length (10 pts): mean 30–80 words.
	pScore := 0
	if len(page.Paragraphs) > 0 {
		lens := make([]int, 0, len(page.Paragraphs))
		for _, p := range page.Paragraphs {
			lens = append(lens, len(strings.Fields(p)))
		}
		mean := meanInt(lens)
		switch {
		case mean >= 30 && mean <= 80:
			pScore = 10
		case mean > 80 && mean <= 150:
			pScore = 5
			findings = append(findings, Finding{
				Severity: SeverityLow,
				Code:     "CONTENT_PARAGRAPHS_TOO_LONG",
				Message:  "Paragraphs exceed the 30–80 word scanability band.",
			})
		case mean > 150:
			pScore = 0
			findings = append(findings, Finding{
				Severity: SeverityMedium,
				Code:     "CONTENT_PARAGRAPHS_TOO_LONG",
				Message:  "Paragraphs are very long; break them up.",
			})
		}
	}
	total += pScore

	// 5. Heading depth/validity (15 pts).
	hScore, hFindings := headingDepthScore(page)
	total += hScore
	findings = append(findings, hFindings...)

	// 6. Readability (10 pts) via Flesch reading ease.
	fre := fleschReadingEase(page)
	rScore := 0
	switch {
	case fre >= 50:
		rScore = 10
	case fre >= 20:
		rScore = int(10 * (fre - 20) / 30)
	default:
		rScore = 0
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "CONTENT_READING_EASE_LOW",
			Message:  "Reading ease is low; complex syntax suppresses AI extraction.",
		})
	}
	total += rScore

	// 7. Keyword-stuffing penalty (0 to -5).
	top, pct := topTermRatio(page)
	if pct > 0.03 && page.WordN > 50 {
		total -= 5
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "CONTENT_KEYWORD_STUFFING",
			Message:  "Term \"" + top + "\" accounts for more than 3% of body copy; keyword stuffing is a negative signal for LLM citation.",
		})
	}

	return Sub{Score: clamp(total, 0, 100), Findings: findings}
}

func looksAnswerShaped(sentence string) bool {
	// Heuristic: at least 5 words, contains a verb-shaped token.
	words := strings.Fields(sentence)
	if len(words) < 5 {
		return false
	}
	for _, w := range words {
		lw := strings.ToLower(strings.Trim(w, ".,;:!?"))
		if _, ok := commonVerbs[lw]; ok {
			return true
		}
	}
	return false
}

var commonVerbs = map[string]struct{}{
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"has": {}, "have": {}, "had": {},
	"do": {}, "does": {}, "did": {},
	"can": {}, "could": {}, "will": {}, "would": {}, "should": {}, "may": {}, "might": {},
	"provides": {}, "offers": {}, "uses": {}, "used": {}, "produces": {}, "makes": {}, "made": {},
	"creates": {}, "created": {}, "helps": {}, "built": {}, "enables": {},
	"includes": {}, "contains": {}, "means": {}, "allows": {}, "gives": {},
}

func chunkWordCounts(page *parse.ParsedPage) []int {
	if len(page.Headings) == 0 {
		return []int{page.WordN}
	}
	// Simpler proxy: total words / number of sections.
	secs := len(page.Headings)
	if secs == 0 {
		secs = 1
	}
	avg := page.WordN / secs
	out := make([]int, secs)
	for i := range out {
		out[i] = avg
	}
	return out
}

func headingDepthScore(page *parse.ParsedPage) (int, []Finding) {
	findings := []Finding{}
	if len(page.Headings) == 0 {
		return 0, findings
	}
	maxLevel := 0
	levels := map[int]bool{}
	brokenHierarchy := 0
	prev := 0
	for _, h := range page.Headings {
		if h.Level > maxLevel {
			maxLevel = h.Level
		}
		levels[h.Level] = true
		if prev > 0 && h.Level > prev+1 {
			brokenHierarchy++
		}
		prev = h.Level
	}
	score := 15
	if maxLevel < 3 {
		score -= 8
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "CONTENT_HEADINGS_TOO_FLAT",
			Message:  "Heading structure is very shallow; 3–5 levels is optimal for retrieval.",
		})
	} else if maxLevel > 5 {
		score -= 8
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "CONTENT_HEADINGS_TOO_DEEP",
			Message:  "Heading hierarchy exceeds 5 levels; excessive depth dilutes attention.",
		})
	}
	if brokenHierarchy > 0 {
		score -= 5 * brokenHierarchy
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "CONTENT_H_HIERARCHY_BROKEN",
			Message:  "Heading levels skip (e.g. h3 without a preceding h2).",
		})
	}
	if score < 0 {
		score = 0
	}
	return score, findings
}

// fleschReadingEase computes the Flesch reading ease from parsed page
// text. Approximate — counts syllables via vowel groups.
func fleschReadingEase(page *parse.ParsedPage) float64 {
	if page.WordN == 0 || len(page.Sentences) == 0 {
		return 0
	}
	syllables := 0
	for _, w := range page.Words {
		syllables += syllableCount(w)
	}
	asl := float64(page.WordN) / float64(len(page.Sentences))
	asw := float64(syllables) / float64(page.WordN)
	return 206.835 - 1.015*asl - 84.6*asw
}

func syllableCount(word string) int {
	w := strings.ToLower(word)
	w = strings.Trim(w, ".,;:!?\"'()[]")
	if w == "" {
		return 0
	}
	count := 0
	prevVowel := false
	for _, r := range w {
		isV := strings.ContainsRune("aeiouy", r)
		if isV && !prevVowel {
			count++
		}
		prevVowel = isV
	}
	// Silent 'e' at end.
	if strings.HasSuffix(w, "e") && count > 1 {
		count--
	}
	if count == 0 {
		count = 1
	}
	return count
}

var stopwords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "of": {}, "in": {},
	"to": {}, "for": {}, "with": {}, "on": {}, "at": {}, "by": {}, "as": {}, "is": {},
	"are": {}, "was": {}, "were": {}, "be": {}, "this": {}, "that": {}, "it": {},
	"i": {}, "you": {}, "we": {}, "they": {}, "he": {}, "she": {}, "not": {},
	"from": {}, "have": {}, "has": {}, "had": {}, "do": {}, "does": {}, "did": {},
	"will": {}, "would": {}, "can": {}, "could": {}, "if": {}, "then": {}, "so": {},
	"also": {}, "there": {}, "their": {}, "our": {}, "your": {}, "its": {},
}

func topTermRatio(page *parse.ParsedPage) (string, float64) {
	counts := map[string]int{}
	for _, w := range page.Words {
		lw := strings.ToLower(strings.Trim(w, ".,;:!?\"'()[]"))
		if lw == "" {
			continue
		}
		if _, stop := stopwords[lw]; stop {
			continue
		}
		if len(lw) < 3 {
			continue
		}
		counts[lw]++
	}
	var (
		top  string
		topN int
	)
	for w, n := range counts {
		if n > topN {
			top = w
			topN = n
		}
	}
	if page.WordN == 0 {
		return top, 0
	}
	return top, float64(topN) / float64(page.WordN)
}

func meanInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return int(math.Round(float64(sum) / float64(len(xs))))
}
