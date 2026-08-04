// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"xorm.io/xorm/schemas"
)

// TestMysqlDoesNotImplementColumnCommentModifier pins the decision
// documented on ColumnSyncFeatures.ColumnCommentOnly: mysql/mariadb's
// MODIFY COLUMN embeds the comment inside the full column definition,
// with no standalone "comment only" statement, so mysql does not
// implement ColumnCommentModifier at all - sync.go's columnSyncFeaturesFor
// therefore always computes ColumnCommentOnly false for mysql, and
// resolveCommentSyncDecision keeps gating mysql's comment sync on the
// type comparison being ColumnCompareEqual.
func TestMysqlDoesNotImplementColumnCommentModifier(t *testing.T) {
	dialect := &mysql{}
	assert.NoError(t, dialect.Init(&URI{DBType: schemas.MYSQL}))

	features := dialect.Features().ColumnSync
	assert.True(t, features.ColumnComment)

	_, ok := Dialect(dialect).(ColumnCommentModifier)
	assert.False(t, ok, "mysql must not implement ColumnCommentModifier")
}
