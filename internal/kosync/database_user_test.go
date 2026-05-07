//
// File:        internal/kosync/database_user_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"errors"
	"testing"
)

func TestCreateUser(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a database for testing: %v", err)
	}

	username := "testuser"
	password := "testpassword"

	user, err := db.CreateUser(username, password)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.Username != username {
		t.Errorf("Expected username %s, got %s", username, user.Username)
	}

	if user.Password != password {
		t.Errorf("Expected password %s, got %s", password, user.Password)
	}

	if user.Id == "" {
		t.Error("Expected non-empty user ID")
	}

	// Try to create the same user again
	_, err = db.CreateUser(username, password)
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Errorf("Expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestFindUser(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a database for testing: %v", err)
	}

	username := "findme"
	password := "findpw"

	createdUser, err := db.CreateUser(username, password)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Find by ID
	user, found, err := db.FindUserById(createdUser.Id)
	if err != nil {
		t.Fatalf("FindUserById failed: %v", err)
	}
	if !found {
		t.Fatal("User was not found by ID")
	}
	if user.Username != username {
		t.Errorf("Expected username %s, got %s", username, user.Username)
	}

	// Find by Username
	user, found, err = db.FindUserByUsername(username)
	if err != nil {
		t.Fatalf("FindUserByUsername failed: %v", err)
	}
	if !found {
		t.Fatal("User was not found by username")
	}
	if user.Id != createdUser.Id {
		t.Errorf("Expected user ID %s, got %s", createdUser.Id, user.Id)
	}

	// Find non-existent user
	_, found, err = db.FindUserById("non-existent-id")
	if err != nil {
		t.Fatalf("FindUserById for non-existent user failed: %v", err)
	}
	if found {
		t.Fatal("User found for non-existent ID")
	}
}

func TestIsUserExists(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a database for testing: %v", err)
	}

	username := "existsuser"
	password := "existspw"

	createdUser, err := db.CreateUser(username, password)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Check by ID and username
	exists, err := db.IsUserExists(createdUser.Id, "somethingelse")
	if err != nil {
		t.Fatalf("IsUserExists failed: %v", err)
	}
	if !exists {
		t.Fatal("User should exist by ID")
	}

	exists, err = db.IsUserExists("somethingelse", username)
	if err != nil {
		t.Fatalf("IsUserExists failed: %v", err)
	}
	if !exists {
		t.Fatal("User should exist by username")
	}

	exists, err = db.IsUserExists("nothing", "nobody")
	if err != nil {
		t.Fatalf("IsUserExists failed: %v", err)
	}
	if exists {
		t.Fatal("User should not exist")
	}
}

func TestFindUserBy_Error(t *testing.T) {
	db, _ := NewTemporaryDatabase(true)
	db.Close()
	_, _, err := db.FindUserBy("id", "test")
	if err == nil {
		t.Error("Expected error when database is closed")
	}
}

func TestIsUserExists_Error(t *testing.T) {
	db, _ := NewTemporaryDatabase(true)
	db.Close()
	_, err := db.IsUserExists("id", "username")
	if err == nil {
		t.Error("Expected error when database is closed")
	}
}
