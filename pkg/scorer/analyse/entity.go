package analyse

import (
	"regexp"
	"strings"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// EntityInputs bundles the pages the entity-clarity analyser needs.
type EntityInputs struct {
	Homepage *parse.ParsedPage
	About    *parse.ParsedPage
	Articles []*parse.ParsedPage
}

var (
	phoneRE = regexp.MustCompile(`(?i)(?:\+?\d[\d\s().-]{7,}\d)`)
	emailRE = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	// Postal-address heuristic: has a word like "street", "road", "avenue", "suite", ZIP.
	addressRE = regexp.MustCompile(`(?i)\b(street|st\.|road|rd\.|avenue|ave\.|suite|ste\.|boulevard|blvd\.|drive|dr\.|road|highway|hwy\.|zip|post code|postcode)\b`)
)

// Entity scores the E-E-A-T / Entity Clarity dimension. Domain-level.
//
// Weights (methodology §4): 20+15+10+10+25+10+10 = 100.
func Entity(in EntityInputs) Sub {
	findings := []Finding{}
	score := 0

	pages := []*parse.ParsedPage{in.Homepage, in.About}
	pages = append(pages, in.Articles...)

	// 1. Organization schema on homepage with name + url (20 pts).
	if in.Homepage != nil && hasOrgWithNameURL(in.Homepage) {
		score += 20
	} else {
		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Code:     "ENTITY_ORG_SCHEMA_MISSING",
			Message:  "Homepage lacks a complete Organization JSON-LD block (name + url).",
		})
	}

	// 2. Organization.sameAs chain (15 pts): at least 3 social/professional URLs.
	if sameAsCount(findAny(pages, "Organization"), 3) {
		score += 15
	} else {
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "ENTITY_ORG_SAMEAS_MISSING",
			Message:  "Organization schema has fewer than 3 sameAs links (LinkedIn, Crunchbase, Wikipedia, etc.).",
		})
	}

	// 3. /about exists + contains org name (10 pts).
	orgName := orgNameFrom(pages)
	if in.About != nil && in.About.StatusCode != 404 && in.About.WordN > 0 {
		if orgName == "" || strings.Contains(strings.ToLower(in.About.WordText), strings.ToLower(orgName)) {
			score += 10
		} else {
			score += 5
		}
	} else {
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "ENTITY_ABOUT_MISSING",
			Message:  "No /about page reachable on this domain.",
		})
	}

	// 4. NAP completeness (10 pts): 2 of {phone, email, address}.
	nap := countNAP(pages)
	switch {
	case nap >= 2:
		score += 10
	case nap == 1:
		score += 5
	}
	if nap < 2 {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "ENTITY_NAP_INCOMPLETE",
			Message:  "Fewer than 2 of (phone, email, postal address) are discoverable.",
		})
	}

	// 5. Person schema on articles + byline links to bio (25 pts).
	if len(in.Articles) > 0 {
		withPerson := 0
		withBioLink := 0
		for _, a := range in.Articles {
			if a == nil {
				continue
			}
			if hasPersonAuthor(a) {
				withPerson++
			}
			if hasAuthorBylineLink(a) {
				withBioLink++
			}
		}
		fullFrac := float64(withPerson) / float64(len(in.Articles))
		linkFrac := float64(withBioLink) / float64(len(in.Articles))
		add := int(25 * (0.7*fullFrac + 0.3*linkFrac))
		score += add
		if withPerson == 0 {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				Code:     "ENTITY_PERSON_SCHEMA_MISSING",
				Message:  "No article pages expose Person schema for the author.",
			})
		}
		if withBioLink == 0 {
			findings = append(findings, Finding{
				Severity: SeverityMedium,
				Code:     "ENTITY_AUTHOR_BIO_MISSING",
				Message:  "Author bylines do not link to a bio/about page.",
			})
		}
	}

	// 6. Author credentials on a Person schema (10 pts): jobTitle present.
	if hasJobTitleOnPerson(pages) {
		score += 10
	} else {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "ENTITY_AUTHOR_CREDENTIALS_MISSING",
			Message:  "Person schema does not advertise a jobTitle or professional affiliation.",
		})
	}

	// 7. Person.sameAs social proof (10 pts): ≥2.
	if sameAsCountAnyPerson(pages, 2) {
		score += 10
	} else {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "ENTITY_SOCIAL_PROOF_WEAK",
			Message:  "Person schema has fewer than 2 sameAs links (LinkedIn, ORCID, Google Scholar, etc.).",
		})
	}

	return Sub{Score: clamp(score, 0, 100), Findings: findings}
}

func sameAsCount(obj map[string]any, min int) bool {
	if obj == nil {
		return false
	}
	v, ok := obj["sameAs"]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case string:
		return min <= 1
	case []any:
		return len(x) >= min
	}
	return false
}

func sameAsCountAnyPerson(pages []*parse.ParsedPage, min int) bool {
	for _, p := range pages {
		if p == nil {
			continue
		}
		for _, obj := range p.JSONLD {
			if hasType(obj, "Person") && sameAsCount(obj, min) {
				return true
			}
		}
	}
	return false
}

func orgNameFrom(pages []*parse.ParsedPage) string {
	obj := findAny(pages, "Organization")
	if obj == nil {
		return ""
	}
	if s, ok := obj["name"].(string); ok {
		return s
	}
	return ""
}

func countNAP(pages []*parse.ParsedPage) int {
	hasPhone, hasEmail, hasAddr := false, false, false
	for _, p := range pages {
		if p == nil {
			continue
		}
		text := p.WordText
		if !hasPhone && phoneRE.MatchString(text) {
			hasPhone = true
		}
		if !hasEmail && emailRE.MatchString(text) {
			hasEmail = true
		}
		if !hasAddr && addressRE.MatchString(text) {
			hasAddr = true
		}
	}
	n := 0
	if hasPhone {
		n++
	}
	if hasEmail {
		n++
	}
	if hasAddr {
		n++
	}
	return n
}

func hasAuthorBylineLink(p *parse.ParsedPage) bool {
	for _, a := range p.Anchors {
		lowText := strings.ToLower(a.Text)
		lowHref := strings.ToLower(a.Href)
		if strings.Contains(lowHref, "/author/") || strings.Contains(lowHref, "/authors/") ||
			strings.Contains(lowHref, "/team/") || strings.Contains(lowHref, "/people/") ||
			strings.Contains(lowText, "by ") {
			return true
		}
	}
	return false
}

func hasJobTitleOnPerson(pages []*parse.ParsedPage) bool {
	for _, p := range pages {
		if p == nil {
			continue
		}
		for _, obj := range p.JSONLD {
			if !hasType(obj, "Person") {
				continue
			}
			if s, ok := obj["jobTitle"].(string); ok && s != "" {
				return true
			}
		}
	}
	return false
}
