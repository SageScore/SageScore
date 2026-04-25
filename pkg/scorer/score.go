package scorer

import (
	"math"

	"github.com/iserter/sagescore/pkg/scorer/analyse"
)

// aggregate computes the final SageScore and populates the Audit's
// domain-level Subscores by rolling up page-level dimensions.
//
// Roll-up rules (docs/methodology.md): page-level dimensions are
// averaged across pages with homepage 2× / articles 1× / others 1×.
// Domain-level dimensions (ai_crawlers, entity_clarity) are passed
// through.
func aggregate(a *Audit, aiCrawlers, entity analyse.Sub) {
	pageDims := []Dimension{DimStructuredData, DimContentStructure, DimTechSEOBaseline, DimEvidenceCitation}
	a.Subscores = map[Dimension]Sub{}

	for _, dim := range pageDims {
		var weighted, weightSum float64
		var findings []Finding
		for _, pg := range a.Pages {
			sub, ok := pg.Subscores[dim]
			if !ok {
				continue
			}
			w := pageWeight(pg.Kind)
			weighted += float64(sub.Score) * w
			weightSum += w
			for _, f := range sub.Findings {
				findings = append(findings, f)
			}
		}
		var avg int
		if weightSum > 0 {
			avg = int(math.Round(weighted / weightSum))
		}
		a.Subscores[dim] = Sub{Score: avg, Findings: dedupeFindings(findings)}
	}
	a.Subscores[DimAICrawlerAccess] = fromAnalyse(aiCrawlers)
	a.Subscores[DimEntityClarity] = fromAnalyse(entity)

	// Cap Evidence & Citation Readiness at 70 when we found fewer
	// than 3 articles (methodology).
	articleCount := 0
	for _, pg := range a.Pages {
		if pg.Kind == PageArticle {
			articleCount++
		}
	}
	if articleCount < 3 {
		sub := a.Subscores[DimEvidenceCitation]
		if sub.Score > 70 {
			sub.Score = 70
		}
		sub.Findings = append(sub.Findings, Finding{
			Severity: SeverityMedium,
			Code:     "ARTICLES_INSUFFICIENT_SAMPLE",
			Message:  "Fewer than 3 article-kind pages were discoverable; Evidence & Citation Readiness score capped at 70.",
		})
		a.Subscores[DimEvidenceCitation] = sub
	}

	total := 0.0
	for dim, w := range Weights {
		total += float64(a.Subscores[dim].Score) * float64(w)
	}
	a.Score = int(math.RoundToEven(total / 100.0))
}

func pageWeight(kind PageKind) float64 {
	switch kind {
	case PageHomepage:
		return 2
	case PageArticle:
		return 1
	default:
		return 1
	}
}

func fromAnalyse(s analyse.Sub) Sub {
	fs := make([]Finding, 0, len(s.Findings))
	for _, f := range s.Findings {
		fs = append(fs, Finding(f))
	}
	return Sub{Score: s.Score, Findings: fs}
}

// dedupeFindings keeps the first occurrence of each Code.
func dedupeFindings(in []Finding) []Finding {
	seen := map[string]struct{}{}
	out := make([]Finding, 0, len(in))
	for _, f := range in {
		if _, ok := seen[f.Code]; ok {
			continue
		}
		seen[f.Code] = struct{}{}
		out = append(out, f)
	}
	return out
}
