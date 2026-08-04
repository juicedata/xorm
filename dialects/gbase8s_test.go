// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"xorm.io/xorm/schemas"
)

func mustInitGBase8sDialect(t *testing.T) *gbase8s {
	t.Helper()

	dialect := &gbase8s{}
	assert.NoError(t, dialect.Init(&URI{DBType: schemas.GBASE8S}))
	return dialect
}

// TestGbase8sImplementsColumnCommentModifier pins the capability
// sync.go's columnSyncFeaturesFor type-asserts for: gbase8s must satisfy
// ColumnCommentModifier, which is what lets resolveCommentSyncDecision
// sync a comment even when the type comparison is only
// ColumnCompareEquivalent, because ModifyColumnCommentSQL never emits a
// type-altering clause. Features() itself deliberately does not carry
// this as a hand-set literal any more (see ColumnSyncFeatures.ColumnCommentOnly's
// doc comment) - the interface satisfaction below is the actual source
// of truth.
func TestGbase8sImplementsColumnCommentModifier(t *testing.T) {
	dialect := mustInitGBase8sDialect(t)

	features := dialect.Features().ColumnSync
	assert.True(t, features.ColumnComment)

	_, ok := Dialect(dialect).(ColumnCommentModifier)
	assert.True(t, ok, "gbase8s must implement ColumnCommentModifier")
}

// TestGbase8sModifyColumnCommentSQLHasNoModifyClause pins the
// comment-only DDL path: ModifyColumnCommentSQL must render nothing but a
// standalone "COMMENT ON COLUMN" statement, with no "ALTER TABLE ...
// MODIFY" clause at all, unlike ModifyColumnSQL which always couples the
// two.
func TestGbase8sModifyColumnCommentSQLHasNoModifyClause(t *testing.T) {
	dialect := mustInitGBase8sDialect(t)

	col := &schemas.Column{Name: "price", Comment: "new comment"}

	commentSQL := dialect.ModifyColumnCommentSQL("t", col)
	assert.Equal(t, `COMMENT ON COLUMN "t"."price" IS 'new comment'`, commentSQL)
	assert.NotContains(t, commentSQL, "MODIFY")
	assert.NotContains(t, commentSQL, "ALTER")

	fullRewriteSQL := dialect.ModifyColumnSQL("t", col)
	assert.Contains(t, fullRewriteSQL, "MODIFY",
		"sanity: the full-rewrite path for the same column does render a MODIFY clause, unlike ModifyColumnCommentSQL")
}
