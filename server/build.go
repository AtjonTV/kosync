//
// File:        build.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
// Run this file with "go run build.go" to build kosync.
// See the help output for additional options.

//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// keepFile keeps the otherwise empty WebUI output directory in source control.
// "bun build --emptyOutDir" removes it, so it is written back after a build.
const keepFile = "internal/webui/public/.keep"

func main() {
	goPath := flag.String("go", "go", "Path to the go executable (uses PATH otherwise)")
	buildWeb := flag.Bool("web", true, "Build the web interface using Bun")
	outName := flag.String("out", defaultOutName(), "Path of the output executable")
	runAfter := flag.Bool("run", false, "Run the output executable")
	printHelp := flag.Bool("help", false, "Print this help and exit")
	flag.Parse()

	if *printHelp {
		fmt.Printf("Usage: %s [options]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		return
	}

	if base := filepath.Base(*goPath); base != "go" && base != "go.exe" {
		panic(fmt.Sprintf("Provided go executable path is invalid, the binary must be called 'go' but '%s' given!", base))
	}

	out := "." + string(os.PathSeparator) + filepath.Base(*outName)

	if *buildWeb {
		run(*goPath, "generate", "./internal/webui")
		writeKeepFile()
	}

	run(*goPath, "build", "-tags", "netgo", "-o", out, ".")

	if *runAfter {
		run(out, "serve")
	}
}

// defaultOutName returns the platform specific executable name.
func defaultOutName() string {
	if runtime.GOOS == "windows" {
		return "./kosync.exe"
	}

	return "./kosync"
}

// run executes a command and passes its output through.
func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

// writeKeepFile restores the placeholder the WebUI build removed.
func writeKeepFile() {
	if err := os.MkdirAll(filepath.Dir(keepFile), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(keepFile, nil, 0o644); err != nil {
		panic(err)
	}
}
