// Run this file with "go run build.go" to build kosync.
// See the help output for additional options.

//go:build ignore
package main

import (
	"os"
	"flag"
	"path/filepath"
	"os/exec"
	"fmt"
)

const (
	OutName = "./kosync.exe"
)

func main() {
	goPath := flag.String("go", "go", "Path to the go executable (uses PATH otherwise)")
	buildWeb := flag.Bool("web", true, "Build the web interface using Bun")
	outName := flag.String("out", OutName, "Path of the output executable")
	runAfter := flag.Bool("run", false, "Run the output executable")
	printHelp := flag.Bool("help", false, "Print this help and exit")
	flag.Parse()

	if printHelp != nil && *printHelp {
		fmt.Printf("Usage: %s [options]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		return
	}

    if filepath.Base(*goPath) != "go" {
		panic(fmt.Sprintf("Provided go executable path is invalid, the binary must be called 'go' but '%s' given!", filepath.Base(*goPath)))
	}

	*outName = "." + string(os.PathSeparator) + filepath.Base(*outName)

	if buildWeb != nil && *buildWeb {
		cmd := exec.Command(*goPath, "generate", "kosync.go")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			panic(err)
		}
	}

	cmd := exec.Command(*goPath, "build", "-tags", "netgo", "-o", *outName, "kosync.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	if runAfter != nil && *runAfter {
		cmd = exec.Command(*outName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			panic(err)
		} 
	}
}
