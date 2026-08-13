//
// File:        internal/webui/webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webui

import (
	"embed"
	"io/fs"
)

//go:generate bun install --cwd ../../../webui
//go:generate bun --cwd ../../../webui build-only --emptyOutDir --outDir ../server/internal/webui/public

// Public holds the compiled WebUI. It always contains at least the ".keep"
// placeholder, which is how IsBuilt() tells a real build from an empty tree.
//
//go:embed all:public
var Public embed.FS

// KeepFileName is the placeholder that keeps the otherwise empty public
// directory tracked in source control. "bun build --emptyOutDir" removes it,
// so build.go writes it back after generating the WebUI.
const KeepFileName = ".keep"

// FS returns the compiled WebUI rooted at its index.html.
func FS() fs.FS {
	public, err := fs.Sub(Public, "public")
	if err != nil {
		// Only reachable if the embedded tree is missing, which cannot happen
		// once the package compiles, because "public" is embedded above.
		panic(err)
	}
	return public
}

// IsBuilt reports whether the binary was built with a compiled WebUI.
func IsBuilt() bool {
	_, err := fs.Stat(Public, "public/index.html")
	return err == nil
}
