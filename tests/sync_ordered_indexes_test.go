// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type orderedIndexAction struct {
	UserID      int64 `xorm:"user_id"`
	IsDeleted   bool
	CreatedUnix int64
}

func (*orderedIndexAction) TableIndices() []*schemas.Index {
	a := schemas.NewIndex("c_u", schemas.IndexType)
	a.AddColumn("user_id", "is_deleted", "created_unix")
	b := schemas.NewIndex("c_u_d", schemas.IndexType)
	b.AddColumn("created_unix", "user_id", "is_deleted")
	return []*schemas.Index{a, b}
}

func TestSyncOrderedIndexes(t *testing.T) {
	require.NoError(t, PrepareEngine())
	bean := new(orderedIndexAction)
	require.NoError(t, testEngine.Sync(bean))
	check := func() {
		t.Helper()
		tables, err := testEngine.DBMetas()
		require.NoError(t, err)
		require.Len(t, tables, 1)
		require.Len(t, tables[0].Indexes, 2)
		for _, expected := range bean.TableIndices() {
			actual := tables[0].Indexes[expected.Name]
			require.NotNil(t, actual, "missing index %s", expected.Name)
			require.Equal(t, expected.Cols, actual.Cols)
		}
	}
	check()
	for i := 0; i < 3; i++ {
		require.NoError(t, testEngine.Sync(bean))
		check()
	}
	for _, index := range bean.TableIndices() {
		sql := testEngine.Dialect().DropIndexSQL(testEngine.TableName(bean), index)
		_, err := testEngine.Exec(sql)
		require.NoError(t, err)
		require.NoError(t, testEngine.Sync(bean))
		check()
	}
}
