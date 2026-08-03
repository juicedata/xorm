// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"xorm.io/xorm/log"
	"xorm.io/xorm/schemas"
)

func TestSchemaTableForSyncIgnoresOnlyFromDBColumns(t *testing.T) {
	type syncOnlyFromDBSchema struct {
		Id         int64  `xorm:"pk"`
		Name       string `xorm:"index(idx_name)"`
		JoinedName string `xorm:"<- index(idx_joined_name)"`
	}

	engine := newTestEngine(t)
	table, err := engine.tagParser.ParseWithCache(reflect.ValueOf(syncOnlyFromDBSchema{}))
	if err != nil {
		t.Fatalf("ParseWithCache() error = %v", err)
	}

	filtered := schemaTableForSync(table)
	if filtered.GetColumn("joined_name") != nil {
		t.Fatalf("expected joined_name to be ignored during sync")
	}
	if filtered.GetColumn("name") == nil {
		t.Fatalf("expected regular columns to remain in sync schema")
	}
	if _, ok := filtered.Indexes["idx_joined_name"]; ok {
		t.Fatalf("expected indexes on xorm:\"<-\" columns to be ignored during sync")
	}
	if _, ok := filtered.Indexes["idx_name"]; !ok {
		t.Fatalf("expected indexes on regular columns to remain in sync schema")
	}
}

type syncOnlyFromDBExisting struct {
	Id         int64 `xorm:"pk"`
	Name       string
	JoinedName string `xorm:"<- index(idx_joined_name)"`
}

func (*syncOnlyFromDBExisting) TableName() string {
	return "sync_ignore_only_from_db_existing"
}

type syncOnlyFromDBNew struct {
	Id         int64 `xorm:"pk"`
	Name       string
	JoinedName string `xorm:"<- index(idx_joined_name)"`
}

func (*syncOnlyFromDBNew) TableName() string {
	return "sync_ignore_only_from_db_new"
}

func findSyncTestTable(t *testing.T, engine *Engine, tableName string) *schemas.Table {
	t.Helper()

	tables, err := engine.DBMetas()
	if err != nil {
		t.Fatalf("DBMetas() error = %v", err)
	}

	for _, table := range tables {
		if strings.EqualFold(table.Name, tableName) {
			return table
		}
	}

	t.Fatalf("table %s not found", tableName)
	return nil
}

func TestSyncWithOptionsIgnoreOnlyFromDBColumns(t *testing.T) {
	t.Run("default sync skips only-from-db columns", func(t *testing.T) {
		engine, err := NewEngine("sqlite3", filepath.Join(t.TempDir(), "sync-default.db"))
		if err != nil {
			t.Fatalf("NewEngine() error = %v", err)
		}
		defer engine.Close()
		engine.SetLogger(log.DiscardLogger{})

		if _, err = engine.Exec("CREATE TABLE sync_ignore_only_from_db_existing (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
			t.Fatalf("CREATE TABLE error = %v", err)
		}

		if _, err = engine.SyncWithOptions(SyncOptions{}, new(syncOnlyFromDBExisting)); err != nil {
			t.Fatalf("SyncWithOptions() error = %v", err)
		}

		table := findSyncTestTable(t, engine, "sync_ignore_only_from_db_existing")
		if table.GetColumn("joined_name") != nil {
			t.Fatalf("expected default sync to skip joined_name")
		}
		if len(table.Indexes) != 0 {
			t.Fatalf("expected no explicit indexes to be created for ignored columns, got %d", len(table.Indexes))
		}
	})

	t.Run("new table also skips only-from-db columns by default", func(t *testing.T) {
		engine, err := NewEngine("sqlite3", filepath.Join(t.TempDir(), "sync-ignore-new.db"))
		if err != nil {
			t.Fatalf("NewEngine() error = %v", err)
		}
		defer engine.Close()
		engine.SetLogger(log.DiscardLogger{})

		if _, err = engine.SyncWithOptions(SyncOptions{}, new(syncOnlyFromDBNew)); err != nil {
			t.Fatalf("SyncWithOptions() error = %v", err)
		}

		table := findSyncTestTable(t, engine, "sync_ignore_only_from_db_new")
		if table.GetColumn("joined_name") != nil {
			t.Fatalf("expected joined_name to be omitted when creating a new table by default")
		}
		if len(table.Indexes) != 0 {
			t.Fatalf("expected no explicit indexes to be created for ignored columns, got %d", len(table.Indexes))
		}
	})

	t.Run("only-from-db columns are not reported as missing struct fields", func(t *testing.T) {
		engine, err := NewEngine("sqlite3", filepath.Join(t.TempDir(), "sync-warn.db"))
		if err != nil {
			t.Fatalf("NewEngine() error = %v", err)
		}
		defer engine.Close()

		if _, err = engine.Exec("CREATE TABLE sync_ignore_only_from_db_existing (id INTEGER PRIMARY KEY, name TEXT, joined_name TEXT)"); err != nil {
			t.Fatalf("CREATE TABLE error = %v", err)
		}

		var buf bytes.Buffer
		engine.SetLogger(log.NewSimpleLogger3(&buf, "", 0, log.LOG_WARNING))
		if _, err = engine.SyncWithOptions(SyncOptions{WarnIfDatabaseColumnMissed: true}, new(syncOnlyFromDBExisting)); err != nil {
			t.Fatalf("SyncWithOptions() error = %v", err)
		}

		if strings.Contains(buf.String(), "has column joined_name but struct has not related field") {
			t.Fatalf("unexpected missing-column warning, got %q", buf.String())
		}
	})
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
