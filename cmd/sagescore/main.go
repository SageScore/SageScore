// Command sagescore is the SageScore CLI.
//
//	sagescore audit <domain> [-o audit.json] [-v]
//
// Produces a full Audit JSON (or a human-readable summary with no -o).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/iserter/sagescore/pkg/scorer"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "audit":
		if err := cmdAudit(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Printf("sagescore %s\n", scorer.Version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  sagescore audit <domain> [-o audit.json] [-v]")
	fmt.Fprintln(os.Stderr, "  sagescore version")
}

func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	out := fs.String("o", "", "write JSON audit to file")
	verbose := fs.Bool("v", false, "print per-finding detail")
	timeout := fs.Duration("timeout", 120*time.Second, "overall audit timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("missing domain argument")
	}
	domain := fs.Arg(0)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	audit, err := scorer.Run(ctx, domain, scorer.Config{})
	if err != nil {
		return err
	}

	if *out != "" {
		b, err := json.MarshalIndent(audit, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
	}
	printSummary(audit, *verbose)
	return nil
}

func printSummary(a *scorer.Audit, verbose bool) {
	if a.OptedOut {
		fmt.Printf("%s — owner has opted out via robots.txt. No audit published.\n", a.Domain)
		return
	}
	fmt.Printf("SageScore %s @ %s\n", scorer.Version, a.FetchedAt.Format(time.RFC3339))
	fmt.Printf("Domain:  %s\n", a.Domain)
	fmt.Printf("Score:   %d / 100\n", a.Score)
	fmt.Printf("CMS:     %s\n", a.CMS)
	fmt.Printf("Pages:   %d\n", len(a.Pages))
	fmt.Println()
	fmt.Println("Dimensions:")
	for _, dim := range []scorer.Dimension{
		scorer.DimStructuredData,
		scorer.DimAICrawlerAccess,
		scorer.DimContentStructure,
		scorer.DimEntityClarity,
		scorer.DimTechSEOBaseline,
		scorer.DimEvidenceCitation,
	} {
		sub, ok := a.Subscores[dim]
		if !ok {
			continue
		}
		fmt.Printf("  %-24s %3d  (weight %d%%)\n", dim, sub.Score, scorer.Weights[dim])
		if verbose {
			for _, f := range sub.Findings {
				fmt.Printf("    - [%s] %s — %s\n", f.Severity, f.Code, f.Message)
			}
		}
	}

	fmt.Println()
	fmt.Println("Page breakdown:")
	for _, p := range a.Pages {
		fmt.Printf("  %-8s %3d  %s\n", p.Kind, p.Score, p.URL)
	}

	if len(a.Errors) > 0 {
		fmt.Println()
		fmt.Println("Non-fatal errors:")
		for _, e := range a.Errors {
			fmt.Println("  -", e)
		}
	}
}
