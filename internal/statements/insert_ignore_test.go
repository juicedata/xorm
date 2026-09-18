// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"strings"
	"testing"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func TestInsertIgnoreSQL(t *testing.T) {
	for _, db := range []schemas.DBType{schemas.MYSQL, schemas.SQLITE, schemas.POSTGRES, schemas.MSSQL, schemas.ORACLE, schemas.DAMENG} {
		t.Run(string(db), func(t *testing.T) {
			statement := newStatementWithTable(t, db)
			statement.InsertIgnore = true
			prefix, suffix := "INSERT INTO ", " ON CONFLICT DO NOTHING"
			switch db {
			case schemas.MYSQL:
				prefix, suffix = "INSERT IGNORE INTO ", ""
			case schemas.SQLITE:
				prefix, suffix = "INSERT OR IGNORE INTO ", ""
			}
			generators := map[string]func() (string, error){
				"struct": func() (string, error) {
					query, _, err := statement.GenInsertSQL([]string{"name"}, []any{"value"})
					return query, err
				},
				"defaults": func() (string, error) {
					query, _, err := statement.GenInsertSQL(nil, nil)
					return query, err
				},
				"structs": func() (string, error) {
					w := builder.NewWriter()
					err := statement.WriteInsertMultiple(w, "users", []string{"name"}, []string{"?", "?"})
					return w.String(), err
				},
				"map": func() (string, error) {
					query, _, err := statement.GenInsertMapSQL([]string{"name"}, []any{"value"})
					return query, err
				},
				"maps": func() (string, error) {
					query, _, err := statement.GenInsertMultipleMapSQL([]string{"name"}, [][]any{{"value"}, {"other"}})
					return query, err
				},
			}
			for name, generate := range generators {
				t.Run(name, func(t *testing.T) {
					query, err := generate()
					if db != schemas.MYSQL && db != schemas.SQLITE && db != schemas.POSTGRES {
						if err == nil || !strings.Contains(err.Error(), "InsertIgnore is not supported") {
							t.Fatalf("expected unsupported error, got query %q, err %v", query, err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if !strings.HasPrefix(query, prefix) || !strings.Contains(query, suffix) {
						t.Fatalf("unexpected SQL: %s", query)
					}
					if db == schemas.POSTGRES && (name == "struct" || name == "defaults") && !strings.Contains(query, "ON CONFLICT DO NOTHING RETURNING") {
						t.Fatalf("conflict clause must precede RETURNING: %s", query)
					}
				})
			}
			statement.Reset()
			if statement.InsertIgnore {
				t.Fatal("Reset retained InsertIgnore")
			}
		})
	}
}
