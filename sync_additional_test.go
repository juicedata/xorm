// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"testing"

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
