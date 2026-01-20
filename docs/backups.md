# Backup

When a newer version of KOsync is started which changes the database.json schema,  
the server automatically creates a backup file `.bak`.

These backup files are a bit special, instead of just copying the database file,  
they are PEM encoded files.

I have chosen to use PEM for three reasons:

- fun. (just though it would be funny)
- Metadata, PEM has the feature of "Headers"
- Alternative encodings (like [msgpack](https://msgpack.io))

Backup files can be restored by adding the `--restore <path/to/database.bak>` command line option.  
The server will try to restore the database and then start on success.
