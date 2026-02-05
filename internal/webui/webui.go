package webui

import (
	"embed"
	"io/fs"
)

//go:generate bun install --cwd ../../webui
//go:generate bun --cwd ../../webui build-only --base /web --emptyOutDir --outDir ../internal/webui/public
// Fix .keep being deleted by "--emptyOutDir" so Git does not track it as deleted
//go:generate touch public/.keep

//go:embed public/*
var WebUi embed.FS

func WebUiPublic() fs.FS {
	public, err := fs.Sub(WebUi, "public")
	if err != nil {
		panic(err)
	}
	return public
}
