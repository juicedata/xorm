// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"testing"

	"xorm.io/xorm/schemas"
)

func mustInitDialect(t *testing.T, dbType schemas.DBType) Dialect {
	t.Helper()

	dialect := QueryDialect(dbType)
	if dialect == nil {
		t.Fatalf("QueryDialect(%q) returned nil", dbType)
	}

	if err := dialect.Init(&URI{DBType: dbType}); err != nil {
		t.Fatalf("Init(%q) error = %v", dbType, err)
	}

	return dialect
}

func TestColumnDefaultsMatch(t *testing.T) {
	tests := []struct {
		name string
		col  *schemas.Column
		ori  *schemas.Column
		want bool
	}{
		{
			name: "bool keyword casing",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Bool},
				Default:        "TRUE",
				DefaultIsEmpty: false,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Bool},
				Default:        "true",
				DefaultIsEmpty: false,
			},
			want: true,
		},
		{
			name: "bool numeric form",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Boolean},
				Default:        "FALSE",
				DefaultIsEmpty: false,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Boolean},
				Default:        "0",
				DefaultIsEmpty: false,
			},
			want: true,
		},
		{
			name: "numeric quoted in database",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Default:        "-1",
				DefaultIsEmpty: false,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Default:        "'-1'",
				DefaultIsEmpty: false,
			},
			want: true,
		},
		{
			name: "version helper default without schema default",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Default:        "1",
				DefaultIsEmpty: true,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				DefaultIsEmpty: true,
			},
			want: true,
		},
		{
			name: "autoincrement ignores default diff",
			col: &schemas.Column{
				SQLType:         schemas.SQLType{Name: schemas.Int},
				Default:         "1",
				DefaultIsEmpty:  false,
				IsAutoIncrement: true,
			},
			ori: &schemas.Column{
				SQLType:         schemas.SQLType{Name: schemas.Int},
				Default:         "999",
				DefaultIsEmpty:  false,
				IsAutoIncrement: true,
			},
			want: true,
		},
		{
			name: "nullable null default equals implicit null",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.BigInt},
				Default:        "NULL",
				DefaultIsEmpty: false,
				Nullable:       true,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.BigInt},
				DefaultIsEmpty: true,
				Nullable:       true,
			},
			want: true,
		},
		{
			name: "quoted text null is not treated as sql null",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Varchar},
				Default:        "'NULL'",
				DefaultIsEmpty: false,
				Nullable:       true,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Varchar},
				DefaultIsEmpty: true,
				Nullable:       true,
			},
			want: false,
		},
		{
			name: "not null column keeps explicit null mismatch",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Default:        "NULL",
				DefaultIsEmpty: false,
				Nullable:       false,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				DefaultIsEmpty: true,
				Nullable:       false,
			},
			want: false,
		},
		{
			name: "empty string still differs from no default",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Varchar},
				Default:        "",
				DefaultIsEmpty: false,
				Nullable:       true,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Varchar},
				DefaultIsEmpty: true,
				Nullable:       true,
			},
			want: false,
		},
		{
			name: "real default still differs from no default",
			col: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Default:        "1",
				DefaultIsEmpty: false,
			},
			ori: &schemas.Column{
				SQLType:        schemas.SQLType{Name: schemas.Int},
				DefaultIsEmpty: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnDefaultsMatch(tt.col, tt.ori); got != tt.want {
				t.Fatalf("columnDefaultsMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeColumnDefaultValue(t *testing.T) {
	tests := []struct {
		name         string
		sqlType      schemas.SQLType
		defaultValue string
		want         string
	}{
		{
			name:         "bool keeps canonical true",
			sqlType:      schemas.SQLType{Name: schemas.Bool},
			defaultValue: "'TRUE'",
			want:         "true",
		},
		{
			name:         "bool keeps canonical false",
			sqlType:      schemas.SQLType{Name: schemas.Boolean},
			defaultValue: " 0 ",
			want:         "false",
		},
		{
			name:         "numeric strips quotes and scale",
			sqlType:      schemas.SQLType{Name: schemas.Decimal},
			defaultValue: "'1.00'",
			want:         "1",
		},
		{
			name:         "text keeps quoted literal",
			sqlType:      schemas.SQLType{Name: schemas.Varchar},
			defaultValue: "'NULL'",
			want:         "'NULL'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeColumnDefaultValue(tt.sqlType, tt.defaultValue); got != tt.want {
				t.Fatalf("normalizeColumnDefaultValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeColumnTypeForComparison(t *testing.T) {
	alias := func(columnType string) string {
		if columnType == "NUMERIC" {
			return "DECIMAL"
		}
		return columnType
	}

	tests := []struct {
		name       string
		columnType string
		want       string
	}{
		{
			name:       "mysql integer display width is ignored",
			columnType: "INT(10) UNSIGNED",
			want:       "INT UNSIGNED",
		},
		{
			name:       "signed integer keeps base type only",
			columnType: "BIGINT(20)",
			want:       "BIGINT",
		},
		{
			name:       "alias still applies after normalization",
			columnType: "NUMERIC(10,2)",
			want:       "DECIMAL(10,2)",
		},
		{
			name:       "non integer length stays significant",
			columnType: "VARCHAR(255)",
			want:       "VARCHAR(255)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeColumnTypeForComparison(alias, tt.columnType); got != tt.want {
				t.Fatalf("normalizeColumnTypeForComparison() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCompareColumnTypesLevels exercises the four fallback levels of
// compareColumnTypes directly, without a live dialect. Level 4 (base sql
// type name) is the #2583 regression: before that fix, a struct type and a
// db type that only differ in the presence of a length/precision suffix
// (e.g. "DECIMAL" vs "DECIMAL(10,2)") produced a false sync warning.
func TestCompareColumnTypesLevels(t *testing.T) {
	identity := func(columnType string) string {
		return columnType
	}

	tests := []struct {
		name         string
		alias        func(string) string
		expectedCol  *schemas.Column
		actualCol    *schemas.Column
		expectedType string
		actualType   string
		wantStatus   ColumnCompareStatus
		wantReason   string
	}{
		{
			name:         "equal types",
			alias:        identity,
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "VARCHAR(64)",
			actualType:   "VARCHAR(64)",
			wantStatus:   ColumnCompareEqual,
		},
		{
			name:         "json-tagged text matches native json column",
			alias:        identity,
			expectedCol:  &schemas.Column{IsJSON: true},
			actualCol:    &schemas.Column{},
			expectedType: "TEXT",
			actualType:   "JSON",
			wantStatus:   ColumnCompareEquivalent,
			wantReason:   "json semantic type",
		},
		{
			name:         "jsonb-tagged text matches native jsonb column",
			alias:        identity,
			expectedCol:  &schemas.Column{IsJSON: true, IsJSONB: true},
			actualCol:    &schemas.Column{},
			expectedType: "TEXT",
			actualType:   "JSONB",
			wantStatus:   ColumnCompareEquivalent,
			wantReason:   "json semantic type",
		},
		{
			name:         "plain text still differs from native json column",
			alias:        identity,
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "TEXT",
			actualType:   "JSON",
			wantStatus:   ColumnCompareDifferent,
		},
		{
			name:         "mysql unsigned display width warning is suppressed",
			alias:        identity,
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "INT UNSIGNED",
			actualType:   "INT(10) UNSIGNED",
			wantStatus:   ColumnCompareEquivalent,
			wantReason:   "normalized sql type",
		},
		{
			name:         "mysql signedness mismatch still warns",
			alias:        identity,
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "INT UNSIGNED",
			actualType:   "INT(10)",
			wantStatus:   ColumnCompareDifferent,
		},
		{
			// #2583 regression: base names match after stripping the
			// length/precision suffix, but normalizeColumnTypeForComparison
			// (level 3) does not strip it for non integer-display types, so
			// only level 4 (base sql type name) catches this.
			name:         "decimal without precision matches decimal with precision",
			alias:        identity,
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "DECIMAL",
			actualType:   "DECIMAL(10,2)",
			wantStatus:   ColumnCompareEquivalent,
			wantReason:   "base sql type name",
		},
		{
			name:         "char length change matches at base type name level",
			alias:        identity,
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "CHAR(10)",
			actualType:   "CHAR(20)",
			wantStatus:   ColumnCompareEquivalent,
			wantReason:   "base sql type name",
		},
		{
			// mysql/postgres alias "numeric" to "decimal"; when the struct
			// type renders without a length suffix ("NUMERIC") and the db
			// type renders with one ("DECIMAL(10,2)"), level 3 fails because
			// of the differing suffix, but level 4 still matches because
			// EqualFold ignores the alias's lowercase spelling.
			name: "mysql alias pair matches at base type name level",
			alias: func(columnType string) string {
				if columnType == "NUMERIC" {
					return "decimal"
				}
				return columnType
			},
			expectedCol:  &schemas.Column{},
			actualCol:    &schemas.Column{},
			expectedType: "NUMERIC",
			actualType:   "DECIMAL(10,2)",
			wantStatus:   ColumnCompareEquivalent,
			wantReason:   "base sql type name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := compareColumnTypes(tt.alias, tt.expectedCol, tt.actualCol, tt.expectedType, tt.actualType)
			if field.Status != tt.wantStatus {
				t.Fatalf("compareColumnTypes().Status = %v, want %v", field.Status, tt.wantStatus)
			}
			if tt.wantReason != "" && field.Reason != tt.wantReason {
				t.Fatalf("compareColumnTypes().Reason = %q, want %q", field.Reason, tt.wantReason)
			}
		})
	}
}

func TestCompareColumnsType(t *testing.T) {
	mysqlDialect := mustInitDialect(t, schemas.MYSQL)
	postgresDialect := mustInitDialect(t, schemas.POSTGRES)

	tests := []struct {
		name    string
		dialect Dialect
		expect  *schemas.Column
		actual  *schemas.Column
		want    ColumnCompareStatus
	}{
		{
			name:    "mysql unsigned display width warning is suppressed",
			dialect: mysqlDialect,
			expect:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.UnsignedInt}},
			actual:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.UnsignedInt}, Length: 10},
			want:    ColumnCompareEquivalent,
		},
		{
			name:    "mysql signedness mismatch still warns",
			dialect: mysqlDialect,
			expect:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.UnsignedInt}},
			actual:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Int}, Length: 10},
			want:    ColumnCompareDifferent,
		},
		{
			name:    "type aliases still match",
			dialect: mysqlDialect,
			expect:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Decimal}, Length: 10, Length2: 2},
			actual:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Numeric}, Length: 10, Length2: 2},
			want:    ColumnCompareEquivalent,
		},
		{
			name:    "json-tagged text matches native json column",
			dialect: postgresDialect,
			expect:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Text}, IsJSON: true},
			actual:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Json}},
			want:    ColumnCompareEquivalent,
		},
		{
			name:    "jsonb-tagged text matches native jsonb column",
			dialect: postgresDialect,
			expect:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Text}, IsJSON: true, IsJSONB: true},
			actual:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Jsonb}},
			want:    ColumnCompareEquivalent,
		},
		{
			name:    "plain text still differs from native json column",
			dialect: postgresDialect,
			expect:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Text}},
			actual:  &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Json}},
			want:    ColumnCompareDifferent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comparison := tt.dialect.CompareColumns(tt.expect, tt.actual)
			if comparison.Type.Status != tt.want {
				t.Fatalf("CompareColumns().Type.Status = %v, want %v", comparison.Type.Status, tt.want)
			}
		})
	}
}

func TestCompareColumnsDoesNotMutateInputs(t *testing.T) {
	dialect := mustInitDialect(t, schemas.MYSQL)
	expect := &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Uuid}}
	actual := &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Varchar}, Length: 36}

	beforeExpectLength, beforeActualLength := expect.Length, actual.Length
	beforeExpectNullable, beforeActualNullable := expect.Nullable, actual.Nullable

	_ = dialect.CompareColumns(expect, actual)

	if expect.Length != beforeExpectLength || expect.Nullable != beforeExpectNullable {
		t.Fatalf("CompareColumns mutated expected column: got Length=%d Nullable=%v, want Length=%d Nullable=%v",
			expect.Length, expect.Nullable, beforeExpectLength, beforeExpectNullable)
	}
	if actual.Length != beforeActualLength || actual.Nullable != beforeActualNullable {
		t.Fatalf("CompareColumns mutated actual column: got Length=%d Nullable=%v, want Length=%d Nullable=%v",
			actual.Length, actual.Nullable, beforeActualLength, beforeActualNullable)
	}
}

// TestCompareColumnsNormalizesMySQLUuidLength is a regression guard for the
// normalized-clone requirement: a mysql Uuid struct column has Length == 0
// until SQLType renders it as varchar(40) as a side effect. CompareColumns
// must expose that normalized length via ColumnComparison.Expected so the
// sync policy layer can decide the db's varchar(36) needs expanding.
func TestCompareColumnsNormalizesMySQLUuidLength(t *testing.T) {
	dialect := mustInitDialect(t, schemas.MYSQL)
	expect := &schemas.Column{Name: "id", SQLType: schemas.SQLType{Name: schemas.Uuid}}
	actual := &schemas.Column{Name: "id", SQLType: schemas.SQLType{Name: schemas.Varchar}, Length: 36}

	comparison := dialect.CompareColumns(expect, actual)

	if comparison.Expected.Length != 40 {
		t.Fatalf("comparison.Expected.Length = %d, want 40", comparison.Expected.Length)
	}
	if comparison.Actual.Length != 36 {
		t.Fatalf("comparison.Actual.Length = %d, want 36", comparison.Actual.Length)
	}
}

// TestCompareColumnsPostgresSerialNullable is a regression guard for the
// normalized-clone requirement on the Nullable comparison: the struct
// parser leaves Nullable true on a non-primary-key Serial field, but
// postgres.SQLType forces Nullable false as a side effect of rendering
// "serial". The nullable comparison must run against that normalized
// value, matching v1's behaviour of mutating the column in place before
// comparing.
func TestCompareColumnsPostgresSerialNullable(t *testing.T) {
	dialect := mustInitDialect(t, schemas.POSTGRES)
	expect := &schemas.Column{Name: "seq", SQLType: schemas.SQLType{Name: schemas.Serial}, Nullable: true}
	actual := &schemas.Column{Name: "seq", SQLType: schemas.SQLType{Name: schemas.Serial}, Nullable: false}

	comparison := dialect.CompareColumns(expect, actual)

	if comparison.Nullable.IsDifferent() {
		t.Fatalf("Nullable comparison reported a difference: expected=%v actual=%v",
			comparison.Expected.Nullable, comparison.Actual.Nullable)
	}
}

// TestCompareColumnsMySQLNumericPrefixMismatch is the real-dialect
// counterpart of the "unaliased NUMERIC struct type" default-arm guard in
// package xorm's resolveColumnTypeSyncAction: mysql aliases "numeric" to
// "decimal", so a bare "NUMERIC" struct type and a literal "NUMERIC(10,2)"
// db type fail both the normalized-type (level 3) and base-name (level 4)
// checks and are correctly reported as Different here; the sync policy
// layer is responsible for silencing the resulting warning.
func TestCompareColumnsMySQLNumericPrefixMismatch(t *testing.T) {
	dialect := mustInitDialect(t, schemas.MYSQL)
	expect := &schemas.Column{Name: "n", SQLType: schemas.SQLType{Name: schemas.Numeric}}
	actual := &schemas.Column{Name: "n", SQLType: schemas.SQLType{Name: schemas.Numeric}, Length: 10, Length2: 2}

	comparison := dialect.CompareColumns(expect, actual)

	if comparison.Type.Status != ColumnCompareDifferent {
		t.Fatalf("comparison.Type.Status = %v, want ColumnCompareDifferent", comparison.Type.Status)
	}
	if comparison.Type.Expected != "NUMERIC" || comparison.Type.Actual != "NUMERIC(10,2)" {
		t.Fatalf("comparison.Type = %q/%q, want NUMERIC/NUMERIC(10,2)", comparison.Type.Expected, comparison.Type.Actual)
	}
}
