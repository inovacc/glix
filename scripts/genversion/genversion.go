//go:build ignore

package main

import (
	"github.com/inovacc/genversioninfo"
)

func main() {
	// GenWithCobraCLI generates:
	// - VERSION file (JSON with all metadata) in the root
	// - cmd/VERSION file (version string for embedding)
	// - cmd/version.go (Cobra command with embedded VERSION)
	if err := genversioninfo.GenWithCobraCLI(); err != nil {
		panic(err)
	}
}
