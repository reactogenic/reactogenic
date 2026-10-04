// Package build is the builder: `reactogenic build` (specs/phase02/builder.md).
package build

// The builder's dependencies, held here until the packages that use them
// land (plan.md, RGP2-020, RGP2-021): the HTML parser of the page checks,
// the selector matcher of the CSS pruner's tests. esbuild and the engine are
// internal/build/render's.
import (
	_ "github.com/andybalholm/cascadia"
	_ "golang.org/x/net/html"
)
