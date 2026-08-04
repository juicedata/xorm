package dialects

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"xorm.io/xorm/schemas"
)

func TestParsePostgres(t *testing.T) {
	tests := []struct {
		in       string
		expected string
		valid    bool
	}{
		{"postgres://auser:password@localhost:5432/db?sslmode=disable", "db", true},
		{"postgresql://auser:password@localhost:5432/db?sslmode=disable", "db", true},
		{"postg://auser:password@localhost:5432/db?sslmode=disable", "db", false},
		// {"postgres://auser:pass with space@localhost:5432/db?sslmode=disable", "db", true},
		// {"postgres:// auser : password@localhost:5432/db?sslmode=disable", "db", true},
		{"postgres://%20auser%20:pass%20with%20space@localhost:5432/db?sslmode=disable", "db", true},
		// {"postgres://auser:パスワード@localhost:5432/データベース?sslmode=disable", "データベース", true},
		{"dbname=db sslmode=disable", "db", true},
		{"user=auser password=password dbname=db sslmode=disable", "db", true},
		{"user=auser password='pass word' dbname=db sslmode=disable", "db", true},
		{"user=auser password='pass word' sslmode=disable dbname='db'", "db", true},
		{"user=auser password='pass word' sslmode='disable dbname=db'", "db", false},
		{"", "db", false},
		{"dbname=db =disable", "db", false},
	}

	driver := QueryDriver("postgres")
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			uri, err := driver.Parse("postgres", test.in)

			if err != nil && test.valid {
				t.Errorf("%q got unexpected error: %s", test.in, err)
			} else if err == nil && !reflect.DeepEqual(test.expected, uri.DBName) {
				t.Errorf("%q got: %#v want: %#v", test.in, uri.DBName, test.expected)
			}
		})
	}
}

func TestParsePgx(t *testing.T) {
	tests := []struct {
		in       string
		expected string
		valid    bool
	}{
		{"postgres://auser:password@localhost:5432/db?sslmode=disable", "db", true},
		{"postgresql://auser:password@localhost:5432/db?sslmode=disable", "db", true},
		{"postg://auser:password@localhost:5432/db?sslmode=disable", "db", false},
		// {"postgres://auser:pass with space@localhost:5432/db?sslmode=disable", "db", true},
		// {"postgres:// auser : password@localhost:5432/db?sslmode=disable", "db", true},
		{"postgres://%20auser%20:pass%20with%20space@localhost:5432/db?sslmode=disable", "db", true},
		// {"postgres://auser:パスワード@localhost:5432/データベース?sslmode=disable", "データベース", true},
		{"dbname=db sslmode=disable", "db", true},
		{"user=auser password=password dbname=db sslmode=disable", "db", true},
		{"", "db", false},
		{"dbname=db =disable", "db", false},
	}

	driver := QueryDriver("pgx")

	for _, test := range tests {
		uri, err := driver.Parse("pgx", test.in)

		if err != nil && test.valid {
			t.Errorf("%q got unexpected error: %s", test.in, err)
		} else if err == nil && !reflect.DeepEqual(test.expected, uri.DBName) {
			t.Errorf("%q got: %#v want: %#v", test.in, uri.DBName, test.expected)
		}

		// Register DriverConfig
		uri, err = driver.Parse("pgx", test.in)
		if err != nil && test.valid {
			t.Errorf("%q got unexpected error: %s", test.in, err)
		} else if err == nil && !reflect.DeepEqual(test.expected, uri.DBName) {
			t.Errorf("%q got: %#v want: %#v", test.in, uri.DBName, test.expected)
		}
	}
}

func TestGetIndexColName(t *testing.T) {
	t.Run("Index", func(t *testing.T) {
		s := "CREATE INDEX test2_mm_idx ON test2 (major);"
		colNames := getIndexColName(s)
		assert.Equal(t, []string{"major"}, colNames)
	})

	t.Run("Multicolumn indexes", func(t *testing.T) {
		s := "CREATE INDEX test2_mm_idx ON test2 (major, minor);"
		colNames := getIndexColName(s)
		assert.Equal(t, []string{"major", "minor"}, colNames)
	})

	t.Run("Indexes and ORDER BY", func(t *testing.T) {
		s := "CREATE INDEX test2_mm_idx ON test2 (major  NULLS FIRST, minor DESC NULLS LAST);"
		colNames := getIndexColName(s)
		assert.Equal(t, []string{"major", "minor"}, colNames)
	})

	t.Run("Combining Multiple Indexes", func(t *testing.T) {
		s := "CREATE INDEX test2_mm_cm_idx ON public.test2 USING btree (major, minor) WHERE ((major <> 5) AND (minor <> 6))"
		colNames := getIndexColName(s)
		assert.Equal(t, []string{"major", "minor"}, colNames)
	})

	t.Run("unique", func(t *testing.T) {
		s := "CREATE UNIQUE INDEX test2_mm_uidx ON test2 (major);"
		colNames := getIndexColName(s)
		assert.Equal(t, []string{"major"}, colNames)
	})

	t.Run("Indexes on Expressions", func(t *testing.T) {})
}

// TestPostgresModifyColumnSQLAutoIncrementUsesConcreteType pins
// xorm/xorm#2594: SQLType renders a serial pseudo-type (BIGSERIAL/SERIAL)
// for an autoincrement column, since that is the correct spelling for
// CREATE TABLE, but those are pseudo-types PostgreSQL rejects in ALTER
// TABLE ... ALTER COLUMN ... TYPE ("type \"bigserial\" does not exist").
// ModifyColumnSQL must render the concrete underlying type instead - by
// mapping the rendered pseudo-type name, not by re-deriving the type from
// a modified column. The "smallint falls through to SERIAL" case pins the
// regression review found in the first version of this fix: re-deriving
// from a column copy with IsAutoIncrement cleared rendered "SMALLINT" for
// that shape, silently narrowing a real integer-backed serial column.
func TestPostgresModifyColumnSQLAutoIncrementUsesConcreteType(t *testing.T) {
	dialect, err := OpenDialect("postgres", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	assert.NoError(t, err)

	tests := []struct {
		name        string
		sqlTypeName string
		wantModify  string
		wantCreate  string
	}{
		{"implicit bigint autoincrement", schemas.BigInt, "BIGINT", "BIGSERIAL"},
		{"implicit integer autoincrement", schemas.Integer, "INTEGER", "SERIAL"},
		{"explicit BIGSERIAL type tag", schemas.BigSerial, "BIGINT", "BIGSERIAL"},
		{"explicit SERIAL type tag", schemas.Serial, "INTEGER", "SERIAL"},
		{"smallint autoincrement falls through to SERIAL, must not narrow to SMALLINT", schemas.SmallInt, "INTEGER", "SERIAL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			col := &schemas.Column{
				Name:            "id",
				SQLType:         schemas.SQLType{Name: test.sqlTypeName},
				IsAutoIncrement: true,
				IsPrimaryKey:    true,
				Comment:         "new comment",
			}

			assert.Equal(t, test.wantCreate, dialect.SQLType(col),
				"sanity: SQLType must still render the pseudo-type for CREATE TABLE")

			modifySQL := dialect.ModifyColumnSQL("t", col)
			assert.Contains(t, modifySQL, "TYPE "+test.wantModify,
				"expected ModifyColumnSQL to render the concrete type, got: %s", modifySQL)
			assert.NotContains(t, modifySQL, test.wantCreate,
				"expected ModifyColumnSQL to never render the CREATE-only pseudo-type, got: %s", modifySQL)
		})
	}
}

// TestPostgresModifyColumnSQLNonAutoincrementUnaffected pins that
// alterColumnPseudoTypes' substitution only ever fires on a serial
// pseudo-type name: a non-autoincrement column's ModifyColumnSQL output
// must be byte-identical to its SQLType output, so this change cannot
// alter behaviour for any column that was never affected by
// xorm/xorm#2594 in the first place.
func TestPostgresModifyColumnSQLNonAutoincrementUnaffected(t *testing.T) {
	dialect, err := OpenDialect("postgres", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	assert.NoError(t, err)

	col := &schemas.Column{
		Name:    "price",
		SQLType: schemas.SQLType{Name: schemas.Decimal},
		Length:  10,
		Length2: 2,
		Comment: "new comment",
	}

	sqlType := dialect.SQLType(col)
	assert.Equal(t, "DECIMAL(10,2)", sqlType, "sanity: precondition for this test")

	modifySQL := dialect.ModifyColumnSQL("t", col)
	assert.Contains(t, modifySQL, "TYPE "+sqlType,
		"expected ModifyColumnSQL to render exactly what SQLType produced for a non-autoincrement column, got: %s", modifySQL)
}

// TestPostgresModifyColumnCommentSQLHasNoTypeClause pins the comment-only
// DDL path xorm/xorm#2591's follow-up adds: ModifyColumnCommentSQL must
// render nothing but a "COMMENT ON COLUMN" statement, with no "ALTER
// TABLE" or "TYPE" clause at all, so Sync can never rewrite a column's
// type as a side effect of syncing only its comment - not even for an
// autoincrement column whose SQLType rendering (SERIAL/BIGSERIAL) would
// otherwise need alterColumnTypeSQL's substitution in ModifyColumnSQL.
func TestPostgresModifyColumnCommentSQLHasNoTypeClause(t *testing.T) {
	dialect, err := OpenDialect("postgres", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	assert.NoError(t, err)
	modifier, ok := dialect.(ColumnCommentModifier)
	assert.True(t, ok, "sanity: postgres must implement ColumnCommentModifier")

	col := &schemas.Column{
		Name:            "id",
		SQLType:         schemas.SQLType{Name: schemas.SmallInt},
		IsAutoIncrement: true,
		IsPrimaryKey:    true,
		Comment:         "new comment",
	}

	commentSQL := modifier.ModifyColumnCommentSQL("t", col)
	assert.Equal(t, `COMMENT ON COLUMN "public"."t"."id" IS 'new comment'`, commentSQL)
	assert.NotContains(t, commentSQL, "ALTER")
	assert.NotContains(t, commentSQL, "TYPE")

	fullRewriteSQL := dialect.ModifyColumnSQL("t", col)
	assert.Contains(t, fullRewriteSQL, "TYPE INTEGER",
		"sanity: the full-rewrite path for the same column does render a TYPE clause, unlike ModifyColumnCommentSQL")
}

// TestPostgresModifyColumnCommentSQLWithSchema pins that
// ModifyColumnCommentSQL qualifies the table with the configured schema,
// the same way ModifyColumnSQL's own embedded COMMENT ON COLUMN clause
// does.
func TestPostgresModifyColumnCommentSQLWithSchema(t *testing.T) {
	dialect, err := OpenDialect("postgres", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	assert.NoError(t, err)
	dialect.URI().SetSchema("myschema")
	modifier, ok := dialect.(ColumnCommentModifier)
	assert.True(t, ok, "sanity: postgres must implement ColumnCommentModifier")

	col := &schemas.Column{Name: "price", Comment: "new comment"}

	commentSQL := modifier.ModifyColumnCommentSQL("t", col)
	assert.Equal(t, `COMMENT ON COLUMN "myschema"."t"."price" IS 'new comment'`, commentSQL)
}
