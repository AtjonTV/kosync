//
// File:        internal/kosync/database_backup.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
)

func BackupDatabase(cfg *Config, db *sql.DB) error {
	type SQLiteBackup interface {
		NewBackup(string) (*sqlite.Backup, error)
	}

	c, _ := db.Conn(context.Background())
	err := c.Raw(func(driverConn any) error {
		now := time.Now().UTC()
		dbFileName := strings.Replace(filepath.Base(cfg.DatabaseFile), ".db", "", 1)
		newFileName := fmt.Sprintf("%s_%s-%s.bak", dbFileName, now.Format("20060102"), now.Format("150405"))
		bak, err := driverConn.(SQLiteBackup).NewBackup(filepath.Join(filepath.Dir(cfg.DatabaseFile), newFileName))
		if err != nil {
			return err
		}

		morePagesToProcess, err := bak.Step(-1)
		if err != nil {
			return err
		}
		if morePagesToProcess {
			morePagesToProcess, err = bak.Step(-1)
			if err != nil {
				return err
			}
			if morePagesToProcess {
				return fmt.Errorf("failed to backup database")
			}
		}

		return bak.Finish()
	})
	if err != nil {
		return err
	}
	return nil
}

func RestoreDatabase(db *sql.DB, backupFile string) error {
	type SQLiteBackup interface {
		NewRestore(string) (*sqlite.Backup, error)
	}

	c, _ := db.Conn(context.Background())
	err := c.Raw(func(driverConn any) error {
		bak, err := driverConn.(SQLiteBackup).NewRestore(backupFile)
		if err != nil {
			return err
		}

		morePagesToProcess, err := bak.Step(-1)
		if err != nil {
			return err
		}
		if morePagesToProcess {
			morePagesToProcess, err = bak.Step(-1)
			if err != nil {
				return err
			}
			if morePagesToProcess {
				return fmt.Errorf("failed to backup database")
			}
		}

		return bak.Finish()
	})
	if err != nil {
		return err
	}
	return nil
}
