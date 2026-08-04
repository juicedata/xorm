// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"strings"
	"testing"

	"xorm.io/xorm/schemas"
)

// TestInvariantSweepCommentSyncNeverChangesType is the invariant sweep
// xorm/xorm#2591's follow-up (xorm/xorm#2594's residual hole) asks for:
// across every (expected type, actual type, length) combination that
// reaches a comment sync - i.e. every pair CompareColumns does not call
// ColumnCompareDifferent, on every dialect that supports comment sync -
// the statement Sync would actually run must never change the column's
// type. For a dialect implementing ColumnCommentModifier (postgres,
// gbase8s) that means ModifyColumnCommentSQL never renders "ALTER" or a
// " TYPE " clause at all; for a full-rewrite-only dialect restricted to
// ColumnCompareEqual (mysql/mariadb) it means ModifyColumnSQL's reissued
// type text contains an independently, freshly computed SQLType() ground
// truth for that exact (name, length) shape - not comparison.Type.Actual
// itself, which ColumnCompareEqual already guarantees equals
// comparison.Type.Expected by construction, making a check against it
// tautological (review feedback on an earlier revision of this sweep).
func TestInvariantSweepCommentSyncNeverChangesType(t *testing.T) {
	lengthShapes := []struct {
		length1, length2 int64
	}{
		{0, 0}, {10, 0}, {19, 4}, {255, 0},
	}

	dbTypes := []schemas.DBType{schemas.MYSQL, schemas.POSTGRES, schemas.GBASE8S}

	total := 0
	gatePassing := 0
	commentOnlyCount := 0
	fullRewriteCount := 0

	for _, dbType := range dbTypes {
		dialect := QueryDialect(dbType)
		if dialect == nil {
			t.Fatalf("QueryDialect(%q) returned nil", dbType)
		}
		if err := dialect.Init(&URI{DBType: dbType}); err != nil {
			t.Fatalf("Init(%q) error = %v", dbType, err)
		}
		features := dialect.Features().ColumnSync
		modifier, isCommentOnly := dialect.(ColumnCommentModifier)

		for expectedName := range schemas.SqlTypes {
			for actualName := range schemas.SqlTypes {
				for _, ls := range lengthShapes {
					total++

					expected := &schemas.Column{Name: "col", SQLType: schemas.SQLType{Name: expectedName}, Length: ls.length1, Length2: ls.length2, Comment: "new"}
					actual := &schemas.Column{Name: "col", SQLType: schemas.SQLType{Name: actualName}, Length: ls.length1, Length2: ls.length2, Comment: "old"}

					// Ground truth for the full-rewrite check below,
					// computed fresh on a throwaway column instance
					// before CompareColumns gets a chance to mutate
					// expected/actual (SQLType has side effects on
					// several dialects), so the later assertion is not
					// just re-deriving what ColumnCompareEqual already
					// guarantees about comparison.Type.Actual itself.
					actualRenderedTypeGroundTruth := dialect.SQLType(&schemas.Column{SQLType: schemas.SQLType{Name: actualName}, Length: ls.length1, Length2: ls.length2})

					comparison := dialect.CompareColumns(expected, actual)
					if comparison.Type.Status == ColumnCompareDifferent {
						continue
					}
					if !features.ColumnComment {
						continue
					}

					gatePassing++

					if isCommentOnly {
						commentOnlyCount++
						sql := modifier.ModifyColumnCommentSQL("t", comparison.Expected)
						if strings.Contains(strings.ToUpper(sql), "ALTER") || strings.Contains(strings.ToUpper(sql), " TYPE ") {
							t.Fatalf("%s: ModifyColumnCommentSQL rendered a type-altering clause for expected=%q actual=%q: %s",
								dbType, expectedName, actualName, sql)
						}
						continue
					}

					if comparison.Type.Status != ColumnCompareEqual {
						continue
					}
					fullRewriteCount++
					sql := dialect.ModifyColumnSQL("t", comparison.Expected)
					if !strings.Contains(sql, actualRenderedTypeGroundTruth) {
						t.Fatalf("%s: ModifyColumnSQL's reissued type does not contain the independently computed actual rendered type %q for expected=%q actual=%q: %s",
							dbType, actualRenderedTypeGroundTruth, expectedName, actualName, sql)
					}
				}
			}
		}
	}

	t.Logf("invariant sweep: total=%d gatePassing=%d commentOnly=%d fullRewrite=%d", total, gatePassing, commentOnlyCount, fullRewriteCount)
	if gatePassing == 0 {
		t.Fatalf("gatePassing = 0, want > 0: the sweep found nothing to check, which would make every assertion above vacuous")
	}
	if commentOnlyCount == 0 {
		t.Fatalf("commentOnlyCount = 0, want > 0: expected postgres/gbase8s to contribute comment-only cases")
	}
	if fullRewriteCount == 0 {
		t.Fatalf("fullRewriteCount = 0, want > 0: expected mysql to contribute Equal-gated full-rewrite cases")
	}
}
