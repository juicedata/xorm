// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"

	_ "gitee.com/travelliu/dm"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/assert"
	_ "github.com/ziutek/mymysql/godrv"
	_ "modernc.org/sqlite"
)

func TestPing(t *testing.T) {
	if err := testEngine.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestPingContext(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	ctx, canceled := context.WithTimeout(context.Background(), time.Nanosecond)
	defer canceled()

	time.Sleep(time.Nanosecond)

	err := testEngine.(*xorm.Engine).PingContext(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestAutoTransaction(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestTx struct {
		Id      int64     `xorm:"autoincr pk"`
		Msg     string    `xorm:"varchar(255)"`
		Created time.Time `xorm:"created"`
	}

	assert.NoError(t, testEngine.Sync(new(TestTx)))

	engine := testEngine.(*xorm.Engine)

	// will success
	_, err := engine.Transaction(func(session *xorm.Session) (any, error) {
		_, err := session.Insert(TestTx{Msg: "hi"})
		assert.NoError(t, err)

		return nil, nil
	})
	assert.NoError(t, err)

	has, err := engine.Exist(&TestTx{Msg: "hi"})
	assert.NoError(t, err)
	assert.EqualValues(t, true, has)

	// will rollback
	_, err = engine.Transaction(func(session *xorm.Session) (any, error) {
		_, err := session.Insert(TestTx{Msg: "hello"})
		assert.NoError(t, err)

		return nil, errors.New("rollback")
	})
	assert.Error(t, err)

	has, err = engine.Exist(&TestTx{Msg: "hello"})
	assert.NoError(t, err)
	assert.EqualValues(t, false, has)
}

func assertSync(t *testing.T, beans ...any) {
	for _, bean := range beans {
		t.Run(testEngine.TableName(bean, true), func(t *testing.T) {
			assert.NoError(t, testEngine.DropTables(bean))
			assert.NoError(t, testEngine.Sync(bean))
		})
	}
}

func TestDump(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestDumpStruct struct {
		Id      int64
		Name    string
		IsMan   bool
		Created time.Time `xorm:"created"`
	}

	assertSync(t, new(TestDumpStruct))

	cnt, err := testEngine.Insert([]TestDumpStruct{
		{Name: "1", IsMan: true},
		{Name: "2\n"},
		{Name: "3;"},
		{Name: "4\n;\n''"},
		{Name: "5'\n"},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 5, cnt)

	fp := fmt.Sprintf("%v.sql", testEngine.Dialect().URI().DBType)
	os.Remove(fp)
	assert.NoError(t, testEngine.DumpAllToFile(fp))

	assert.NoError(t, PrepareEngine())

	sess := testEngine.NewSession()
	defer sess.Close()
	assert.NoError(t, sess.Begin())
	_, err = sess.ImportFile(fp)
	assert.NoError(t, err)
	assert.NoError(t, sess.Commit())

	for _, tp := range []schemas.DBType{schemas.SQLITE, schemas.MYSQL, schemas.POSTGRES, schemas.MSSQL} {
		name := fmt.Sprintf("dump_%v.sql", tp)
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, testEngine.DumpAllToFile(name, tp))
		})
	}
}

func TestDumpSQLiteLargeText(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	if testEngine.Dialect().URI().DBType != schemas.SQLITE {
		t.Skip("sqlite only")
	}

	type TestDumpLargeText struct {
		Id   int64
		Body string
	}

	assertSync(t, new(TestDumpLargeText))

	largeText := strings.Repeat("line\n", 1200)
	_, err := testEngine.Insert(&TestDumpLargeText{Body: largeText})
	assert.NoError(t, err)

	fp := "sqlite_dump_large_text.sql"
	os.Remove(fp)
	assert.NoError(t, testEngine.DumpAllToFile(fp))

	assert.NoError(t, PrepareEngine())

	sess := testEngine.NewSession()
	defer sess.Close()
	assert.NoError(t, sess.Begin())
	_, err = sess.ImportFile(fp)
	assert.NoError(t, err)
	assert.NoError(t, sess.Commit())

	var got TestDumpLargeText
	has, err := testEngine.Get(&got)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, largeText, got.Body)
}

var dbtypes = []schemas.DBType{schemas.SQLITE, schemas.MYSQL, schemas.POSTGRES, schemas.MSSQL}

func TestDumpTables(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestDumpTableStruct struct {
		Id      int64
		Data    []byte `xorm:"BLOB"`
		Name    string
		IsMan   bool
		Created time.Time `xorm:"created"`
	}

	assertSync(t, new(TestDumpTableStruct))

	_, err := testEngine.Insert([]TestDumpTableStruct{
		{Name: "1", IsMan: true},
		{Name: "2\n", Data: []byte{'\000', '\001', '\002'}},
		{Name: "3;", Data: []byte("0x000102")},
		{Name: "4\n;\n''", Data: []byte("Help")},
		{Name: "5'\n", Data: []byte("0x48656c70")},
		{Name: "6\\n'\n", Data: []byte("48656c70")},
		{Name: "7\\n'\r\n", Data: []byte("7\\n'\r\n")},
		{Name: "x0809ee"},
		{Name: "090a10"},
	})
	assert.NoError(t, err)

	fp := fmt.Sprintf("%v-table.sql", testEngine.Dialect().URI().DBType)
	os.Remove(fp)
	tb, err := testEngine.TableInfo(new(TestDumpTableStruct))
	assert.NoError(t, err)
	assert.NoError(t, testEngine.(*xorm.Engine).DumpTablesToFile([]*schemas.Table{tb}, fp))

	assert.NoError(t, PrepareEngine())

	sess := testEngine.NewSession()
	defer sess.Close()
	assert.NoError(t, sess.Begin())
	_, err = sess.ImportFile(fp)
	assert.NoError(t, err)
	assert.NoError(t, sess.Commit())

	for _, tp := range dbtypes {
		name := fmt.Sprintf("dump_%v-table.sql", tp)
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, testEngine.(*xorm.Engine).DumpTablesToFile([]*schemas.Table{tb}, name, tp))
		})
	}

	assert.NoError(t, testEngine.DropTables(new(TestDumpTableStruct)))

	importPath := fmt.Sprintf("dump_%v-table.sql", testEngine.Dialect().URI().DBType)
	t.Run("import_"+importPath, func(t *testing.T) {
		sess := testEngine.NewSession()
		defer sess.Close()
		assert.NoError(t, sess.Begin())
		_, err = sess.ImportFile(importPath)
		assert.NoError(t, err)
		assert.NoError(t, sess.Commit())
	})
}

func TestDumpTables2(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestDumpTableStruct2 struct {
		Id      int64
		Created time.Time `xorm:"Default CURRENT_TIMESTAMP"`
	}

	assertSync(t, new(TestDumpTableStruct2))

	fp := fmt.Sprintf("./dump2-%v-table.sql", testEngine.Dialect().URI().DBType)
	os.Remove(fp)
	tb, err := testEngine.TableInfo(new(TestDumpTableStruct2))
	assert.NoError(t, err)
	assert.NoError(t, testEngine.(*xorm.Engine).DumpTablesToFile([]*schemas.Table{tb}, fp))
}

func TestSetSchema(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	if testEngine.Dialect().URI().DBType == schemas.POSTGRES {
		oldSchema := testEngine.Dialect().URI().Schema
		testEngine.SetSchema("my_schema")
		assert.EqualValues(t, "my_schema", testEngine.Dialect().URI().Schema)
		testEngine.SetSchema(oldSchema)
		assert.EqualValues(t, oldSchema, testEngine.Dialect().URI().Schema)
	}
}

func TestImport(t *testing.T) {
	if testEngine.Dialect().URI().DBType != schemas.MYSQL {
		t.Skip()
		return
	}
	sess := testEngine.NewSession()
	defer sess.Close()
	assert.NoError(t, sess.Begin())
	_, err := sess.ImportFile("./testdata/import1.sql")
	assert.NoError(t, err)
	assert.NoError(t, sess.Commit())

	assert.NoError(t, sess.Begin())
	_, err = sess.ImportFile("./testdata/import2.sql")
	assert.NoError(t, err)
	assert.NoError(t, sess.Commit())
}

func TestDBVersion(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	version, err := testEngine.DBVersion()
	assert.NoError(t, err)

	t.Log(testEngine.Dialect().URI().DBType, "version is", version)
}

func TestGetColumnsComment(t *testing.T) {
	switch testEngine.Dialect().URI().DBType {
	case schemas.POSTGRES, schemas.MYSQL:
	default:
		t.Skip()
		return
	}
	comment := "this is a comment"
	type TestCommentStruct struct {
		HasComment int `xorm:"comment('this is a comment')"`
		NoComment  int
	}

	assertSync(t, new(TestCommentStruct))

	tables, err := testEngine.DBMetas()
	assert.NoError(t, err)
	tableName := testEngine.GetColumnMapper().Obj2Table("TestCommentStruct")
	var hasComment, noComment string
	for _, table := range tables {
		if table.Name == tableName {
			col := table.GetColumn(testEngine.GetColumnMapper().Obj2Table("HasComment"))
			assert.NotNil(t, col)
			hasComment = col.Comment
			col2 := table.GetColumn(testEngine.GetColumnMapper().Obj2Table("NoComment"))
			assert.NotNil(t, col2)
			noComment = col2.Comment
			break
		}
	}
	assert.Equal(t, comment, hasComment)
	assert.Zero(t, noComment)
}

type TestCommentUpdate struct {
	HasComment int `xorm:"bigint comment('this is a comment before update')"`
}

func (m *TestCommentUpdate) TableName() string {
	return "test_comment_struct"
}

type TestCommentUpdate2 struct {
	HasComment int `xorm:"bigint comment('this is a comment after update')"`
}

func (m *TestCommentUpdate2) TableName() string {
	return "test_comment_struct"
}

func TestColumnCommentUpdate(t *testing.T) {
	comment := "this is a comment after update"
	assertSync(t, new(TestCommentUpdate))
	assert.NoError(t, testEngine.Sync2(new(TestCommentUpdate2))) // modify table column comment

	switch testEngine.Dialect().URI().DBType {
	case schemas.POSTGRES, schemas.MYSQL: // only postgres / mysql dialect implement the feature of modify comment in postgres.ModifyColumnSQL
	default:
		t.Skip()
		return
	}
	tables, err := testEngine.DBMetas()
	assert.NoError(t, err)
	tableName := "test_comment_struct"
	var hasComment string
	for _, table := range tables {
		if table.Name == tableName {
			col := table.GetColumn(testEngine.GetColumnMapper().Obj2Table("HasComment"))
			assert.NotNil(t, col)
			hasComment = col.Comment
			break
		}
	}
	assert.Equal(t, comment, hasComment)
}

func TestGetColumnsLength(t *testing.T) {
	var max_length int64
	switch testEngine.Dialect().URI().DBType {
	case schemas.POSTGRES:
		max_length = 0
	case schemas.MYSQL:
		max_length = 65535
	default:
		t.Skip()
		return
	}

	type TestLengthStringStruct struct {
		Content string `xorm:"TEXT NOT NULL"`
	}

	assertSync(t, new(TestLengthStringStruct))

	tables, err := testEngine.DBMetas()
	assert.NoError(t, err)
	tableLengthStringName := testEngine.GetColumnMapper().Obj2Table("TestLengthStringStruct")
	for _, table := range tables {
		if table.Name == tableLengthStringName {
			col := table.GetColumn("content")
			assert.Equal(t, col.Length, max_length)
			assert.Zero(t, col.Length2)
			break
		}
	}
}
