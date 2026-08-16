//
// File:        internal/webdav/store.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webdav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	dav "golang.org/x/net/webdav"
)

// dirPerm and filePerm are what the store creates things with. Nothing here is
// meant to be readable by anybody but the server process.
const (
	dirPerm  fs.FileMode = 0o700
	filePerm fs.FileMode = 0o600
)

// store is the file system the WebDAV handler serves.
//
// It is not a file system in any general sense, and that is the design. One
// directory per account, containing at most one file, whose name is fixed. A
// sync target for a reader's statistics needs exactly that much and anything
// more would be an invitation: an authenticated device credential could
// otherwise park a few hundred megabytes of anything on somebody else's server.
//
// The owning account comes from the request context, put there by the
// authentication middleware, because the WebDAV handler has one file system for
// every request and the paths it is given say nothing about who is asking.
type store struct {
	root  string
	limit int64

	// stored is called once an upload has been accepted and put in place, with
	// the account it belongs to and where it now is.
	stored func(ownerId, path string)

	// refused is called for every method and name this store turns down.
	//
	// This is here on purpose while the shape of KOReader's own client is still
	// being learned. Refusing quietly would mean a device that cannot sync and a
	// server with nothing to say about why; refusing loudly means the first run
	// against a real reader is also the report on what that reader wanted.
	refused func(operation, name string, err error)
}

// mine returns the calling account's directory, creating it on first use.
func (s store) mine(ctx context.Context) (string, error) {
	owner := ownerFrom(ctx)
	if owner == "" {
		return "", os.ErrPermission
	}

	dir := filepath.Join(s.root, owner)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("prepare the sync directory: %w", err)
	}

	return dir, nil
}

// resolve turns a request path into a real one, or refuses it.
//
// The two answers it can give are the root of the account's own directory and
// the one file that may live in it. Everything else is os.ErrPermission, which
// includes every attempt to climb out: the name is cleaned to an absolute path
// first, so "../../pb_data" is "/pb_data" by the time it is compared against the
// only name that is allowed.
func (s store) resolve(ctx context.Context, name string) (real string, isRoot bool, err error) {
	dir, err := s.mine(ctx)
	if err != nil {
		return "", false, err
	}

	clean := path.Clean("/" + name)
	if clean == "/" {
		return dir, true, nil
	}
	if clean != "/"+FileName {
		return "", false, os.ErrPermission
	}

	return filepath.Join(dir, FileName), false, nil
}

// OpenFile serves the one file, or the directory that holds it.
func (s store) OpenFile(ctx context.Context, name string, flag int, _ fs.FileMode) (dav.File, error) {
	real, isRoot, err := s.resolve(ctx, name)
	if err != nil {
		s.refused("open", name, err)

		return nil, err
	}

	if isRoot {
		if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
			s.refused("write to the directory", name, os.ErrPermission)

			return nil, os.ErrPermission
		}

		// bearer:disable go_gosec_filesystem_filereadtaint
		opened, err := os.Open(real) // #nosec G304 -- resolved above
		if err != nil {
			return nil, err
		}

		return &listing{File: opened}, nil
	}

	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		// bearer:disable go_gosec_filesystem_filereadtaint
		return os.Open(real) // #nosec G304 -- resolved above
	}

	return s.receive(ownerFrom(ctx), real)
}

// receive starts an upload.
//
// It is written beside the final file rather than into it, so that the copy
// already on the server survives an upload that is cut off half way. A reader
// that loses WiFi mid-sync would otherwise be left with a truncated database on
// both ends, and the next merge would take the truncation for the truth.
func (s store) receive(ownerId, final string) (dav.File, error) {
	temp, err := os.CreateTemp(filepath.Dir(final), "."+FileName+".*")
	if err != nil {
		return nil, fmt.Errorf("open the upload: %w", err)
	}
	if err := temp.Chmod(filePerm); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())

		return nil, fmt.Errorf("open the upload: %w", err)
	}

	return &upload{
		File:    temp,
		owner:   ownerId,
		final:   final,
		limit:   s.limit,
		refused: s.refused,
		stored:  s.stored,
	}, nil
}

// Stat describes the directory or the file.
func (s store) Stat(ctx context.Context, name string) (fs.FileInfo, error) {
	real, _, err := s.resolve(ctx, name)
	if err != nil {
		s.refused("stat", name, err)

		return nil, err
	}

	return os.Stat(real)
}

// Mkdir is refused: the only directory here is the one the server makes.
func (s store) Mkdir(_ context.Context, name string, _ fs.FileMode) error {
	s.refused("mkdir", name, os.ErrPermission)

	return os.ErrPermission
}

// RemoveAll deletes the stored file, and only that.
//
// Allowed because a client that wants to start over should be able to, and
// because refusing it would be refusing something the account can already
// achieve by uploading an empty database.
func (s store) RemoveAll(ctx context.Context, name string) error {
	real, isRoot, err := s.resolve(ctx, name)
	if err != nil || isRoot {
		if err == nil {
			err = os.ErrPermission
		}
		s.refused("delete", name, err)

		return err
	}

	return os.Remove(real)
}

// Rename is refused.
//
// There is one name here, so a rename is either a no-op or an attempt to create
// a second one. If it turns out KOReader uploads under a temporary name and
// moves it into place, the log this leaves behind is what will say so.
func (s store) Rename(_ context.Context, oldName, newName string) error {
	s.refused("rename", oldName+" to "+newName, os.ErrPermission)

	return os.ErrPermission
}

// listing is the account's directory, showing only what this endpoint admits to
// having.
//
// The store accepts one name, so a listing that showed anything else would be
// describing a directory the endpoint does not offer: a half-written upload, or
// a file left behind by a version of this code that leaked one. A client asks
// what is here before it decides whether to download, and the honest answer is
// that one file, or nothing.
type listing struct {
	*os.File

	read bool
}

func (l *listing) Readdir(count int) ([]fs.FileInfo, error) {
	if l.read {
		if count <= 0 {
			return nil, nil
		}

		return nil, io.EOF
	}
	l.read = true

	all, err := l.File.Readdir(-1)
	if err != nil {
		return nil, err
	}

	kept := []fs.FileInfo{}
	for _, one := range all {
		if one.Name() == FileName {
			kept = append(kept, one)
		}
	}

	return kept, nil
}

// upload is a file being written by a device.
//
// It exists to do two things the plain file cannot: stop at the size limit
// rather than filling a disk, and refuse to become the stored copy unless what
// arrived is really a statistics database. Both have to happen here because the
// WebDAV handler hands the body straight to Write and reports what Close says.
type upload struct {
	*os.File

	owner   string
	final   string
	limit   int64
	written int64
	refused func(operation, name string, err error)
	stored  func(ownerId, path string)
}

// ErrTooLarge is returned when an upload runs past the configured limit.
var ErrTooLarge = errors.New("the upload is larger than this server accepts")

func (u *upload) Write(data []byte) (int, error) {
	if u.limit > 0 && u.written+int64(len(data)) > u.limit {
		return 0, ErrTooLarge
	}

	written, err := u.File.Write(data)
	u.written += int64(written)

	return written, err
}

// Close validates what arrived and only then lets it replace the stored copy.
//
// A rename is what makes the swap atomic: a reader downloading the file at the
// same moment gets either the whole of the old one or the whole of the new one,
// never the seam between them.
func (u *upload) Close() error {
	temp := u.File.Name()

	if err := u.File.Sync(); err != nil {
		_ = u.File.Close()
		_ = os.Remove(temp)

		return fmt.Errorf("store the upload: %w", err)
	}
	if err := u.File.Close(); err != nil {
		_ = os.Remove(temp)

		return fmt.Errorf("store the upload: %w", err)
	}

	// Whatever happens next, nothing but the stored file may be left in the
	// directory. Validation opens the upload with immutable set so that SQLite
	// writes no sidecars of its own, and this is the second lock on that door:
	// a directory that accumulated two files per sync would fill up quietly and
	// show a reader entries it never wrote.
	defer discard(temp)

	if err := Validate(temp); err != nil {
		u.refused("keep", FileName, err)

		return err
	}

	if err := os.Rename(temp, u.final); err != nil {
		return fmt.Errorf("store the upload: %w", err)
	}

	// Only now, and only on the way out: whatever reads this file wants the one
	// that is in place, not the one still being written.
	if u.stored != nil {
		u.stored(u.owner, u.final)
	}

	return nil
}

// discard removes a temporary upload and anything SQLite may have put beside
// it. A file that was renamed into place is already gone, which is what makes
// this safe to defer.
func discard(temp string) {
	for _, path := range []string{temp, temp + "-shm", temp + "-wal", temp + "-journal"} {
		_ = os.Remove(path)
	}
}

// Readdir belongs to the directory handle and is inherited from *os.File for
// it; on an upload there is nothing to list.
func (u *upload) Readdir(int) ([]fs.FileInfo, error) {
	return nil, os.ErrPermission
}

// modTime is used by the tests and by the log line that says when a device last
// synced.
func modTime(path string) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}

	return info.ModTime(), true
}
