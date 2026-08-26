//
// File:        internal/webui/webui_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webui

import (
	"io/fs"
	"testing"
)

func TestFSIsRootedAtPublic(t *testing.T) {
	public := FS()

	// The ".keep" placeholder is committed, so it is present whether or not the
	// WebUI was built into this binary.
	if _, err := fs.Stat(public, KeepFileName); err != nil {
		t.Fatalf("expected %q at the root of the embedded FS: %v", KeepFileName, err)
	}

	if _, err := fs.Stat(public, "public"); err == nil {
		t.Errorf("embedded FS still contains the 'public' prefix, fs.Sub did not apply")
	}
}

func TestIsBuiltMatchesIndexPresence(t *testing.T) {
	_, err := fs.Stat(Public, "public/index.html")
	expected := err == nil

	if got := IsBuilt(); got != expected {
		t.Errorf("expected IsBuilt() to be %v, got %v", expected, got)
	}
}
