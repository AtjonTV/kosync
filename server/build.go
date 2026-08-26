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
	"regexp"
	"runtime"
)

// allowedName is what a file name given on the command line may look like.
var allowedName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

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

	// Both values end up as arguments of an executed command, so they are
	// reduced to a plain file name and checked against a strict pattern first.
	out := "." + string(os.PathSeparator) + safeName(*outName, "output executable")

	if *buildWeb {
		run(*goPath, "generate", "./internal/webui")
		writeKeepFile()
	}

	run(*goPath, "build", "-tags", "netgo", "-o", out, ".")

	if *runAfter {
		run(out, "serve")
	}
}

// safeName reduces a path to its file name and rejects anything that is not a
// plain, unsurprising name.
func safeName(value, what string) string {
	name := filepath.Base(value)

	if !allowedName.MatchString(name) {
		panic(fmt.Sprintf("Provided %s name %q is invalid, allowed are letters, digits, '.', '_' and '-'.", what, name))
	}

	return name
}

// defaultOutName returns the platform specific executable name.
func defaultOutName() string {
	if runtime.GOOS == "windows" {
		return "./kosync.exe"
	}

	return "./kosync"
}

// run executes a command and passes its output through.
//
// This is a developer build script (see the "ignore" build tag above): it is
// never compiled into the server, it is started by hand, and the only values
// that reach it are its own flags, which main validates before use.
func run(name string, args ...string) {
	// bearer:disable go_gosec_injection_subproc_injection
	cmd := exec.Command(name, args...) // #nosec G204 -- see the comment above
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
	if err := os.WriteFile(keepFile, nil, 0o600); err != nil {
		panic(err)
	}
}
