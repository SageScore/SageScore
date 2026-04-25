// Command sagescore is the SageScore CLI.
//
// Phase 0 status: stub. The full CLI (`sagescore audit <domain>`) lands in
// Phase 1 once the scoring engine is implemented.
package main

import (
	"fmt"
	"os"

	"github.com/iserter/sagescore/pkg/scorer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sagescore audit <domain>")
		os.Exit(2)
	}
	fmt.Printf("sagescore CLI v%s — engine not yet implemented (Phase 1)\n", scorer.Version)
}
