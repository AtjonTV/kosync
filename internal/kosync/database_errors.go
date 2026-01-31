//
// File:        internal/kosync/database_errors.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "errors"

var UserAlreadyExistsError = errors.New("db: user already exists")
