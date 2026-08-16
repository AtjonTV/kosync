//
// File:        internal/epub/hash.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package epub reads the parts of an EPUB that KOsync needs: the document
// hashes KOReader identifies a book by, and the metadata the library shows.
//
// The hashes are MD5, and every scanner that looks at this file says so. They
// are not a security decision and cannot be changed here: KOReader computes them
// on the device and sends them as the identity of a document, so a stronger
// digest would simply match nothing. Nothing is authenticated, signed or kept
// secret with them — they are names for files. Hence the #nosec and
// bearer:disable annotations below, one per site rather than a blanket skip, so
// that a new use of MD5 anywhere else is still reported.
package epub

import (
	// bearer:disable go_gosec_blocklist_md5
	"crypto/md5" // #nosec G501 -- KOReader's document identity is MD5; not a security decision
	"encoding/hex"
	"io"
	"path/filepath"
)

// sampleSize is how many bytes KOReader hashes at each offset.
const sampleSize = 1024

// SampleOffsets are the byte offsets KOReader samples for the binary document
// hash, in order.
//
// KOReader computes these as bit.lshift(1024, 2*i) for i = -1..10. That is
// LuaJIT's shift, which masks the shift count to five bits, so the first
// iteration is not 1024 >> 2 = 256 as the Lua reads — it is 1024 << 30
// truncated to 32 bits, which is 0. The first sample is therefore the file
// header.
//
// This is verified against real KOReader hashes (see TestPartialMD5Offsets and
// the fixtures in hash_test.go). Do not "correct" the mask: implementing the
// loop the way it reads produces hashes that match nothing.
var SampleOffsets = sampleOffsets()

func sampleOffsets() []int64 {
	offsets := make([]int64, 0, 12)
	for i := -1; i <= 10; i++ {
		shift := uint((2 * i) & 31)
		offsets = append(offsets, int64(uint32(sampleSize)<<shift))
	}

	return offsets
}

// PartialMD5 returns KOReader's binary document hash: MD5 over 1024-byte
// samples taken at SampleOffsets, stopping at the first offset that reads
// nothing. Two files match only if they are byte-identical in those windows,
// which in practice means byte-identical files.
func PartialMD5(rs io.ReadSeeker) (string, error) {
	// bearer:disable go_gosec_crypto_weak_crypto
	digest := md5.New() // #nosec G401 -- see the package comment
	buffer := make([]byte, sampleSize)

	for _, offset := range SampleOffsets {
		if _, err := rs.Seek(offset, io.SeekStart); err != nil {
			// Seeking past the end is not an error on a file, but it is on
			// some readers. Either way there is nothing left to sample.
			break
		}

		read, err := io.ReadFull(rs, buffer)
		if read > 0 {
			digest.Write(buffer[:read])
		}
		if err != nil {
			// ErrUnexpectedEOF means a short final sample, which still counts;
			// anything else, including EOF, ends the loop.
			break
		}
	}

	// bearer:disable go_lang_weak_hash_md5
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// FilenameMD5 returns KOReader's other document hash, used when the checksum
// method is set to "filename": the MD5 of the base name alone.
//
// It only matches when the reader kept the name the file was served under, so
// acquisition links have to serve a deterministic name.
func FilenameMD5(name string) string {
	// bearer:disable go_gosec_crypto_weak_crypto, go_lang_weak_hash_md5
	sum := md5.Sum([]byte(filepath.Base(name))) // #nosec G401 -- see the package comment

	return hex.EncodeToString(sum[:])
}
