// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"reflect"
	"testing"
)

func TestParseMSSQL(t *testing.T) {
	tests := []struct {
		in       string
		expected string
		valid    bool
	}{
		{"sqlserver://sa:yourStrong(!)Password@localhost:1433?database=db&connection+timeout=30", "db", true},
		{"server=localhost;user id=sa;password=yourStrong(!)Password;database=db", "db", true},
	}

	driver := QueryDriver("mssql")

	for _, test := range tests {
		uri, err := driver.Parse("mssql", test.in)

		if err != nil && test.valid {
			t.Errorf("%q got unexpected error: %s", test.in, err)
		} else if err == nil && !reflect.DeepEqual(test.expected, uri.DBName) {
			t.Errorf("%q got: %#v want: %#v", test.in, uri.DBName, test.expected)
		}
	}
}

// TestMssqlAliasNumericToDecimal pins mssql's Alias mapping, which mirrors
// the one postgres already has: DECIMAL and NUMERIC are exact synonyms in
// SQL Server, so CompareColumns' base-name level (dialects/compare.go)
// must treat them as the same type regardless of which spelling the
// database reports or the struct tag uses.
func TestMssqlAliasNumericToDecimal(t *testing.T) {
	db := &mssql{}

	tests := []struct {
		in   string
		want string
	}{
		{"numeric", "decimal"},
		{"NUMERIC", "decimal"},
		{"decimal", "decimal"},
		{"varchar", "varchar"},
	}

	for _, tt := range tests {
		if got := db.Alias(tt.in); got != tt.want {
			t.Errorf("Alias(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
