package webui

import "embed"

//go:generate bun install --cwd ../../webui
//go:generate bun --cwd ../../webui build-only --base /web --emptyOutDir --outDir ../internal/webui/public
// Fix .keep being deleted by "--emptyOutDir" so Git does not track it as deleted
//go:generate touch public/.keep

//go:embed public/*
var WebUi embed.FS
