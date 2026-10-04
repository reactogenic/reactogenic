// Package build is the builder: `reactogenic build` (specs/phase02/builder.md).
package build

// The builder's dependencies, held here until the packages that use them
// land (plan.md, RGP2-010…): esbuild as the linker (public API only), the
// embedded engine that executes pages, the HTML parser of the page checks.
import (
	_ "github.com/andybalholm/cascadia"
	_ "github.com/evanw/esbuild/pkg/api"
	_ "golang.org/x/net/html"
	_ "modernc.org/quickjs"
)
