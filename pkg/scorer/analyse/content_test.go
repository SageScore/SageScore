package analyse

import (
	"testing"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

func TestContent_KeywordStuffingPenalty(t *testing.T) {
	words := []string{}
	// 100 words, 40% "widget"
	for i := 0; i < 40; i++ {
		words = append(words, "widget")
	}
	for i := 0; i < 60; i++ {
		words = append(words, "foo")
	}
	page := &parse.ParsedPage{
		Words:      words,
		WordN:      len(words),
		WordText:   joinWords(words),
		Sentences:  []string{"widget widget widget."},
		Paragraphs: []string{"widget widget widget widget widget widget widget."},
	}
	sub := Content(page)
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "CONTENT_KEYWORD_STUFFING" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("CONTENT_KEYWORD_STUFFING not emitted when 40% of body is one term")
	}
}

func TestContent_BLUFMissing(t *testing.T) {
	// First sentence deliberately exceeds 200 chars so the
	// answer-first check fails regardless of which verbs it contains.
	longLead := "The following discussion explores at considerable length a variety " +
		"of topics which may or may not resemble other topics discussed elsewhere " +
		"in similar or not-entirely-similar contexts over many overlapping paragraphs " +
		"that may or may not build on any particular previous paragraph whatsoever."
	if len(longLead) < 200 {
		t.Fatalf("test fixture bug: longLead is %d chars, need >=200", len(longLead))
	}
	page := &parse.ParsedPage{
		Paragraphs: []string{longLead, "second", "third"},
		Sentences:  []string{longLead},
		Words:      []string{"the", "following"},
		WordN:      30,
	}
	sub := Content(page)
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "CONTENT_BLUF_MISSING" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("CONTENT_BLUF_MISSING should fire on long non-answer lead")
	}
}

func TestContent_BLUFPresent(t *testing.T) {
	page := &parse.ParsedPage{
		Paragraphs: []string{"SageScore is a free public audit tool that scores AI-search readiness."},
		Sentences:  []string{"SageScore is a free public audit tool that scores AI-search readiness."},
		Words:      []string{"sagescore", "is", "a", "free", "public", "audit", "tool"},
		WordN:      7,
	}
	sub := Content(page)
	for _, f := range sub.Findings {
		if f.Code == "CONTENT_BLUF_MISSING" {
			t.Fatal("BLUF should not fire on an answer-shaped opening")
		}
	}
}

func joinWords(ws []string) string {
	out := ""
	for i, w := range ws {
		if i > 0 {
			out += " "
		}
		out += w
	}
	return out
}
