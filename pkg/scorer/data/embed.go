// Package data embeds the pinned static data files consumed by the
// scorer: the authoritative-domains list and the CMS-fingerprint set.
// The file version is part of the scorer-version stamp (see
// docs/methodology.md "Versioning policy").
package data

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed authoritative.txt
var authoritativeRaw string

//go:embed cms-fingerprints.json
var cmsFingerprintsRaw []byte

// AuthoritativeDomains returns the set of domains in authoritative.txt
// (lowercase, trimmed, comment-and-blank lines skipped).
func AuthoritativeDomains() map[string]struct{} {
	out := map[string]struct{}{}
	for _, line := range strings.Split(authoritativeRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[strings.ToLower(line)] = struct{}{}
	}
	return out
}

// CMSFingerprint is one entry from cms-fingerprints.json.
type CMSFingerprint struct {
	Name        string            `json:"name"`
	Display     string            `json:"display"`
	HeaderMatch map[string]string `json:"header_match"`
	MetaMatch   map[string]string `json:"meta_match"`
	PathMatch   []string          `json:"path_match"`
	Fallback    bool              `json:"fallback"`
}

type cmsFile struct {
	Fingerprints []CMSFingerprint `json:"fingerprints"`
}

// CMSFingerprints returns the parsed fingerprint list in file order.
func CMSFingerprints() []CMSFingerprint {
	var f cmsFile
	if err := json.Unmarshal(cmsFingerprintsRaw, &f); err != nil {
		return nil
	}
	return f.Fingerprints
}
