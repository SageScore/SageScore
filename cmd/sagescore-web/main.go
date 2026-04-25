// Command sagescore-web serves the SageScore public audit pages.
//
// Phase 0 status: stub. The full web service (chi router, handlers, queue,
// templates) lands in Phase 2.
package main

import (
	"fmt"

	"github.com/iserter/sagescore/pkg/scorer"
)

func main() {
	fmt.Printf("sagescore-web v%s — web service not yet implemented (Phase 2)\n", scorer.Version)
}
