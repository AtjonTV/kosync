package webui

import "embed"

//go:generate bun --cwd ../../webui build-only --base /web --outDir ../internal/webui/public

//go:embed public/*
var WebUi embed.FS
