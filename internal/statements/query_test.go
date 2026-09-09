// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func TestGenExistSQL(t *testing.T) {
	statement, err := createTestStatement()
	assert.NoError(t, err)

	statement.RefTable = nil
	statement.SetTable("testDB")
	statement.Alias("tdb")
	statement.Where(`tdb.id=1`)
	sql, _, err := statement.GenExistSQL()
	assert.NoError(t, err)
	assert.Equal(t, "SELECT 1 FROM `testDB` AS `tdb` WHERE (tdb.id=1) LIMIT 1", sql)
}

func TestWriteForUpdate(t *testing.T) {
	tests := []struct {
		name   string
		dbType schemas.DBType
		want   string
	}{
		{"mysql", schemas.MYSQL, " FOR UPDATE"},
		{"postgres", schemas.POSTGRES, " FOR UPDATE"},
		{"sqlite", schemas.SQLITE, ""},
		{"mssql", schemas.MSSQL, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := newTestStatement(t, tt.dbType)
			statement.ForUpdate()
			writer := builder.NewWriter()
			if err := statement.writeForUpdate(writer); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := writer.String(); got != tt.want {
				t.Fatalf("unexpected SQL: got %q, want %q", got, tt.want)
			}
		})
	}
}
