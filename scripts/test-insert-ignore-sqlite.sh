#!/usr/bin/env bash
set -euo pipefail

# Link the actual Go driver to an old or current SQLite library, rather than
# testing SQL syntax with a different SQLite executable.
release=${1:-latest}
work=$(mktemp -d "${TMPDIR:-/tmp}/xorm-sqlite.XXXXXX")
trap 'rm -rf "$work"' EXIT
if [[ $release == latest ]]; then
    url=$(curl -fsSL https://www.sqlite.org/download.html | sed -n 's/^PRODUCT,[^,]*,\([^,]*sqlite-amalgamation-[^,]*\.zip\),.*/\1/p' | head -1)
    [[ -n $url ]] || { echo 'Cannot find the latest SQLite amalgamation' >&2; exit 1; }
else
    url=$release
fi
curl -fsSL "https://www.sqlite.org/$url" -o "$work/source.zip"
unzip -q "$work/source.zip" -d "$work"
src=$(echo "$work"/sqlite-amalgamation-*)
cc -O2 -fPIC -DSQLITE_ENABLE_COLUMN_METADATA -c "$src/sqlite3.c" -o "$work/sqlite3.o"
ar rcs "$work/libsqlite3.a" "$work/sqlite3.o"
# Passing the archive explicitly also takes precedence over platform SQLite.
export CGO_CFLAGS="-I$src"
export CGO_LDFLAGS="$work/libsqlite3.a -lpthread -ldl -lm"
go test -tags libsqlite3 ./tests -run '^TestInsertIgnore' -count=1 -v -db=sqlite3 -conn_str="$work/test.db" -show_sql=false
