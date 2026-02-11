# Backup and Restore

Stop KOsync and copy the database file to create a backup. Replace the database to restore one.

## Automatic Backups

Before KOsync runs migrations, it will automatically create a backup of the database.  
It will try to write into the same location as `DATABASE_FILE` but with a different file name.

Backup file names have the following format: `${DATABASE_FILE.replace(".db", "")}_${YYYYMMDD}-${HHmmSS}.bak`.

For example, a backup created on the first of January 2026, at 12:34:56 UTC and with a `DATABASE_FILE` of `/home/kosync/data/kosync.db`,  
the backup file will be created as `/home/kosync/data/kosync_20260101-123456.bak`.

## Restore by command

Besides replacing the database file with the backup by hand, you can run KOsync with `--restore=/path/to/backup.bak` to restore the database from a backup.  
However, this will require a pre-existing database file (located at `DATABASE_FILE`).

If you want to restore a backup on a system where KOsync has never run, copy the backup into  
the location you want, rename the file to whatever you like, and set the `DATABASE_FILE` environment variable. (See [config](config.md))

When starting KOsync it will use the database file `DATABASE_FILE` points to. No further action is required.
