package scorer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iserter/sagescore/pkg/scorer"
	"github.com/iserter/sagescore/pkg/scorer/crawl"
	"github.com/iserter/sagescore/pkg/scorer/fetch"
)

// Synthetic site with enough substance to exercise all six analysers.
func syntheticMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nAllow: /\n\nUser-agent: GPTBot\nAllow: /\n"))
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		host := "http://" + r.Host
		w.Write([]byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>` + host + `/</loc></url>
<url><loc>` + host + `/about</loc></url>
<url><loc>` + host + `/blog/ai-search-ready</loc><lastmod>2026-03-01</lastmod></url>
<url><loc>` + host + `/blog/statistics-that-matter</loc><lastmod>2026-03-10</lastmod></url>
<url><loc>` + host + `/blog/quoting-sources</loc><lastmod>2026-03-15</lastmod></url>
</urlset>`))
	})
	mux.HandleFunc("/llms.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# Synthetic Test Site\n> A test site for SageScore.\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(homepageHTML))
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(aboutHTML))
	})
	for _, slug := range []string{"ai-search-ready", "statistics-that-matter", "quoting-sources"} {
		slug := slug
		mux.HandleFunc("/blog/"+slug, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(articleHTML(slug)))
		})
	}
	return mux
}

func TestRun_SyntheticSiteEndToEnd(t *testing.T) {
	srv := httptest.NewServer(syntheticMux())
	defer srv.Close()

	orig := crawl.HostDelay
	crawl.HostDelay = 0
	defer func() { crawl.HostDelay = orig }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass the full URL (with http scheme) so the scorer speaks HTTP
	// to the test server.
	audit, err := scorer.Run(ctx, srv.URL, scorer.Config{
		HTTPClient: fetch.NewClient(),
		Now:        time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Assertions: all six dimensions present, score in [0,100], 3 articles found.
	if len(audit.Subscores) != 6 {
		t.Fatalf("expected 6 sub-scores, got %d", len(audit.Subscores))
	}
	if audit.Score < 0 || audit.Score > 100 {
		t.Fatalf("score out of range: %d", audit.Score)
	}
	articles := 0
	for _, p := range audit.Pages {
		if p.Kind == scorer.PageArticle {
			articles++
		}
	}
	if articles < 3 {
		t.Fatalf("expected ≥3 article pages, got %d", articles)
	}
	// Articles-insufficient-sample finding must not appear.
	for _, f := range audit.Subscores[scorer.DimEvidenceCitation].Findings {
		if f.Code == "ARTICLES_INSUFFICIENT_SAMPLE" {
			t.Fatalf("unexpected ARTICLES_INSUFFICIENT_SAMPLE with %d articles", articles)
		}
	}
	// Domain-level AI-Crawler Access should be high (all allowed + llms.txt present).
	if sub := audit.Subscores[scorer.DimAICrawlerAccess]; sub.Score < 70 {
		t.Fatalf("expected AI-crawler sub-score ≥70, got %d", sub.Score)
	}
}

// --- fixtures -------------------------------------------------------------

const homepageHTML = `<!doctype html>
<html><head>
<title>Synthetic — an AI-search-ready demonstration site</title>
<meta name="description" content="Synthetic is a fixture site used to exercise the SageScore auditing engine end-to-end.">
<link rel="canonical" href="http://HOST/">
<meta property="og:title" content="Synthetic">
<meta property="og:description" content="Demo site">
<meta property="og:type" content="website">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="Synthetic">
<script type="application/ld+json">
{"@type":"Organization","name":"Synthetic","url":"http://HOST","sameAs":["https://linkedin.com/company/synthetic","https://crunchbase.com/organization/synthetic","https://wikipedia.org/wiki/Synthetic"]}
</script>
</head><body>
<h1>Synthetic is an AI-search-ready demo site.</h1>
<p>Synthetic is a test site that scores well on SageScore because it was designed to. It has articles, schema, quotes, and outbound citations.</p>
<h2>Why it exists</h2>
<p>The purpose of this site is to test SageScore. It provides reliable answers to fixture questions that the scoring engine can extract.</p>
<ul><li>Organization schema on the homepage.</li><li>FAQ-style headings.</li><li>Articles under /blog.</li></ul>
<h2>What is SageScore?</h2>
<p>SageScore is a free, public audit tool that measures how ready a website is to be cited by AI-search engines.</p>
<h2>How does it work?</h2>
<p>It fetches public HTML and scores six dimensions.</p>
<p>Read our <a href="/blog/ai-search-ready">article on AI-search readiness</a>, our <a href="/blog/statistics-that-matter">piece on statistics</a>, and our <a href="/blog/quoting-sources">guide to quoting sources</a>.</p>
</body></html>`

const aboutHTML = `<!doctype html>
<html><head>
<title>About Synthetic — Contact, Team, Locations</title>
<meta name="description" content="About Synthetic: who we are, what we do, and how to reach us.">
<link rel="canonical" href="http://HOST/about">
<meta property="og:title" content="About">
<meta property="og:description" content="About">
<meta property="og:type" content="article">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="About">
</head><body>
<h1>About Synthetic</h1>
<p>Synthetic is a test organisation for SageScore fixtures.</p>
<p>Email: contact@synthetic.test. Phone: +1 415 555 0100. Address: 123 Example Street, San Francisco, CA.</p>
<h2>Our mission</h2>
<p>To be an AI-search-ready example used by anyone writing test suites for audit tools.</p>
</body></html>`

// articleHTML produces an article page with author, quotes, statistics,
// and outbound authoritative links. dateModified is 2026-03-01 so it's
// fresh relative to the injected Now (2026-04-25).
func articleHTML(slug string) string {
	return `<!doctype html>
<html><head>
<title>` + slug + ` — SageScore article fixture for full audits</title>
<meta name="description" content="An article fixture with author, schema, quotations, and citations for the SageScore audit engine.">
<link rel="canonical" href="http://HOST/blog/` + slug + `">
<meta property="og:title" content="Article">
<meta property="og:description" content="Article">
<meta property="og:type" content="article">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="Article">
<script type="application/ld+json">
{"@type":"Article","headline":"` + slug + `","datePublished":"2026-01-15","dateModified":"2026-03-15","author":{"@type":"Person","name":"Jane Doe","url":"/team/jane","jobTitle":"Staff Writer","sameAs":["https://linkedin.com/in/janedoe","https://orcid.org/0000-0000-0000-0000"]}}
</script>
</head><body>
<article>
<h1>` + slug + `</h1>
<p>AI-search readiness depends on six structural signals. This piece covers them with citations and data.</p>

<h2>What the research shows</h2>
<p>A 2024 study reported a 41% improvement in citation visibility from adding quotations, and a 40% improvement from statistics.</p>
<p>According to the Princeton GEO paper, keyword stuffing has a negative effect on LLM citation — roughly 10%.</p>

<h2>How to apply it</h2>
<ul><li>Add Person schema.</li><li>Quote sources.</li><li>Include numeric facts.</li></ul>
<ol><li>Schema first.</li><li>Structure second.</li><li>Evidence third.</li></ol>

<blockquote>"The effectiveness of generative engine optimisation varies across domains," the authors wrote. According to Dr. Smith, the strongest signals are structural.</blockquote>

<p>For background, see <a href="https://wikipedia.org/wiki/Answer_engine_optimization">Wikipedia on AEO</a>, <a href="https://nih.gov/somepage">NIH research</a>, and <a href="https://arxiv.org/abs/2311.09735">the GEO paper</a>.</p>

<p>Internal links: <a href="/blog/ai-search-ready">one</a>, <a href="/blog/statistics-that-matter">two</a>, <a href="/blog/quoting-sources">three</a>, <a href="/about">about</a>.</p>

<p>By <a href="/team/jane">Jane Doe</a>.</p>
</article>
</body></html>`
}
