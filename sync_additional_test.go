// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"xorm.io/xorm/log"
	"xorm.io/xorm/schemas"
)

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

func TestColumnTypesMatch(t *testing.T) {
	alias := func(columnType string) string {
		if columnType == "NUMERIC" {
			return "DECIMAL"
		}
		return columnType
	}

	tests := []struct {
		name         string
		expectedCol  *schemas.Column
		currentCol   *schemas.Column
		expectedType string
		currentType  string
		want         bool
	}{
		{
			name:         "mysql unsigned display width warning is suppressed",
			expectedCol:  &schemas.Column{},
			currentCol:   &schemas.Column{},
			expectedType: "INT UNSIGNED",
			currentType:  "INT(10) UNSIGNED",
			want:         true,
		},
		{
			name:         "mysql signedness mismatch still warns",
			expectedCol:  &schemas.Column{},
			currentCol:   &schemas.Column{},
			expectedType: "INT UNSIGNED",
			currentType:  "INT(10)",
			want:         false,
		},
		{
			name:         "type aliases still match",
			expectedCol:  &schemas.Column{},
			currentCol:   &schemas.Column{},
			expectedType: "DECIMAL(10,2)",
			currentType:  "NUMERIC(10,2)",
			want:         true,
		},
		{
			name: "json-tagged text matches native json column",
			expectedCol: &schemas.Column{
				IsJSON: true,
			},
			currentCol:   &schemas.Column{},
			expectedType: "TEXT",
			currentType:  "JSON",
			want:         true,
		},
		{
			name: "jsonb-tagged text matches native jsonb column",
			expectedCol: &schemas.Column{
				IsJSON:  true,
				IsJSONB: true,
			},
			currentCol:   &schemas.Column{},
			expectedType: "TEXT",
			currentType:  "JSONB",
			want:         true,
		},
		{
			name:         "plain text still differs from native json column",
			expectedCol:  &schemas.Column{},
			currentCol:   &schemas.Column{},
			expectedType: "TEXT",
			currentType:  "JSON",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnTypesMatch(alias, tt.expectedCol, tt.currentCol, tt.expectedType, tt.currentType); got != tt.want {
				t.Fatalf("columnTypesMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

type syncWarnMissingColumnInitial struct {
	ID        int64
	LegacyCol string
}

func (syncWarnMissingColumnInitial) TableName() string {
	return "sync_warn_missing_column"
}

type syncWarnMissingColumnCurrent struct {
	ID int64
}

func (syncWarnMissingColumnCurrent) TableName() string {
	return "sync_warn_missing_column"
}

func TestSyncWithWarnIfDatabaseColumnMissed(t *testing.T) {
	engine, err := NewEngine("sqlite3", filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	defer engine.Close()

	assertNoError := func(err error) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	assertNoError(engine.Sync(new(syncWarnMissingColumnInitial)))

	var buf bytes.Buffer
	logger := log.NewSimpleLogger3(&buf, "", 0, log.LOG_WARNING)
	engine.SetLogger(logger)

	_, err = engine.SyncWithOptions(SyncOptions{WarnIfDatabaseColumnMissed: true}, new(syncWarnMissingColumnCurrent))
	assertNoError(err)

	if !strings.Contains(buf.String(), "Table sync_warn_missing_column has column legacy_col but struct has not related field") {
		t.Fatalf("expected missing-column warning, got %q", buf.String())
	}
}

func TestSyncWithoutWarnIfDatabaseColumnMissed(t *testing.T) {
	engine, err := NewEngine("sqlite3", filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	defer engine.Close()

	if err = engine.Sync(new(syncWarnMissingColumnInitial)); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	var buf bytes.Buffer
	logger := log.NewSimpleLogger3(&buf, "", 0, log.LOG_WARNING)
	engine.SetLogger(logger)

	_, err = engine.SyncWithOptions(SyncOptions{}, new(syncWarnMissingColumnCurrent))
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}

	if strings.Contains(buf.String(), "struct has not related field") {
		t.Fatalf("unexpected missing-column warning, got %q", buf.String())
	}
}
