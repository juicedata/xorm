// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"

	"github.com/stretchr/testify/assert"
)

// qualifiedTableName returns tableName prefixed with the current schema
// (when one is configured via -schema), the same way Sync itself resolves
// table names internally. Raw DDL run through testEngine.Exec must use
// this, or it lands in the connection's default search path instead of the
// schema Sync and GetColumns are actually looking at.
func qualifiedTableName(tableName string) string {
	return testEngine.TableName(tableName, true)
}

// dropTestTable drops tableName ignoring errors, so tests can clean up
// regardless of whether the table was actually created.
func dropTestTable(tableName string) {
	_, _ = testEngine.Exec("DROP TABLE IF EXISTS " + qualifiedTableName(tableName))
}

// assertTableVisible fails t if tableName is not visible to the engine
// through the same schema resolution Sync itself uses (IsTableExist
// resolves the schema internally, just like Sync/GetColumns do). Without
// this, a raw DDL table created under the wrong schema makes Sync silently
// create a fresh table and compare nothing, so every "must be silent"
// assertion in this file would pass vacuously.
func assertTableVisible(t *testing.T, tableName string) {
	t.Helper()

	exists, err := testEngine.IsTableExist(tableName)
	assert.NoError(t, err)
	assert.True(t, exists, "raw DDL table %q must be visible to the engine before Sync runs", tableName)
}

// columnFromDBMetas fetches tableName's columnName column straight from
// DBMetas, independently of Sync, so a test can assert on what GetColumns
// actually read back rather than on what CompareColumns concluded from it.
func columnFromDBMetas(t *testing.T, tableName, columnName string) *schemas.Column {
	t.Helper()

	tables, err := testEngine.DBMetas()
	assert.NoError(t, err)

	for _, table := range tables {
		if !strings.EqualFold(table.Name, tableName) {
			continue
		}
		col := table.GetColumn(columnName)
		assert.NotNil(t, col, "expected column %q on table %q", columnName, tableName)
		return col
	}

	t.Fatalf("table %q not found via DBMetas", tableName)
	return nil
}

// TestSyncWarningMySQLUnsignedIntDisplayWidth reproduces
// go-gitea/gitea#22275: "db type is INT(10) UNSIGNED, struct type is INT
// UNSIGNED" must not be a warning. MariaDB reports the display width back
// through GetColumns; MySQL 8 does not, but the assertion only checks for
// the absence of a warning about the column, not the rendered type string.
func TestSyncWarningMySQLUnsignedIntDisplayWidth(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MYSQL {
		t.Skip("mysql/mariadb only")
	}

	const tableName = "sync_warning_unsigned_int_width"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT PRIMARY KEY, num_watches INT(10) UNSIGNED NOT NULL DEFAULT 0)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningUnsignedIntWidth struct {
		Id         int64
		NumWatches uint32 `xorm:"NOT NULL DEFAULT 0"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningUnsignedIntWidth)))
	assert.False(t, recorder.hasMessageContaining("num_watches"),
		"expected no warning about num_watches, got: %v", recorder.messages())
}

// TestSyncWarningOptimisticLockNoDBDefault reproduces go-gitea/gitea#22275:
// "Column version db default is , struct default is 1" must not be a
// warning. VersionTagHandler sets Default="1" on the struct column but
// leaves DefaultIsEmpty=true, so a database column with no default at all
// should compare as equivalent on every dialect. The table is deliberately
// not named after the "version" column, so hasMessageContaining("version")
// cannot accidentally match the table name inside every warning line.
func TestSyncWarningOptimisticLockNoDBDefault(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	const tableName = "sync_warning_optimistic_lock_no_default"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	var ddl string
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, version INT NOT NULL)", qualifiedTableName(tableName))
	case schemas.POSTGRES:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, version INTEGER NOT NULL)", qualifiedTableName(tableName))
	case schemas.SQLITE:
		ddl = fmt.Sprintf("CREATE TABLE %s (id INTEGER NOT NULL PRIMARY KEY, version INTEGER NOT NULL)", qualifiedTableName(tableName))
	default:
		t.Skip("mysql/postgres/sqlite3 only")
	}

	_, err := testEngine.Exec(ddl)
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningOptimisticLockNoDefault struct {
		Id      int64 `xorm:"pk"`
		Version int   `xorm:"version notnull"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningOptimisticLockNoDefault)))
	assert.False(t, recorder.hasMessageContaining("version"),
		"expected no warning about version, got: %v", recorder.messages())
}

// TestSyncWarningPostgresBoolDefaultTrue reproduces go-gitea/gitea#22275:
// "db default is true, struct default is TRUE" must not be a warning.
func TestSyncWarningPostgresBoolDefaultTrue(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_bool_default_true"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, is_active BOOLEAN NOT NULL DEFAULT true)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningBoolDefaultTrue struct {
		Id       int64 `xorm:"pk"`
		IsActive bool  `xorm:"NOT NULL default(TRUE)"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningBoolDefaultTrue)))
	assert.False(t, recorder.hasMessageContaining("is_active"),
		"expected no warning about is_active, got: %v", recorder.messages())
}

// TestSyncWarningPostgresIntDefaultNegative reproduces go-gitea/gitea#22275:
// "db default is '-1', struct default is -1" must not be a warning.
func TestSyncWarningPostgresIntDefaultNegative(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_int_default_negative"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, num_stars INTEGER NOT NULL DEFAULT '-1')", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningIntDefaultNegative struct {
		Id       int64 `xorm:"pk"`
		NumStars int   `xorm:"NOT NULL default(-1)"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningIntDefaultNegative)))
	assert.False(t, recorder.hasMessageContaining("num_stars"),
		"expected no warning about num_stars, got: %v", recorder.messages())
}

// TestSyncWarningPostgresNullableDefaultNull reproduces go-gitea/gitea#22275:
// "Column archived_unix db default is , struct default is NULL" must not be
// a warning for a nullable column with no database default.
func TestSyncWarningPostgresNullableDefaultNull(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_nullable_default_null"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, archived_unix BIGINT)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningNullableDefaultNull struct {
		Id           int64 `xorm:"pk"`
		ArchivedUnix int64 `xorm:"NULL default(NULL)"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningNullableDefaultNull)))
	assert.False(t, recorder.hasMessageContaining("archived_unix"),
		"expected no warning about archived_unix, got: %v", recorder.messages())
}

// TestSyncWarningPostgresVarcharReadBack is a regression guard for
// dialects/postgres.go's GetColumns, which maps the "character varying"
// data_type reported by information_schema.columns back onto
// schemas.Varchar with the correct length. It asserts the read-back length
// directly through DBMetas rather than relying on Sync's silence, because
// CompareColumns's base-name fallback would collapse any VARCHAR(N) vs
// VARCHAR(255) length difference to ColumnCompareEquivalent, masking a
// GetColumns regression that reads back the wrong length (verified by
// forcing GetColumns to report Length=42 for this column, which turns the
// length assertion red while Sync itself stays silent). It does NOT
// exercise compare.go's alias/normalisation logic: postgres already reads
// the column back as "VARCHAR(255)", which is byte-identical to what an
// explicit `xorm:"VARCHAR(255)"` struct tag renders, so CompareColumns
// takes the ColumnCompareEqual fast path before any normalisation runs.
func TestSyncWarningPostgresVarcharReadBack(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_varchar_readback"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name CHARACTER VARYING(255) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 255, col.Length,
		"expected GetColumns to read the character varying(255) length back as 255")

	type SyncWarningVarcharReadBack struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(255) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningVarcharReadBack)))
	assert.False(t, recorder.hasMessageContaining("lower_name"),
		"expected no warning about lower_name, got: %v", recorder.messages())
}

// TestSyncWarningPostgresNumericPrecisionReadBack asserts that a
// NUMERIC(10,2) column reads back with its declared precision and scale
// (dialects/postgres.go's GetColumns query selects numeric_precision and
// numeric_scale from information_schema.columns), so it renders as
// "NUMERIC(10,2)" and matches an explicit `xorm:"DECIMAL(10,2)"` struct
// tag without a Sync warning. See go-gitea/gitea#22275.
func TestSyncWarningPostgresNumericPrecisionReadBack(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_numeric_precision_readback"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price NUMERIC(10,2) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningNumericPrecisionReadBack struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningNumericPrecisionReadBack)))
	assert.False(t, recorder.hasMessageContaining("column price db type is"),
		"expected no warning about price, got: %v", recorder.messages())
}

// TestSyncWarningMySQLGenuineUnsignedMismatch is the negative counterpart
// to TestSyncWarningMySQLUnsignedIntDisplayWidth: a signed INT(11) column
// against an unsigned struct field is genuine drift and must warn. The
// assertion is scoped to the type-mismatch message text, not just the
// column name, so it cannot be satisfied by an unrelated warning.
func TestSyncWarningMySQLGenuineUnsignedMismatch(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MYSQL {
		t.Skip("mysql/mariadb only")
	}

	const tableName = "sync_warning_genuine_unsigned_mismatch"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, num_watches INT(11) NOT NULL DEFAULT 0)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningGenuineUnsignedMismatch struct {
		Id         int64  `xorm:"pk"`
		NumWatches uint32 `xorm:"NOT NULL DEFAULT 0"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningGenuineUnsignedMismatch)))
	assert.True(t, recorder.hasMessageContaining("column num_watches db type is"),
		"expected a type mismatch warning about num_watches, got: %v", recorder.messages())
}

// TestSyncWarningPostgresGenuineWidthMismatch is the negative counterpart to
// the silent normalisation cases above: a postgres INTEGER column against a
// BIGINT struct field is a genuine width difference and must warn. The
// assertion is scoped to the type-mismatch message text, not just the
// column name, so it cannot be satisfied by an unrelated warning.
func TestSyncWarningPostgresGenuineWidthMismatch(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_genuine_width_mismatch"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, num_stars INTEGER NOT NULL DEFAULT 0)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningGenuineWidthMismatch struct {
		Id       int64 `xorm:"pk"`
		NumStars int64 `xorm:"BIGINT NOT NULL DEFAULT 0"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningGenuineWidthMismatch)))
	assert.True(t, recorder.hasMessageContaining("column num_stars db type is"),
		"expected a type mismatch warning about num_stars, got: %v", recorder.messages())
}

// TestSyncWarningNullableMismatch reproduces go-gitea/gitea#22275's
// "db nullable is true, struct nullable is false" reports (gitea's
// email_address.lower_email, gpg_key.content, tracked_time.time): a
// nullable database column against a `notnull` struct field is genuine
// drift and must warn, on every dialect. The assertion is scoped to the
// full nullable-mismatch message: on sqlite3, Sync also emits an unrelated
// "db type is VARCHAR(255), struct type is TEXT" warning for this same
// column (sqlite renders a Go string as TEXT while GetColumns parses
// VARCHAR(255) back out of the DDL text), and a bare column-name substring
// match would pass on that message alone without ever proving the
// nullable comparison was exercised.
func TestSyncWarningNullableMismatch(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	const tableName = "sync_warning_nullable_mismatch"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	var ddl string
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_email VARCHAR(255))", qualifiedTableName(tableName))
	case schemas.POSTGRES:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_email VARCHAR(255))", qualifiedTableName(tableName))
	case schemas.SQLITE:
		ddl = fmt.Sprintf("CREATE TABLE %s (id INTEGER NOT NULL PRIMARY KEY, lower_email VARCHAR(255))", qualifiedTableName(tableName))
	case schemas.MSSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_email VARCHAR(255))", qualifiedTableName(tableName))
	default:
		t.Skip("mysql/postgres/sqlite3/mssql only")
	}

	_, err := testEngine.Exec(ddl)
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningNullableMismatch struct {
		Id         int64  `xorm:"pk"`
		LowerEmail string `xorm:"VARCHAR(255) notnull"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningNullableMismatch)))
	assert.True(t, recorder.hasMessageContaining("Column lower_email db nullable is true, struct nullable is false"),
		"expected a nullable mismatch warning about lower_email, got: %v", recorder.messages())
}

// TestSyncWarningDefaultAndNullableMismatch reproduces go-gitea/gitea#22275's
// combined "Column card_type db default is 0, struct default is" and
// "db nullable is false, struct nullable is true" report: a database column
// with a default and NOT NULL against a struct field with neither is
// genuine drift on both fronts, and must warn twice. Both assertions are
// scoped with the column name so neither can be satisfied by a warning
// about a different column.
func TestSyncWarningDefaultAndNullableMismatch(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	const tableName = "sync_warning_default_and_nullable_mismatch"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	var ddl string
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, card_type INT NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	case schemas.POSTGRES:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, card_type INTEGER NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	case schemas.SQLITE:
		ddl = fmt.Sprintf("CREATE TABLE %s (id INTEGER NOT NULL PRIMARY KEY, card_type INTEGER NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	case schemas.MSSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, card_type INT NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	default:
		t.Skip("mysql/postgres/sqlite3/mssql only")
	}

	_, err := testEngine.Exec(ddl)
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningDefaultAndNullableMismatch struct {
		Id       int64 `xorm:"pk"`
		CardType int   `xorm:"NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningDefaultAndNullableMismatch)))
	assert.True(t, recorder.hasMessageContaining("Column card_type db default is 0, struct default is"),
		"expected a default mismatch warning about card_type, got: %v", recorder.messages())
	assert.True(t, recorder.hasMessageContaining("Column card_type db nullable is false, struct nullable is true"),
		"expected a nullable mismatch warning about card_type, got: %v", recorder.messages())
}

// TestSyncWarningDatabaseColumnMissed reproduces go-gitea/gitea#22275's
// "Table hook_task has column repo_id but struct has not related field"
// report: an extra database column that has no matching struct field is
// silent by default and only warns when WarnIfDatabaseColumnMissed is set.
func TestSyncWarningDatabaseColumnMissed(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	const tableName = "hook_task"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	var ddl string
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, repo_id BIGINT NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	case schemas.POSTGRES:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, repo_id BIGINT NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	case schemas.SQLITE:
		ddl = fmt.Sprintf("CREATE TABLE %s (id INTEGER NOT NULL PRIMARY KEY, repo_id INTEGER NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	case schemas.MSSQL:
		ddl = fmt.Sprintf("CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, repo_id BIGINT NOT NULL DEFAULT 0)", qualifiedTableName(tableName))
	default:
		t.Skip("mysql/postgres/sqlite3/mssql only")
	}

	_, err := testEngine.Exec(ddl)
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type HookTask struct {
		Id int64 `xorm:"pk"`
	}

	const missedColumnMessage = "has column repo_id but struct has not related field"

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(HookTask)))
	assert.False(t, recorder.hasMessageContaining(missedColumnMessage),
		"expected no warning about repo_id by default, got: %v", recorder.messages())

	_, err = testEngine.Table(tableName).SyncWithOptions(xorm.SyncOptions{WarnIfDatabaseColumnMissed: true}, new(HookTask))
	assert.NoError(t, err)
	assert.True(t, recorder.hasMessageContaining(missedColumnMessage),
		"expected a warning about repo_id with WarnIfDatabaseColumnMissed, got: %v", recorder.messages())
}

// TestSyncWarningSQLiteBoolDefaultRoundTrip reproduces go-gitea/gitea#22275's
// "Column keep_activity_private db default is , struct default is 0" report
// for sqlite3. sqlite3's GetColumns parses column metadata out of the
// original CREATE TABLE text (dialects/sqlite3.go's parseString), instead
// of a system catalog, so it is worth checking both DDL shapes users
// report: one where the column genuinely has a matching DEFAULT clause
// (silent, since sqlite has no BOOLEAN storage class and bool columns are
// declared INTEGER), and one where it does not (a genuine warning). Using
// a literal BOOLEAN column type here instead of INTEGER reproduces a
// different, unrelated bug: dialects/sqlite3.go's SQLType only normalises
// schemas.Bool ("BOOL"), not schemas.Boolean ("BOOLEAN"), to INTEGER, so it
// is important to match what xorm itself would have generated for this
// column (INTEGER) rather than an arbitrary DDL spelling.
//
// The "matching DEFAULT 0 clause" case is a regression guard for
// dialects/sqlite3.go's parseString DEFAULT-clause parsing rather than for
// compare.go's default-normalisation branches: the db and struct defaults
// are already the byte-identical string "0", so compareColumnDefaults
// takes its literal-equality fast path before any normalisation runs. It
// still meaningfully guards sqlite3's DDL-text parsing (verified by
// disabling parseString's "DEFAULT" case, which turns this subtest red).
func TestSyncWarningSQLiteBoolDefaultRoundTrip(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.SQLITE {
		t.Skip("sqlite3 only")
	}

	type SyncWarningSQLiteBoolDefault struct {
		Id                  int64 `xorm:"pk"`
		KeepActivityPrivate bool  `xorm:"NOT NULL default(0)"`
	}

	t.Run("db column has a matching DEFAULT 0 clause", func(t *testing.T) {
		const tableName = "sync_warning_sqlite_bool_default_with_clause"
		dropTestTable(tableName)
		defer dropTestTable(tableName)

		_, err := testEngine.Exec(fmt.Sprintf(
			"CREATE TABLE %s (id INTEGER NOT NULL PRIMARY KEY, keep_activity_private INTEGER NOT NULL DEFAULT 0)", qualifiedTableName(tableName)))
		assert.NoError(t, err)
		assertTableVisible(t, tableName)

		recorder := captureWarnings(t)
		assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningSQLiteBoolDefault)))
		assert.False(t, recorder.hasMessageContaining("keep_activity_private"),
			"expected no warning about keep_activity_private, got: %v", recorder.messages())
	})

	t.Run("db column has no DEFAULT clause", func(t *testing.T) {
		const tableName = "sync_warning_sqlite_bool_default_without_clause"
		dropTestTable(tableName)
		defer dropTestTable(tableName)

		_, err := testEngine.Exec(fmt.Sprintf(
			"CREATE TABLE %s (id INTEGER NOT NULL PRIMARY KEY, keep_activity_private INTEGER NOT NULL)", qualifiedTableName(tableName)))
		assert.NoError(t, err)
		assertTableVisible(t, tableName)

		recorder := captureWarnings(t)
		assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningSQLiteBoolDefault)))
		assert.True(t, recorder.hasMessageContaining("Column keep_activity_private db default is , struct default is 0"),
			"expected a default mismatch warning since the db column genuinely has no default, got: %v", recorder.messages())
	})
}

// TestSyncWarningMSSQLVarcharLengthChangeNotApplied reproduces the
// dialects/mssql.go ColumnSyncFeatures zero value
// (VarcharLengthChange: false): a database VARCHAR(64) column against a
// struct wanting VARCHAR(255) must not warn and, more importantly, must
// not be widened. mssql's mssql.Features() leaves VarcharLengthChange at
// its zero value, mirroring v1's hardcoded DBType checks that never
// widened varchar columns on mssql/oracle/dameng. The length assertion is
// the load-bearing one here: compareColumnTypes's base-sql-type-name
// fallback already classifies any VARCHAR(N) vs VARCHAR(M) pair as
// ColumnCompareEquivalent rather than ColumnCompareDifferent (see
// TestSyncWarningPostgresVarcharReadBack's doc comment above), so the
// warning stays empty regardless of VarcharLengthChange; only reading the
// column back through DBMetas proves no ALTER ran (verified by
// temporarily forcing mssql's CompareColumns to report the type as
// genuinely different and VarcharLengthChange as true, which widens the
// column to 255 and turns this assertion red).
func TestSyncWarningMSSQLVarcharLengthChangeNotApplied(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MSSQL {
		t.Skip("mssql only")
	}

	const tableName = "sync_warning_mssql_varchar_length_not_applied"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR(64) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningMSSQLVarcharLengthNotApplied struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(255) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMSSQLVarcharLengthNotApplied)))
	assert.False(t, recorder.hasMessageContaining("lower_name"),
		"expected no warning about lower_name, got: %v", recorder.messages())

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 64, col.Length,
		"expected lower_name to stay VARCHAR(64) since mssql cannot widen varchar columns during sync")
}

// TestSyncWarningMSSQLTextFromVarcharNotApplied reproduces the
// dialects/mssql.go ColumnSyncFeatures zero value (TextFromVarchar:
// false): a database VARCHAR(64) column against a struct wanting TEXT
// must not warn and must not be altered. Unlike mysql, where TEXT renders
// as the literal string "TEXT" and a mismatched VARCHAR column genuinely
// warns ("db type is VARCHAR(64), struct type is TEXT") unless
// TextFromVarchar is set, mssql's SQLType renders schemas.Text as
// "VARCHAR(MAX)" (see dialects/mssql.go's SQLType, case schemas.Text).
// That means resolveColumnTypeSyncAction's `expectedType == schemas.Text`
// branch can never match on mssql - expectedType is always "VARCHAR(MAX)",
// never the bare string "TEXT" - so this column is silent independent of
// TextFromVarchar (verified by temporarily changing mssql's SQLType to
// render schemas.Text as the literal "TEXT", which turns both assertions
// below red: a warning appears and the column is left VARCHAR(64) instead
// of being reported as already matching).
func TestSyncWarningMSSQLTextFromVarcharNotApplied(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MSSQL {
		t.Skip("mssql only")
	}

	const tableName = "sync_warning_mssql_text_from_varchar_not_applied"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, description VARCHAR(64) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningMSSQLTextFromVarcharNotApplied struct {
		Id          int64  `xorm:"pk"`
		Description string `xorm:"TEXT NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMSSQLTextFromVarcharNotApplied)))
	assert.False(t, recorder.hasMessageContaining("description"),
		"expected no warning about description, got: %v", recorder.messages())

	col := columnFromDBMetas(t, tableName, "description")
	assert.EqualValues(t, 64, col.Length,
		"expected description to stay VARCHAR(64) since mssql renders TEXT as VARCHAR(MAX)")
}

// TestSyncWarningMSSQLLegacyDatetimeMatchesTimeTime pins a genuinely
// mssql-specific type-normalisation case: a database column declared with
// the legacy DATETIME type against a struct field of time.Time (which
// mssql's SQLType always renders as "DATETIME2", see dialects/mssql.go's
// SQLType, case schemas.TimeStamp, schemas.DateTime) must not warn.
// dialects/mssql.go's GetColumns maps both raw DATETIME and DATETIME2
// columns back onto the same abstract schemas.DateTime SQLType name, and
// CompareColumns renders that name through mssql's SQLType a second time
// before comparing, so the comparison is between two freshly rendered
// "DATETIME2" strings rather than between the column's original raw DDL
// text and the struct's rendered type - the two sides are byte-identical
// by construction (verified by temporarily mapping GetColumns's "DATETIME"
// case onto a different SQLType name, which turns this assertion red with
// a "db type is VARCHAR(MAX)(3), struct type is DATETIME2" warning).
func TestSyncWarningMSSQLLegacyDatetimeMatchesTimeTime(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MSSQL {
		t.Skip("mssql only")
	}

	const tableName = "sync_warning_mssql_legacy_datetime"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, created_at DATETIME NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningMSSQLLegacyDatetime struct {
		Id        int64     `xorm:"pk"`
		CreatedAt time.Time `xorm:"NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMSSQLLegacyDatetime)))
	assert.False(t, recorder.hasMessageContaining("created_at"),
		"expected no warning about created_at, got: %v", recorder.messages())
}

// TestSyncWarningMySQLDecimalCommentDoesNotNarrowColumn is the
// live-database reproduction of xorm/xorm#2591: a real MySQL/MariaDB
// DECIMAL(19,4) column holding 1.2345, against a struct tagged
// DECIMAL(10,2) with a differing comment, is ColumnCompareEquivalent (not
// Equal) at the base-sql-type-name level. Before
// resolveCommentSyncDecision gated comment sync on ColumnCompareEqual,
// applyColumnSyncDecision's modifyComment branch ran
// ModifyColumnSQL(tableName, expected) - "ALTER TABLE ... MODIFY price
// DECIMAL(10,2) COMMENT 'new comment'" - as a side effect of "just"
// syncing the comment, which silently rounds the stored value to the
// struct's narrower scale. The read-back and stored-value assertions
// below are load-bearing: the absence of a type-mismatch warning is not
// enough on its own, since the old ModifyColumnSQL call logged nothing
// either. The skip itself logs at Infof, not Warnf - see
// resolveCommentSyncDecision - so recorder.hasMessageContaining below
// matches an Info-level notice, not a warning.
func TestSyncWarningMySQLDecimalCommentDoesNotNarrowColumn(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MYSQL {
		t.Skip("mysql/mariadb only")
	}

	const tableName = "sync_warning_mysql_decimal_comment_no_narrow"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price DECIMAL(19,4) NOT NULL COMMENT 'old comment')",
		qualifiedTableName(tableName)))
	assert.NoError(t, err)
	_, err = testEngine.Exec(fmt.Sprintf(
		"INSERT INTO %s (id, price) VALUES (1, 1.2345)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 19, before.Length, "sanity: decimal(19,4) column should read back with precision 19")
	assert.EqualValues(t, 4, before.Length2, "sanity: decimal(19,4) column should read back with scale 4")

	type SyncWarningMySQLDecimalCommentNoNarrow struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMySQLDecimalCommentNoNarrow)))
	assert.True(t, recorder.hasMessageContaining("comment not synced"),
		"expected a comment-sync-skipped notice about price, got: %v", recorder.messages())
	assert.False(t, recorder.warnfHasMessageContaining("comment not synced"),
		"expected the comment-sync-skipped notice to log at Infof, not Warnf: got Warnf messages %v", recorder.warnfMessages())
	assert.Equal(t, []string{"Table sync_warning_mysql_decimal_comment_no_narrow column price comment not synced because db type is DECIMAL(19,4), struct type is DECIMAL(10,2)"},
		recorder.infofMessages(), "expected exactly one Infof notice, with no other Infof calls for this column")

	after := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 19, after.Length,
		"expected Sync to leave the decimal(19,4) column's precision unchanged even though its comment differs from the struct tag")
	assert.EqualValues(t, 4, after.Length2,
		"expected Sync to leave the decimal(19,4) column's scale unchanged even though its comment differs from the struct tag")
	assert.Equal(t, "old comment", after.Comment,
		"expected the comment to stay unsynced since the rendered types are not byte-identical")

	var price float64
	has, err := testEngine.Table(tableName).Cols("price").Where("id = ?", 1).Get(&price)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.InDelta(t, 1.2345, price, 0.00001,
		"expected the stored value to survive Sync without being rounded to the struct tag's narrower scale")
}

type syncWarningPostgresDecimal1010_2 struct {
	Id    int64   `xorm:"pk"`
	Price float64 `xorm:"DECIMAL(10,2) NOT NULL"`
}

type syncWarningPostgresNumeric1010_2 struct {
	Id    int64   `xorm:"pk"`
	Price float64 `xorm:"NUMERIC(10,2) NOT NULL"`
}

type syncWarningPostgresDecimal19_4 struct {
	Id    int64   `xorm:"pk"`
	Price float64 `xorm:"DECIMAL(19,4) NOT NULL"`
}

type syncWarningPostgresNumeric19_4 struct {
	Id    int64   `xorm:"pk"`
	Price float64 `xorm:"NUMERIC(19,4) NOT NULL"`
}

// TestSyncWarningPostgresNumericDecimalSynonymsMatchSymmetrically is the
// live-database counterpart of
// TestResolveColumnTypeSyncActionPostgresDecimalNumericSynonymsMatchSymmetrically
// in package xorm: postgres's own "numeric" -> "decimal" Alias mapping
// only silenced compareColumnTypes's base-name level when applying it to
// the expected side happened to line up with the actual side's raw,
// unaliased spelling. Before compareColumnTypes aliased both sides, a
// real postgres NUMERIC(10,2) column warned against a struct tagged
// "DECIMAL(19,4)" or "NUMERIC(19,4)" (a genuine precision mismatch
// postgres cannot ALTER away during Sync, but one that the analogous
// "DECIMAL(19,4)" pair already stayed silent for - see
// dialects.TestCompareColumnsType's "type aliases still match" case), and
// a bare NUMERIC column warned against every sized tag spelling, while
// the same pairs against a "DECIMAL(10,2)" or exactly-matching
// "NUMERIC(10,2)" tag stayed silent. All eight combinations must resolve
// the same way (silent) regardless of which synonym spelling was used.
func TestSyncWarningPostgresNumericDecimalSynonymsMatchSymmetrically(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	tests := []struct {
		name      string
		tableName string
		columnDDL string
		sync      func(tableName string) error
	}{
		{"decimal_10_2_vs_sized_10_2", "swp_num_sym_1", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresDecimal1010_2))
		}},
		{"numeric_10_2_vs_sized_10_2", "swp_num_sym_2", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresNumeric1010_2))
		}},
		{"decimal_19_4_vs_sized_10_2", "swp_num_sym_3", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresDecimal19_4))
		}},
		{"numeric_19_4_vs_sized_10_2", "swp_num_sym_4", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresNumeric19_4))
		}},
		{"decimal_10_2_vs_bare", "swp_num_sym_5", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresDecimal1010_2))
		}},
		{"numeric_10_2_vs_bare", "swp_num_sym_6", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresNumeric1010_2))
		}},
		{"decimal_19_4_vs_bare", "swp_num_sym_7", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresDecimal19_4))
		}},
		{"numeric_19_4_vs_bare", "swp_num_sym_8", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(syncWarningPostgresNumeric19_4))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := tt.tableName
			dropTestTable(tableName)
			defer dropTestTable(tableName)

			_, err := testEngine.Exec(fmt.Sprintf(
				"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price %s NOT NULL)",
				qualifiedTableName(tableName), tt.columnDDL))
			assert.NoError(t, err)
			assertTableVisible(t, tableName)

			before := columnFromDBMetas(t, tableName, "price")

			recorder := captureWarnings(t)
			assert.NoError(t, tt.sync(tableName))
			assert.False(t, recorder.hasMessageContaining("column price db type is"),
				"expected no warning about price, got: %v", recorder.messages())

			after := columnFromDBMetas(t, tableName, "price")
			assert.EqualValues(t, before.Length, after.Length,
				"expected Sync to leave the column's precision unchanged regardless of synonym spelling or declared precision")
			assert.EqualValues(t, before.Length2, after.Length2,
				"expected Sync to leave the column's scale unchanged regardless of synonym spelling or declared precision")
		})
	}
}

// TestSyncWarningPostgresNumericCommentSyncDoesNotNarrowColumn is the
// postgres counterpart of TestSyncWarningMySQLDecimalCommentDoesNotNarrowColumn,
// pinning the interaction between this change's level-4 aliasing fix and
// xorm/xorm#2591's comment gate: once compareColumnTypes' base-name level
// aliases both sides, a real postgres NUMERIC(19,4) column against a
// struct tagged DECIMAL(10,2) is ColumnCompareEquivalent, not Different,
// so it falls through buildColumnSyncDecision's first two switch cases to
// the comment case - and resolveCommentSyncDecision must refuse the
// comment sync there, or applyColumnSyncDecision's ModifyColumnSQL call
// would narrow the column and round the stored value as a side effect of
// "just" syncing the comment.
func TestSyncWarningPostgresNumericCommentSyncDoesNotNarrowColumn(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_postgres_numeric_comment_no_narrow"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price NUMERIC(19,4))", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	_, err = testEngine.Exec(fmt.Sprintf(
		"INSERT INTO %s (id, price) VALUES (1, 1.2345)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	_, err = testEngine.Exec(fmt.Sprintf(
		"COMMENT ON COLUMN %s.price IS 'old comment'", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 19, before.Length, "sanity: numeric(19,4) column should read back with precision 19")
	assert.EqualValues(t, 4, before.Length2, "sanity: numeric(19,4) column should read back with scale 4")

	type SyncWarningPostgresNumericCommentNoNarrow struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) comment('new comment')"`
	}

	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningPostgresNumericCommentNoNarrow)))

	after := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 19, after.Length,
		"expected Sync to leave the numeric(19,4) column's precision unchanged even though its comment differs from the struct tag")
	assert.EqualValues(t, 4, after.Length2,
		"expected Sync to leave the numeric(19,4) column's scale unchanged even though its comment differs from the struct tag")

	var price float64
	has, err := testEngine.Table(tableName).Cols("price").Where("id = ?", 1).Get(&price)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.InDelta(t, 1.2345, price, 0.00001,
		"expected the stored value to survive Sync without being rounded to the struct tag's narrower scale")
}

// TestSyncWarningVarcharShrinkWithCommentDoesNotNarrowColumn is the P1
// composition regression from review of xorm/xorm#2588/#2589/#2591: a
// database VARCHAR(255) column against a struct wanting VARCHAR(64), with
// a differing comment, is ColumnCompareEquivalent (base sql type name),
// never Equal, since the rendered lengths differ. Before
// resolveCommentSyncDecision's gate, a differing comment on this shape
// reached applyColumnSyncDecision's modifyComment branch, which reissues
// the struct's full expected column definition through ModifyColumnSQL -
// "ALTER TABLE ... MODIFY lower_name VARCHAR(64) COMMENT 'new comment'" on
// mysql - silently narrowing the column and truncating any stored value
// too long to fit.
func TestSyncWarningVarcharShrinkWithCommentDoesNotNarrowColumn(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL, schemas.POSTGRES:
	default:
		t.Skip("mysql/postgres only")
	}

	const tableName = "sync_warning_varchar_shrink_comment_no_narrow"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	longValue := strings.Repeat("x", 200)
	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR(255) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	_, err = testEngine.Exec(fmt.Sprintf(
		"INSERT INTO %s (id, lower_name) VALUES (1, '%s')", qualifiedTableName(tableName), longValue))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningVarcharShrinkWithComment struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(64) comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningVarcharShrinkWithComment)))
	assert.True(t, recorder.hasMessageContaining("comment not synced"),
		"expected a comment-sync-skipped notice about lower_name, got: %v", recorder.messages())
	assert.False(t, recorder.warnfHasMessageContaining("comment not synced"),
		"expected the comment-sync-skipped notice to log at Infof, not Warnf: got Warnf messages %v", recorder.warnfMessages())

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 255, col.Length,
		"expected lower_name to stay VARCHAR(255), a shrink triggered by comment sync must not be applied")

	var storedValue string
	has, err := testEngine.Table(tableName).Cols("lower_name").Where("id = ?", 1).Get(&storedValue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, longValue, storedValue,
		"expected the stored value to survive Sync without being truncated to the struct tag's narrower length")
}

// TestSyncWarningCommentSyncsWhenTypesMatchExactly is the positive
// counterpart to the comment-skip tests above: resolveCommentSyncDecision
// must still sync a differing comment when the rendered types are
// byte-identical (ColumnCompareEqual), since that is the shape the gate
// is designed to keep working, not to disable comment sync entirely.
func TestSyncWarningCommentSyncsWhenTypesMatchExactly(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL, schemas.POSTGRES:
	default:
		t.Skip("mysql/postgres only")
	}

	const tableName = "sync_warning_comment_syncs_when_types_match"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR(100) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "lower_name")
	assert.Equal(t, "", before.Comment, "sanity: freshly created column should have no comment")

	type SyncWarningCommentSyncsWhenTypesMatch struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(100) NOT NULL comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningCommentSyncsWhenTypesMatch)))
	assert.False(t, recorder.hasMessageContaining("comment not synced"),
		"expected no comment-sync-skipped notice, got: %v", recorder.messages())

	after := columnFromDBMetas(t, tableName, "lower_name")
	assert.Equal(t, "new comment", after.Comment,
		"expected the comment to sync when the rendered types are byte-identical")
}

// TestSyncWarningMariaDBIntDisplayWidthCommentSkipped pins the first of
// the three real costs documented on resolveCommentSyncDecision: MariaDB
// 10.6 (and MySQL <= 8.0.18) report a plain INT column as "int(11)", so a
// bare `xorm:"INT"` struct type renders without a width and the pair is
// ColumnCompareEquivalent ("normalized sql type", level 3 strips the
// display width), never ColumnCompareEqual - on tables xorm itself
// created, this disables comment sync for every int-family column. The
// test skips itself on a MySQL version that does not reproduce the
// display-width read-back (MySQL 8.0.19+), so it is a live pin rather than
// an assumption. See TestSyncWarningMySQLTextDisplayLengthCommentSkipped
// and TestSyncWarningPostgresDecimalNumericSpellingCommentSkipped for the
// other two documented shapes.
func TestSyncWarningMariaDBIntDisplayWidthCommentSkipped(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MYSQL {
		t.Skip("mysql/mariadb only")
	}

	const tableName = "sync_warning_mariadb_int_width_comment_skipped"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, num_watches INT NOT NULL DEFAULT 0 COMMENT 'old comment')",
		qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "num_watches")
	if before.Length == 0 {
		t.Skip("this MySQL version does not read a bare INT column back with a display width")
	}

	type SyncWarningMariaDBIntWidthCommentSkipped struct {
		Id         int64 `xorm:"pk"`
		NumWatches int32 `xorm:"NOT NULL DEFAULT 0 comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMariaDBIntWidthCommentSkipped)))
	assert.True(t, recorder.hasMessageContaining("comment not synced because db type is"),
		"expected a comment-sync-skipped notice about num_watches, got: %v", recorder.messages())
	assert.False(t, recorder.warnfHasMessageContaining("comment not synced"),
		"expected the comment-sync-skipped notice to log at Infof, not Warnf: got Warnf messages %v", recorder.warnfMessages())

	after := columnFromDBMetas(t, tableName, "num_watches")
	assert.Equal(t, "old comment", after.Comment,
		"expected the comment to stay unsynced: the display-width read-back keeps this pair Equivalent, never Equal")
}

// TestSyncWarningMySQLTextDisplayLengthCommentSkipped pins the second of
// the three real costs documented on resolveCommentSyncDecision: on every
// MySQL 8.0 version (not just old ones - the display-width class above is
// version-dependent, this one is not), a bare `xorm:"TEXT"` column reads
// back as "text(65535)" (CHARACTER_MAXIMUM_LENGTH is populated for TEXT),
// while the struct's bare TEXT tag never renders a length. The pair is
// ColumnCompareEquivalent (level 3, normalized sql type strips the
// length for a TEXT/BLOB family type), never ColumnCompareEqual, so every
// commented TEXT/BLOB-family column stops syncing its comment.
func TestSyncWarningMySQLTextDisplayLengthCommentSkipped(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MYSQL {
		t.Skip("mysql/mariadb only")
	}

	const tableName = "sync_warning_mysql_text_length_comment_skipped"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, body TEXT NOT NULL COMMENT 'old comment')",
		qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "body")
	if before.Length == 0 {
		t.Skip("this MySQL version does not read a bare TEXT column back with a display length")
	}

	type SyncWarningMySQLTextLengthCommentSkipped struct {
		Id   int64  `xorm:"pk"`
		Body string `xorm:"TEXT NOT NULL comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMySQLTextLengthCommentSkipped)))
	assert.True(t, recorder.hasMessageContaining("comment not synced because db type is"),
		"expected a comment-sync-skipped notice about body, got: %v", recorder.messages())
	assert.False(t, recorder.warnfHasMessageContaining("comment not synced"),
		"expected the comment-sync-skipped notice to log at Infof, not Warnf: got Warnf messages %v", recorder.warnfMessages())

	after := columnFromDBMetas(t, tableName, "body")
	assert.Equal(t, "old comment", after.Comment,
		"expected the comment to stay unsynced: the display-length read-back keeps this pair Equivalent, never Equal")
}

// TestSyncWarningPostgresDecimalNumericSpellingCommentSkipped pins the
// third of the three real costs documented on resolveCommentSyncDecision:
// postgres's own catalog normalizes a column declared DECIMAL(p,s) to
// NUMERIC(p,s) (see TestSyncWarningPostgresNumericPrecisionReadBack),
// while a struct tagged `xorm:"DECIMAL(10,2)"` renders "DECIMAL(10,2)".
// The pair is ColumnCompareEquivalent (base sql type name - postgres
// aliases "numeric" to "decimal"), never ColumnCompareEqual, so every
// commented DECIMAL/NUMERIC column stops syncing its comment, regardless
// of MySQL version quirks - this is a postgres catalog behavior, not a
// display-width read-back artifact.
func TestSyncWarningPostgresDecimalNumericSpellingCommentSkipped(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_postgres_decimal_numeric_comment_skipped"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price DECIMAL(10,2) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	_, err = testEngine.Exec(fmt.Sprintf(
		"COMMENT ON COLUMN %s.price IS 'old comment'", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "price")
	assert.Equal(t, schemas.Numeric, before.SQLType.Name,
		"sanity: postgres should read a DECIMAL(10,2) column back as NUMERIC")

	type SyncWarningPostgresDecimalNumericSpellingCommentSkipped struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningPostgresDecimalNumericSpellingCommentSkipped)))
	assert.True(t, recorder.hasMessageContaining("comment not synced because db type is"),
		"expected a comment-sync-skipped notice about price, got: %v", recorder.messages())
	assert.False(t, recorder.warnfHasMessageContaining("comment not synced"),
		"expected the comment-sync-skipped notice to log at Infof, not Warnf: got Warnf messages %v", recorder.warnfMessages())

	after := columnFromDBMetas(t, tableName, "price")
	assert.Equal(t, "old comment", after.Comment,
		"expected the comment to stay unsynced: the NUMERIC/DECIMAL spelling keeps this pair Equivalent, never Equal")
}

// TestSyncWarningVarcharLengthWidened is the xorm/xorm#2588 fix: a
// database VARCHAR(64) column against a struct wanting VARCHAR(255) must
// widen to 255 on mysql/postgres, with an Infof announcement but no Warnf.
// warningRecorder captures both Infof and Warnf calls, so the assertion
// below is scoped to the "db type is" text Warnf uses rather than the
// column name, which would also match the expected Infof announcement.
func TestSyncWarningVarcharLengthWidened(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL, schemas.POSTGRES:
	default:
		t.Skip("mysql/postgres only")
	}

	const tableName = "sync_warning_varchar_length_widened"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR(64) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningVarcharLengthWidened struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(255) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningVarcharLengthWidened)))
	assert.False(t, recorder.hasMessageContaining("db type is"),
		"expected no type-mismatch warning about lower_name, got: %v", recorder.messages())

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 255, col.Length,
		"expected lower_name to widen from VARCHAR(64) to VARCHAR(255)")
}

// TestSyncWarningVarcharLengthShrinkNotApplied is the shrink counterpart of
// TestSyncWarningVarcharLengthWidened: a database VARCHAR(255) column
// against a struct wanting VARCHAR(64) must not warn and must not be
// altered, even on dialects that support widening. Only a database column
// shorter than the struct wants is ever a candidate for
// columnTypeSyncActionModifyVarcharExpand (see resolveVarcharWidenSyncAction
// in sync.go).
func TestSyncWarningVarcharLengthShrinkNotApplied(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL, schemas.POSTGRES:
	default:
		t.Skip("mysql/postgres only")
	}

	const tableName = "sync_warning_varchar_length_shrink_not_applied"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR(255) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningVarcharLengthShrinkNotApplied struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(64) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningVarcharLengthShrinkNotApplied)))
	assert.False(t, recorder.hasMessageContaining("lower_name"),
		"expected no warning about lower_name, got: %v", recorder.messages())

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 255, col.Length,
		"expected lower_name to stay VARCHAR(255), a shrink must not be applied")
}

// TestSyncWarningPostgresUnboundedVarcharNotWidened is the xorm/xorm#2588
// review fix: postgres reports an unbounded "character varying" column
// (declared without a length) with Length == 0, and 0 < 255 reads as
// "shorter" unless resolveVarcharWidenSyncAction explicitly excludes a
// zero actual length. Without that exclusion, Sync would run
// `ALTER TABLE ... TYPE varchar(255)`, which narrows an unbounded column -
// a silent capacity reduction at best, and a failing ALTER (or a runtime
// "value too long" error on the next oversized INSERT/UPDATE) at worst.
// Bare "VARCHAR" is not legal DDL on mysql/mariadb (VARCHAR requires a
// length there), so this shape is postgres-only.
func TestSyncWarningPostgresUnboundedVarcharNotWidened(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.POSTGRES {
		t.Skip("postgres only")
	}

	const tableName = "sync_warning_postgres_unbounded_varchar_not_widened"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 0, col.Length,
		"expected GetColumns to read an unbounded character varying column back as Length 0")

	type SyncWarningPostgresUnboundedVarchar struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(255) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningPostgresUnboundedVarchar)))
	assert.False(t, recorder.hasMessageContaining("lower_name"),
		"expected no message about lower_name, got: %v", recorder.messages())

	col = columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 0, col.Length,
		"expected lower_name to stay unbounded, not be narrowed to varchar(255)")
}

// TestSyncWarningVarcharWidenTakesPriorityOverCommentSync combines the
// xorm/xorm#2588 widen fix with the xorm/xorm#2591 comment gate:
// applyColumnSyncDecision runs at most one ModifyColumnSQL per column, and
// that single call renders the struct's entire expected column
// definition, comment included - so a widen and a differing comment
// resolve in the same statement without ever going through
// resolveCommentSyncDecision (decision.modifyComment stays false, and so
// does commentSkippedTypeMismatch: there was nothing to skip, since no
// separate comment sync was attempted).
func TestSyncWarningVarcharWidenTakesPriorityOverCommentSync(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	switch testEngine.Dialect().URI().DBType {
	case schemas.MYSQL, schemas.POSTGRES:
	default:
		t.Skip("mysql/postgres only")
	}

	const tableName = "sync_warning_varchar_widen_with_comment"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, lower_name VARCHAR(64) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	type SyncWarningVarcharWidenWithComment struct {
		Id        int64  `xorm:"pk"`
		LowerName string `xorm:"VARCHAR(255) NOT NULL comment('new comment')"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningVarcharWidenWithComment)))
	assert.False(t, recorder.hasMessageContaining("comment not synced"),
		"expected no comment-sync-skipped warning, got: %v", recorder.messages())

	col := columnFromDBMetas(t, tableName, "lower_name")
	assert.EqualValues(t, 255, col.Length,
		"expected lower_name to widen from VARCHAR(64) to VARCHAR(255)")
	assert.Equal(t, "new comment", col.Comment,
		"expected the comment to sync as part of the widen's own ModifyColumnSQL, which renders the full expected column")
}

// TestSyncWarningMSSQLNumericPrecisionReadBack is the xorm/xorm#2589
// fix: GetColumns special-cased ct == "DECIMAL" when populating
// Length/Length2 from sys.columns' precision/scale; a real NUMERIC(10,2)
// column fell through to the generic max_length byte-count path instead,
// so it read back as NUMERIC(9) with the scale lost, and Sync warned
// forever against a struct field tagged DECIMAL(10,2).
func TestSyncWarningMSSQLNumericPrecisionReadBack(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MSSQL {
		t.Skip("mssql only")
	}

	const tableName = "sync_warning_mssql_numeric_precision_readback"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price NUMERIC(10,2) NOT NULL)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 10, before.Length, "sanity: numeric(10,2) column should read back with precision 10")
	assert.EqualValues(t, 2, before.Length2, "sanity: numeric(10,2) column should read back with scale 2")

	type SyncWarningMSSQLNumericPrecisionReadBack struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) NOT NULL"`
	}

	recorder := captureWarnings(t)
	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMSSQLNumericPrecisionReadBack)))
	assert.False(t, recorder.hasMessageContaining("column price db type is"),
		"expected no warning about price, got: %v", recorder.messages())
}

// TestSyncWarningMSSQLNumericDecimalSynonymsMatchSymmetrically is the
// live-database counterpart of
// TestResolveColumnTypeSyncActionPostgresDecimalNumericSynonymsMatchSymmetrically,
// on mssql: DECIMAL and NUMERIC are exact synonyms in SQL Server, and all
// eight combinations of struct tag spelling/precision against a sized or
// bare NUMERIC column must resolve the same way (silent), regardless of
// which synonym spelling was used.
func TestSyncWarningMSSQLNumericDecimalSynonymsMatchSymmetrically(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MSSQL {
		t.Skip("mssql only")
	}

	type mssqlDecimal1010_2 struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) NOT NULL"`
	}
	type mssqlNumeric1010_2 struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"NUMERIC(10,2) NOT NULL"`
	}
	type mssqlDecimal19_4 struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(19,4) NOT NULL"`
	}
	type mssqlNumeric19_4 struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"NUMERIC(19,4) NOT NULL"`
	}

	tests := []struct {
		name      string
		tableName string
		columnDDL string
		sync      func(tableName string) error
	}{
		{"decimal_10_2_vs_sized_10_2", "swm_num_sym_1", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlDecimal1010_2))
		}},
		{"numeric_10_2_vs_sized_10_2", "swm_num_sym_2", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlNumeric1010_2))
		}},
		{"decimal_19_4_vs_sized_10_2", "swm_num_sym_3", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlDecimal19_4))
		}},
		{"numeric_19_4_vs_sized_10_2", "swm_num_sym_4", "NUMERIC(10,2)", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlNumeric19_4))
		}},
		{"decimal_10_2_vs_bare", "swm_num_sym_5", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlDecimal1010_2))
		}},
		{"numeric_10_2_vs_bare", "swm_num_sym_6", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlNumeric1010_2))
		}},
		{"decimal_19_4_vs_bare", "swm_num_sym_7", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlDecimal19_4))
		}},
		{"numeric_19_4_vs_bare", "swm_num_sym_8", "NUMERIC", func(tableName string) error {
			return testEngine.Table(tableName).Sync(new(mssqlNumeric19_4))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := tt.tableName
			dropTestTable(tableName)
			defer dropTestTable(tableName)

			_, err := testEngine.Exec(fmt.Sprintf(
				"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price %s NOT NULL)",
				qualifiedTableName(tableName), tt.columnDDL))
			assert.NoError(t, err)
			assertTableVisible(t, tableName)

			before := columnFromDBMetas(t, tableName, "price")

			recorder := captureWarnings(t)
			assert.NoError(t, tt.sync(tableName))
			assert.False(t, recorder.hasMessageContaining("column price db type is"),
				"expected no warning about price, got: %v", recorder.messages())

			after := columnFromDBMetas(t, tableName, "price")
			assert.EqualValues(t, before.Length, after.Length,
				"expected Sync to leave the column's precision unchanged regardless of synonym spelling or declared precision")
			assert.EqualValues(t, before.Length2, after.Length2,
				"expected Sync to leave the column's scale unchanged regardless of synonym spelling or declared precision")
		})
	}
}

// TestSyncWarningMSSQLNumericCommentSyncDoesNotNarrowColumn is the mssql
// counterpart of TestSyncWarningMySQLDecimalCommentDoesNotNarrowColumn: a
// real NUMERIC(19,4) column against a struct tagged DECIMAL(10,2), which
// compareColumnTypes' base-name level now (correctly) treats as the same
// base type after this change, is ColumnCompareEquivalent, not Equal, so
// resolveCommentSyncDecision (xorm/xorm#2591) must refuse the comment
// sync rather than let ModifyColumnSQL narrow the column. mssql's own
// ColumnSyncFeatures never enables ColumnComment, so this is a defensive
// pin rather than a reachable-in-production shape, and the column has no
// comment to compare against in the first place - the assertion that
// matters here is that Sync leaves precision, scale, and the stored value
// untouched.
func TestSyncWarningMSSQLNumericCommentSyncDoesNotNarrowColumn(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.MSSQL {
		t.Skip("mssql only")
	}

	const tableName = "sync_warning_mssql_numeric_comment_no_narrow"
	dropTestTable(tableName)
	defer dropTestTable(tableName)

	_, err := testEngine.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id BIGINT NOT NULL PRIMARY KEY, price NUMERIC(19,4))", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	_, err = testEngine.Exec(fmt.Sprintf(
		"INSERT INTO %s (id, price) VALUES (1, 1.2345)", qualifiedTableName(tableName)))
	assert.NoError(t, err)
	assertTableVisible(t, tableName)

	before := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 19, before.Length, "sanity: numeric(19,4) column should read back with precision 19")
	assert.EqualValues(t, 4, before.Length2, "sanity: numeric(19,4) column should read back with scale 4")

	type SyncWarningMSSQLNumericCommentNoNarrow struct {
		Id    int64   `xorm:"pk"`
		Price float64 `xorm:"DECIMAL(10,2) comment('new comment')"`
	}

	assert.NoError(t, testEngine.Table(tableName).Sync(new(SyncWarningMSSQLNumericCommentNoNarrow)))

	after := columnFromDBMetas(t, tableName, "price")
	assert.EqualValues(t, 19, after.Length,
		"expected Sync to leave the numeric(19,4) column's precision unchanged even though its comment differs from the struct tag")
	assert.EqualValues(t, 4, after.Length2,
		"expected Sync to leave the numeric(19,4) column's scale unchanged even though its comment differs from the struct tag")

	var price float64
	has, err := testEngine.Table(tableName).Cols("price").Where("id = ?", 1).Get(&price)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.InDelta(t, 1.2345, price, 0.00001,
		"expected the stored value to survive Sync without being rounded to the struct tag's narrower scale")
}
