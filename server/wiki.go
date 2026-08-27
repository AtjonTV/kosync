//
// File:        wiki.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
// Run this file with "go run wiki.go" to render the documentation as the files
// of the project wiki. CI pushes the result; see the "wiki" job.

//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"git.obth.eu/atjontv/kosync/internal/wikidocs"
)

func main() {
	root := flag.String("repo", "..", "Path of the repository root")
	out := flag.String("out", "wiki", "Directory the pages are written to")
	url := flag.String("url", "https://git.obth.eu/atjontv/kosync", "Address of the project")
	branch := flag.String("branch", "main", "Branch a link into the repository reads from")
	flag.Parse()

	files, err := wikidocs.Build(os.DirFS(*root), wikidocs.Repo{URL: *url, Branch: *branch})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := os.WriteFile(filepath.Join(*out, name), files[name], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(name)
	}
}
