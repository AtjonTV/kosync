//
// File:        internal/app/kosync/auth.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"
	"net/http"
)

func (app *Kosync) DoAuth(r *http.Request, w http.ResponseWriter) (bool, *UserData) {
	// get the headers
	username := r.Header.Get("x-auth-user")
	password := r.Header.Get("x-auth-key")

	// try to find the user
	user, found := app.Db.Users[username]
	if !found {
		return false, nil
	}

	// verify the passwords match (both are md5 hashed)
	if user.Password != password {
		w.WriteHeader(http.StatusUnauthorized)
		app.DebugPrint(fmt.Sprintf("[user: %s]: Unauthorized access for user with password '%s'", username, password))
		return false, nil
	}

	app.DebugPrint(fmt.Sprintf("[user: %s]: Authorized access for user", username))
	return true, &user
}
