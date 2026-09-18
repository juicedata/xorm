# InsertIgnore

`Engine.InsertIgnore` and `Session.InsertIgnore` accept the same arguments as
`Insert`: structs, pointers, slices, and maps (with `Table` for maps). The method
is also available through `Interface` and `EngineGroup`.

```go
inserted, err := engine.InsertIgnore(&users)

session := engine.NewSession()
defer session.Close()
if err := session.Begin(); err != nil {
    return err
}
if _, err := session.InsertIgnore(&users); err != nil {
    return err
}
return session.Commit()
```

The database keeps the existing record on a duplicate primary or unique key,
without comparing the incoming contents. The affected count is the number of
rows actually inserted. There is no query-before-insert, savepoint, or per-row
retry. The ignore behavior applies only to this call; a subsequent `Insert` on
the same session retains its normal error behavior.

| Database | SQL | Compatibility matrix |
| --- | --- | --- |
| MySQL | `INSERT IGNORE INTO` | 5.7, 8.0, 8.4, latest stable image |
| MariaDB | `INSERT IGNORE INTO` | 10.11, latest stable image |
| PostgreSQL | `ON CONFLICT DO NOTHING` | 9.5, 12, 14, 16, latest stable image; both pq and pgx drivers |
| SQLite | `INSERT OR IGNORE INTO` | 3.24.0, latest stable source release, bundled driver library |

Other dialects return an unsupported error. PostgreSQL requires 9.5 or newer.
MySQL and SQLite use their native IGNORE semantics: some errors beyond duplicate
keys can also be ignored or converted to warnings. This API is not an input
validation mechanism. See the [MySQL INSERT documentation](https://dev.mysql.com/doc/refman/8.4/en/insert.html),
[PostgreSQL INSERT documentation](https://www.postgresql.org/docs/current/sql-insert.html),
and [SQLite conflict documentation](https://www.sqlite.org/lang_conflict.html).

For a single struct, ignored rows do not receive a generated ID, automatic
field values, or after-insert callbacks. For slices, generated IDs are not
assigned, and automatic field values and processors follow the existing bulk
`Insert` behavior for all input records. A bulk callback does not identify which
individual rows were inserted; use single-record calls when this distinction is
required. Before-insert processing still happens before the database decides
whether to skip a row.

## Running compatibility tests

The Docker script creates one temporary database with an automatically assigned
loopback port, runs the regression tests, reports its actual version and image
digest, and removes its container and volumes. It never connects to an existing
local database.

```sh
scripts/test-insert-ignore.sh mysql:5.7
scripts/test-insert-ignore.sh postgres:latest
# On ARM hosts, images without an ARM build need emulation:
XORM_DOCKER_PLATFORM=linux/amd64 scripts/test-insert-ignore.sh mysql:5.7

scripts/test-insert-ignore-sqlite.sh 2018/sqlite-amalgamation-3240000.zip
scripts/test-insert-ignore-sqlite.sh latest
```

The SQLite script downloads the official amalgamation and links the Go sqlite3
driver to that library. `latest` is resolved using the official download page;
the reported database version confirms which library actually ran. The scripts
are also run by `.github/workflows/insert-ignore.yml`. The matrix describes test
coverage, not a guarantee that every SQL-compatible product is supported.
