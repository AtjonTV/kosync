//
// File:        internal/webdav/md5.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webdav

import (
	// KOReader hashes its passwords with MD5 before sending them, and the stored
	// credential is bcrypt over that digest. HTTP Basic delivers the plain
	// password, so the same MD5 step happens here instead of on the device.
	// bearer:disable go_gosec_blocklist_md5
	"crypto/md5" // #nosec G501 -- see above
	"fmt"
)

// md5Hex returns the digest a device would have sent for the given password.
func md5Hex(password string) string {
	// bearer:disable go_gosec_crypto_weak_crypto, go_lang_weak_hash_md5
	return fmt.Sprintf("%x", md5.Sum([]byte(password))) // #nosec G401 -- see the import comment
}
