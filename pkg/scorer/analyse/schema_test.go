package analyse

import (
	"testing"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

func TestSchema_HomepageWithOrg(t *testing.T) {
	page := &parse.ParsedPage{
		JSONLD: []map[string]any{
			{"@type": "Organization", "name": "Example", "url": "https://example.com"},
			{"@type": "BreadcrumbList"},
		},
	}
	sub := Schema(page, KindHomepage)
	if sub.Score < 80 {
		t.Fatalf("expected ≥80 for homepage with valid Org + breadcrumb, got %d", sub.Score)
	}
}

func TestSchema_ArticleMissingPerson(t *testing.T) {
	page := &parse.ParsedPage{
		JSONLD: []map[string]any{
			{"@type": "Article", "headline": "Hi"},
		},
	}
	sub := Schema(page, KindArticle)
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "SCHEMA_PERSON_AUTHOR_MISSING" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("SCHEMA_PERSON_AUTHOR_MISSING not emitted when author missing")
	}
}

func TestSchema_ArticleWithPersonAuthor(t *testing.T) {
	page := &parse.ParsedPage{
		JSONLD: []map[string]any{
			{
				"@type":  "Article",
				"author": map[string]any{"@type": "Person", "name": "Jane Doe"},
			},
		},
	}
	sub := Schema(page, KindArticle)
	for _, f := range sub.Findings {
		if f.Code == "SCHEMA_PERSON_AUTHOR_MISSING" {
			t.Fatal("should not flag missing author when Person present")
		}
	}
}

func TestSchema_ArticleTotallyMissing(t *testing.T) {
	page := &parse.ParsedPage{}
	sub := Schema(page, KindArticle)
	found := 0
	for _, f := range sub.Findings {
		if f.Code == "SCHEMA_ARTICLE_MISSING" || f.Code == "SCHEMA_PERSON_AUTHOR_MISSING" {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected both Article-missing and Person-missing findings, got %d", found)
	}
	if sub.Score > 10 {
		t.Fatalf("expected low score with no schema, got %d", sub.Score)
	}
}

func TestSchema_FAQHeuristic(t *testing.T) {
	page := &parse.ParsedPage{
		Headings: []parse.Heading{
			{Level: 2, Text: "What is X?"},
			{Level: 2, Text: "How does Y work?"},
			{Level: 2, Text: "Why is Z important?"},
		},
	}
	sub := Schema(page, KindOther)
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "SCHEMA_FAQ_MISSING" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("SCHEMA_FAQ_MISSING not emitted on FAQ-shaped headings")
	}
}
