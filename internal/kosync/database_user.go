//
// File:        internal/kosync/database_user.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (db *Database) FindUserBy(field, value string) (*User, bool, error) {
	var findOneUser = fmt.Sprintf("SELECT id, username, password FROM users WHERE %s = ?;", field)
	rows, err := db.rawDb.Query(findOneUser, value)
	if err != nil {
		return nil, false, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	if !rows.Next() {
		return nil, false, nil
	}

	var user User
	err = rows.Scan(&user.Id, &user.Username, &user.Password)

	return &user, true, err
}

func (db *Database) FindUserById(id string) (*User, bool, error) {
	return db.FindUserBy("id", id)
}

func (db *Database) FindUserByUsername(username string) (*User, bool, error) {
	return db.FindUserBy("username", username)
}

func (db *Database) IsUserExistsBy(field, value string) (bool, error) {
	_, found, err := db.FindUserBy(field, value)
	return found, err
}

func (db *Database) IsUserExists(id, username string) (bool, error) {
	found, err := db.IsUserExistsBy("id", id)
	if found {
		return true, nil
	} else if err != nil {
		return false, err
	}
	found, err = db.IsUserExistsBy("username", username)
	if found {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}

func (db *Database) CreateUser(username, password string) (*User, error) {
	newId, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	found, err := db.IsUserExists(newId.String(), username)
	if found {
		return nil, ErrUserAlreadyExists
	} else if err != nil {
		return nil, err
	}

	insertUser := `INSERT INTO users (id, username, password, created_at) VALUES (?, ?, ?, ?)`
	_, err = db.rawDb.Exec(insertUser, newId.String(), username, password, time.Now().Unix())

	user := User{Id: newId.String(), Username: username, Password: password}
	return &user, err
}
