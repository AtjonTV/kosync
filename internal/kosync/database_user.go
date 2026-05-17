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

var logDbUser = NewKlog("db/user")

func (db *Database) FindUserBy(field, value string) (*User, bool, error) {
	logDbUser.Debug("FindUserBy(field='%s', value='%s')", field, value)
	var findOneUser = fmt.Sprintf("SELECT id, username, password FROM users WHERE %s = ?;", field)
	rows, err := db.rawDb.Query(findOneUser, value)
	if err != nil {
		logDbUser.Error("Failed to find user by '%s' with value '%s': %v", field, value, err.Error())
		return nil, false, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	if !rows.Next() {
		logDbUser.Debug("No user found")
		return nil, false, nil
	}

	var user User
	err = rows.Scan(&user.Id, &user.Username, &user.Password)

	return &user, true, err
}

func (db *Database) FindUserById(id string) (*User, bool, error) {
	logDbUser.Debug("FindUserById('%s')", id)
	return db.FindUserBy("id", id)
}

func (db *Database) FindUserByUsername(username string) (*User, bool, error) {
	logDbUser.Debug("FindUserByUsername('%s')", username)
	return db.FindUserBy("username", username)
}

func (db *Database) IsUserExistsBy(field, value string) (bool, error) {
	logDbUser.Debug("IsUserExistsBy(field='%s', value='%s')", field, value)
	_, found, err := db.FindUserBy(field, value)
	return found, err
}

func (db *Database) IsUserExists(id, username string) (bool, error) {
	logDbUser.Debug("IsUserExists(id='%s', username='%s')", id, username)
	found, err := db.IsUserExistsBy("id", id)
	if found {
		logDbUser.Debug("Found user by id")
		return true, nil
	} else if err != nil {
		logDbUser.Error("Failed to check if user with id '%s' exists: %v", id, err.Error())
		return false, err
	}
	found, err = db.IsUserExistsBy("username", username)
	if found {
		logDbUser.Debug("Found user by name")
		return true, nil
	} else if err != nil {
		logDbUser.Error("Failed to check if user with name '%s' exists: %s", username, err.Error())
		return false, err
	}
	logDbUser.Debug("User was not found and no error occurred")
	return false, nil
}

func (db *Database) CreateUser(username, password string) (*User, error) {
	logDbUser.Debug("CreateUser(username='%s', password='<redacted>')", username)
	newId, err := uuid.NewRandom()
	if err != nil {
		logDbUser.Error("Failed to generate a new random UUIDv4")
		return nil, err
	}
	found, err := db.IsUserExists(newId.String(), username)
	if found {
		logDbUser.Debug("User '%s' already exists", username)
		return nil, ErrUserAlreadyExists
	} else if err != nil {
		logDbUser.Error("Failed to check if user '%s' already exists: %v", username, err.Error())
		return nil, err
	}

	insertUser := `INSERT INTO users (id, username, password, created_at) VALUES (?, ?, ?, ?)`
	_, err = db.rawDb.Exec(insertUser, newId.String(), username, password, time.Now().UnixMicro()/100)
	if err != nil {
		logDbUser.Error("Failed to insert new user: %v", err.Error())
		return nil, err
	}

	user := User{Id: newId.String(), Username: username, Password: password}
	return &user, nil
}
