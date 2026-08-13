//
// File:        internal/epub/hash_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub_test

import (
	"bytes"
	"crypto/md5" // #nosec G501 -- mirrors the algorithm under test
	"encoding/csv"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// realHashesEnv names a CSV of "expected hash,path to EPUB" that the binary
// hash is checked against.
//
// The books are not ours to ship and the hashes are personal data, so this
// skips unless supplied. It is the only test that proves the implementation
// agrees with KOReader rather than with itself.
//
//	KOSYNC_REAL_EPUB_HASHES=/path/to/hashes.csv go test ./internal/epub/
const realHashesEnv = "KOSYNC_REAL_EPUB_HASHES"

// TestSampleOffsets pins the offsets exactly.
//
// The first one is the whole point. KOReader computes it with LuaJIT's
// bit.lshift, which masks the shift count to five bits, so the i = -1
// iteration is 1024 << 30 truncated to 32 bits — zero — and not the 256 the
// Lua appears to say. Implementing it the way it reads produces hashes that
// match nothing at all.
func TestSampleOffsets(t *testing.T) {
	want := []int64{
		0, // i = -1, and it is not 256
		1024,
		4096,
		16384,
		65536,
		262144,
		1048576,
		4194304,
		16777216,
		67108864,
		268435456,
		1073741824,
	}

	if len(epub.SampleOffsets) != len(want) {
		t.Fatalf("got %d offsets, want %d", len(epub.SampleOffsets), len(want))
	}
	for index, offset := range want {
		if epub.SampleOffsets[index] != offset {
			t.Errorf("offset %d is %d, want %d", index, epub.SampleOffsets[index], offset)
		}
	}
}

// TestPartialMD5MatchesTheSpecifiedWindows builds the expected digest the long
// way — slicing the documented byte ranges out of the buffer by hand — so the
// test disagrees with the implementation if either drifts.
func TestPartialMD5MatchesTheSpecifiedWindows(t *testing.T) {
	// Large enough to reach the 1 MiB sample, and patterned so that every
	// window differs from every other.
	content := make([]byte, 2<<20)
	for index := range content {
		content[index] = byte(index*7 + index/1024)
	}

	expected := md5.New() // #nosec G401 -- mirrors the algorithm under test
	for _, offset := range epub.SampleOffsets {
		if offset >= int64(len(content)) {
			break
		}
		end := offset + 1024
		if end > int64(len(content)) {
			end = int64(len(content))
		}
		expected.Write(content[offset:end])
	}
	want := hex.EncodeToString(expected.Sum(nil))

	got, err := epub.PartialMD5(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("PartialMD5: %v", err)
	}
	if got != want {
		t.Errorf("hash is %s, want %s", got, want)
	}
}

func TestPartialMD5ShortFiles(t *testing.T) {
	cases := []struct {
		name string
		size int
	}{
		{name: "empty", size: 0},
		{name: "shorter than one sample", size: 300},
		{name: "ends mid-sample", size: 1500},
		{name: "exactly two samples", size: 2048},
	}

	seen := map[string]string{}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			content := make([]byte, testCase.size)
			for index := range content {
				content[index] = byte(index)
			}

			got, err := epub.PartialMD5(bytes.NewReader(content))
			if err != nil {
				t.Fatalf("PartialMD5: %v", err)
			}
			if got == "" {
				t.Fatal("empty hash")
			}
			if previous, clash := seen[got]; clash {
				t.Errorf("%s hashes the same as %s", testCase.name, previous)
			}
			seen[got] = testCase.name
		})
	}
}

func TestFilenameMD5UsesTheBaseNameOnly(t *testing.T) {
	full := epub.FilenameMD5("/home/reader/books/The Witcher/book.epub")
	bare := epub.FilenameMD5("book.epub")
	if full != bare {
		t.Errorf("path %s and base name %s hash differently", full, bare)
	}

	if epub.FilenameMD5("book.epub") == epub.FilenameMD5("other.epub") {
		t.Error("different names hash the same")
	}
}

func TestPartialMD5AgainstRealBooks(t *testing.T) {
	path := os.Getenv(realHashesEnv)
	if path == "" {
		t.Skipf("set %s to check against real KOReader hashes", realHashesEnv)
	}

	file, err := os.Open(path) // #nosec G304 -- test-only path from the developer's environment
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	checked := 0
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		want := strings.TrimSpace(row[0])
		bookPath := strings.TrimSpace(row[1])

		book, err := os.Open(bookPath) // #nosec G304 -- test-only path from the developer's environment
		if err != nil {
			t.Errorf("open %s: %v", bookPath, err)

			continue
		}

		got, err := epub.PartialMD5(book)
		book.Close()
		if err != nil {
			t.Errorf("hash %s: %v", bookPath, err)

			continue
		}

		checked++
		if got != want {
			t.Errorf("%s hashed to %s, KOReader says %s", bookPath, got, want)
		}
	}

	if checked == 0 {
		t.Fatalf("%s listed no readable books", path)
	}
	t.Logf("matched %d real KOReader hashes", checked)
}
