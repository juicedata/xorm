// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"path/filepath"
	"testing"

	"xorm.io/xorm/dialects"
	"xorm.io/xorm/log"
	"xorm.io/xorm/schemas"
)

type recordingSyncLogger struct {
	warnfFormats []string
	infofFormats []string
}

func (l *recordingSyncLogger) BeforeSQL(log.LogContext) {}
func (l *recordingSyncLogger) AfterSQL(log.LogContext)  {}
func (l *recordingSyncLogger) Debugf(string, ...any)    {}
func (l *recordingSyncLogger) Errorf(string, ...any)    {}
func (l *recordingSyncLogger) Infof(format string, v ...any) {
	l.infofFormats = append(l.infofFormats, format)
}
func (l *recordingSyncLogger) Warnf(format string, v ...any) {
	l.warnfFormats = append(l.warnfFormats, format)
}
func (l *recordingSyncLogger) Level() log.LogLevel   { return log.LOG_WARNING }
func (l *recordingSyncLogger) SetLevel(log.LogLevel) {}
func (l *recordingSyncLogger) ShowSQL(...bool)       {}
func (l *recordingSyncLogger) IsShowSQL() bool       { return false }

func mustInitTestDialect(t *testing.T, dbType schemas.DBType) dialects.Dialect {
	t.Helper()

	dialect := dialects.QueryDialect(dbType)
	if dialect == nil {
		t.Fatalf("QueryDialect(%q) returned nil", dbType)
	}

	if err := dialect.Init(&dialects.URI{DBType: dbType}); err != nil {
		t.Fatalf("Init(%q) error = %v", dbType, err)
	}

	return dialect
}

func typeDiffersComparison(expectedType, actualType string, expectedLength, actualLength int64) dialects.ColumnComparison {
	return dialects.ColumnComparison{
		Type: dialects.ColumnCompareField{
			Status:   dialects.ColumnCompareDifferent,
			Expected: expectedType,
			Actual:   actualType,
		},
		Expected: &schemas.Column{Name: "col", Length: expectedLength},
		Actual:   &schemas.Column{Name: "col", Length: actualLength},
	}
}

func TestResolveColumnTypeSyncActionVarcharShrinkStaysSilent(t *testing.T) {
	features := dialects.ColumnSyncFeatures{VarcharLengthChange: true}
	comparison := typeDiffersComparison("VARCHAR(50)", "VARCHAR(100)", 50, 100)

	if got := resolveColumnTypeSyncAction(nil, features, comparison); got != columnTypeSyncActionNone {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionNone", got)
	}
}

func TestResolveColumnTypeSyncActionVarcharLengthChangeUnsupportedStaysSilent(t *testing.T) {
	features := dialects.ColumnSyncFeatures{VarcharLengthChange: false}
	comparison := typeDiffersComparison("VARCHAR(100)", "VARCHAR(50)", 100, 50)

	if got := resolveColumnTypeSyncAction(nil, features, comparison); got != columnTypeSyncActionNone {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionNone", got)
	}
}

func TestResolveColumnTypeSyncActionVarcharExpandModifies(t *testing.T) {
	features := dialects.ColumnSyncFeatures{VarcharLengthChange: true}
	comparison := typeDiffersComparison("VARCHAR(100)", "VARCHAR(50)", 100, 50)

	if got := resolveColumnTypeSyncAction(nil, features, comparison); got != columnTypeSyncActionModifyVarcharExpand {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionModifyVarcharExpand", got)
	}
}

func TestResolveColumnTypeSyncActionTextFromVarcharUnsupportedWarns(t *testing.T) {
	features := dialects.ColumnSyncFeatures{TextFromVarchar: false}
	comparison := typeDiffersComparison(schemas.Text, "VARCHAR(100)", 0, 100)

	if got := resolveColumnTypeSyncAction(nil, features, comparison); got != columnTypeSyncActionWarnTextFromVarchar {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionWarnTextFromVarchar", got)
	}
}

func TestResolveColumnTypeSyncActionTextFromVarcharSupportedModifies(t *testing.T) {
	features := dialects.ColumnSyncFeatures{TextFromVarchar: true}
	comparison := typeDiffersComparison(schemas.Text, "VARCHAR(100)", 0, 100)

	if got := resolveColumnTypeSyncAction(nil, features, comparison); got != columnTypeSyncActionModify {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionModify", got)
	}
}

func TestResolveColumnTypeSyncActionUnaliasedPrefixMatchStaysSilent(t *testing.T) {
	// Reproduces v1's default-arm guard: an unaliased struct type "NUMERIC"
	// against a literal db type "NUMERIC(10,2)" is a byte-for-byte prefix
	// match, so no warning is produced even though the two rendered types
	// differ.
	features := dialects.ColumnSyncFeatures{}
	comparison := typeDiffersComparison("NUMERIC", "NUMERIC(10,2)", 0, 10)

	if got := resolveColumnTypeSyncAction(nil, features, comparison); got != columnTypeSyncActionNone {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionNone", got)
	}
}

// TestResolveColumnTypeSyncActionMySQLNumericPrefixMatchStaysSilent is the
// real-dialect counterpart of the guard above: mysql aliases "numeric" to
// "decimal", so CompareColumns reports a bare "NUMERIC" struct column
// against a literal "NUMERIC(10,2)" db column as Different (see
// dialects.TestCompareColumnsMySQLNumericPrefixMismatch), and
// resolveColumnTypeSyncAction must still stay silent for it.
func TestResolveColumnTypeSyncActionMySQLNumericPrefixMatchStaysSilent(t *testing.T) {
	dialect := mustInitTestDialect(t, schemas.MYSQL)
	expect := &schemas.Column{Name: "n", SQLType: schemas.SQLType{Name: schemas.Numeric}}
	actual := &schemas.Column{Name: "n", SQLType: schemas.SQLType{Name: schemas.Numeric}, Length: 10, Length2: 2}

	comparison := dialect.CompareColumns(expect, actual)
	if !comparison.Type.IsDifferent() {
		t.Fatalf("comparison.Type.IsDifferent() = false, want true")
	}

	features := dialects.ColumnSyncFeatures{TextFromVarchar: true, VarcharLengthChange: true, ColumnComment: true}
	if got := resolveColumnTypeSyncAction(dialect.Alias, features, comparison); got != columnTypeSyncActionNone {
		t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionNone", got)
	}
}

// TestResolveColumnTypeSyncActionPostgresDecimalNumericSynonymsMatchSymmetrically
// pins the fix for the regression found in review of xorm/xorm#2587: postgres
// aliases "numeric" to "decimal", but only in that direction, so a bare
// "NUMERIC" struct type against a "NUMERIC(10,2)" column matched the
// unaliased prefix rule directly while a bare "DECIMAL" struct type against
// the same column did not - the same field's warning depended on which
// synonym the user happened to type. All four struct tag spellings -
// "DECIMAL", "DECIMAL(10,2)", "NUMERIC", "NUMERIC(10,2)" - must resolve the
// same way (silent) against a "NUMERIC(10,2)" column, and the two unsized
// spellings must also agree (silent) against a bare "NUMERIC" column.
//
// Only the two bare-vs-sized rows ("decimal_vs_sized_numeric_column" and
// "numeric_vs_sized_numeric_column") actually reach
// columnTypeBaseNameMatchesPrefix: compareColumnTypes classifies every other
// row as Equal or Equivalent before that guard runs, so those rows document
// the surrounding matrix rather than pinning the fix. See the reachesGuard
// field below.
func TestResolveColumnTypeSyncActionPostgresDecimalNumericSynonymsMatchSymmetrically(t *testing.T) {
	dialect := mustInitTestDialect(t, schemas.POSTGRES)
	features := dialects.ColumnSyncFeatures{TextFromVarchar: true, VarcharLengthChange: true, ColumnComment: true}

	tests := []struct {
		name           string
		expectedName   string
		expectedLength int64
		actualLength   int64
		// reachesGuard records whether CompareColumns classifies this pair as
		// Different, meaning resolveColumnTypeSyncAction's
		// columnTypeBaseNameMatchesPrefix guard is what silences it. The other
		// rows are already Equal or Equivalent before that guard runs at all
		// (compareColumnTypes's own equal/normalized/base-name levels), so
		// they document the surrounding matrix without pinning the guard
		// itself - asserting this explicitly means a future change that
		// silently moves a row across that boundary fails loudly here
		// instead of the guard's coverage quietly shrinking.
		reachesGuard bool
	}{
		{"decimal_vs_sized_numeric_column", schemas.Decimal, 0, 10, true},
		{"sized_decimal_vs_sized_numeric_column", schemas.Decimal, 10, 10, false},
		{"numeric_vs_sized_numeric_column", schemas.Numeric, 0, 10, true},
		{"sized_numeric_vs_sized_numeric_column", schemas.Numeric, 10, 10, false},
		{"decimal_vs_bare_numeric_column", schemas.Decimal, 0, 0, false},
		{"numeric_vs_bare_numeric_column", schemas.Numeric, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect := &schemas.Column{Name: "price", SQLType: schemas.SQLType{Name: tt.expectedName}}
			if tt.expectedLength > 0 {
				expect.Length = tt.expectedLength
				expect.Length2 = 2
			}
			actual := &schemas.Column{Name: "price", SQLType: schemas.SQLType{Name: schemas.Numeric}}
			if tt.actualLength > 0 {
				actual.Length = tt.actualLength
				actual.Length2 = 2
			}

			comparison := dialect.CompareColumns(expect, actual)
			if comparison.Type.IsDifferent() != tt.reachesGuard {
				t.Fatalf("comparison.Type.IsDifferent() = %v, want %v (comparison.Type = %+v)",
					comparison.Type.IsDifferent(), tt.reachesGuard, comparison.Type)
			}
			if got := resolveColumnTypeSyncAction(dialect.Alias, features, comparison); got != columnTypeSyncActionNone {
				t.Fatalf("resolveColumnTypeSyncAction() = %v, want columnTypeSyncActionNone (comparison.Type = %+v)",
					got, comparison.Type)
			}
		})
	}
}

func TestBuildColumnSyncDecisionCommentIgnoredWhenTypeWarns(t *testing.T) {
	features := dialects.ColumnSyncFeatures{ColumnComment: true}
	comparison := typeDiffersComparison("INT", "BIGINT", 0, 0)
	comparison.Comment = dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"}

	decision := buildColumnSyncDecision(nil, features, comparison)

	if decision.typeAction != columnTypeSyncActionWarn {
		t.Fatalf("decision.typeAction = %v, want columnTypeSyncActionWarn", decision.typeAction)
	}
	if decision.modifyComment {
		t.Fatalf("decision.modifyComment = true, want false: comment sync must not run alongside a warn-only type mismatch")
	}
}

func TestBuildColumnSyncDecisionCommentIgnoredWhenTypeModifies(t *testing.T) {
	features := dialects.ColumnSyncFeatures{ColumnComment: true, TextFromVarchar: true}
	comparison := typeDiffersComparison(schemas.Text, "VARCHAR(100)", 0, 100)
	comparison.Comment = dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"}

	decision := buildColumnSyncDecision(nil, features, comparison)

	if decision.typeAction != columnTypeSyncActionModify {
		t.Fatalf("decision.typeAction = %v, want columnTypeSyncActionModify", decision.typeAction)
	}
	if decision.modifyComment {
		t.Fatalf("decision.modifyComment = true, want false: at most one ModifyColumnSQL must run per column")
	}
}

func TestBuildColumnSyncDecisionModifiesCommentWhenTypesMatch(t *testing.T) {
	features := dialects.ColumnSyncFeatures{ColumnComment: true}
	comparison := dialects.ColumnComparison{
		Type:     dialects.ColumnCompareField{Status: dialects.ColumnCompareEqual, Expected: "INT", Actual: "INT"},
		Comment:  dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"},
		Expected: &schemas.Column{Name: "col"},
		Actual:   &schemas.Column{Name: "col"},
	}

	decision := buildColumnSyncDecision(nil, features, comparison)

	if decision.typeAction != columnTypeSyncActionNone {
		t.Fatalf("decision.typeAction = %v, want columnTypeSyncActionNone", decision.typeAction)
	}
	if !decision.modifyComment {
		t.Fatalf("decision.modifyComment = false, want true when types already match and the dialect supports comment sync")
	}
}

func TestBuildColumnSyncDecisionIgnoresCommentWithoutFeature(t *testing.T) {
	features := dialects.ColumnSyncFeatures{ColumnComment: false}
	comparison := dialects.ColumnComparison{
		Type:     dialects.ColumnCompareField{Status: dialects.ColumnCompareEqual, Expected: "INT", Actual: "INT"},
		Comment:  dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"},
		Expected: &schemas.Column{Name: "col"},
		Actual:   &schemas.Column{Name: "col"},
	}

	decision := buildColumnSyncDecision(nil, features, comparison)

	if decision.modifyComment {
		t.Fatalf("decision.modifyComment = true, want false when the dialect does not support comment sync")
	}
}

// TestApplyColumnSyncDecisionWarnMessageFormats is a regression guard for
// v1's inconsistent Warnf format strings: the text-from-varchar warning
// ends with a trailing newline, while the general type-mismatch warning
// does not. Both actions must keep their exact original format string.
func TestApplyColumnSyncDecisionWarnMessageFormats(t *testing.T) {
	engine := newTestEngine(t)
	logger := &recordingSyncLogger{}
	engine.logger = logger
	session := engine.NewSession()
	defer session.Close()

	comparison := typeDiffersComparison("VARCHAR(100)", "VARCHAR(50)", 100, 50)

	textFromVarcharDecision := columnSyncDecision{typeAction: columnTypeSyncActionWarnTextFromVarchar}
	if err := applyColumnSyncDecision(session, "tbl", "tbl", comparison, textFromVarcharDecision); err != nil {
		t.Fatalf("applyColumnSyncDecision() error = %v", err)
	}

	genericWarnDecision := columnSyncDecision{typeAction: columnTypeSyncActionWarn}
	if err := applyColumnSyncDecision(session, "tbl", "tbl", comparison, genericWarnDecision); err != nil {
		t.Fatalf("applyColumnSyncDecision() error = %v", err)
	}

	if len(logger.warnfFormats) != 2 {
		t.Fatalf("len(logger.warnfFormats) = %d, want 2", len(logger.warnfFormats))
	}

	wantTextFromVarchar := "Table %s column %s db type is %s, struct type is %s\n"
	if logger.warnfFormats[0] != wantTextFromVarchar {
		t.Fatalf("text-from-varchar warn format = %q, want %q", logger.warnfFormats[0], wantTextFromVarchar)
	}

	wantGeneric := "Table %s column %s db type is %s, struct type is %s"
	if logger.warnfFormats[1] != wantGeneric {
		t.Fatalf("generic warn format = %q, want %q", logger.warnfFormats[1], wantGeneric)
	}
}

// TestBuildColumnSyncDecisionBareVarcharSuppressesCommentSync is a
// regression guard for v1's second outer-switch case,
// `case expectedType == schemas.Varchar`: when a struct column renders as
// a bare "VARCHAR" (unsized), that case matches even though it cannot
// widen anything, and v1's switch structure prevents the comment case
// below it from ever running. An unsized struct varchar with a comment
// (`xorm:"varchar comment('x')"`) against a sized db column is exactly
// this shape: the type comparison is Equivalent (level 4, "base sql type
// name"), not Different, so it does not go through
// resolveColumnTypeSyncAction at all.
func TestBuildColumnSyncDecisionBareVarcharSuppressesCommentSync(t *testing.T) {
	features := dialects.ColumnSyncFeatures{ColumnComment: true, VarcharLengthChange: true}
	comparison := dialects.ColumnComparison{
		Type: dialects.ColumnCompareField{
			Status:   dialects.ColumnCompareEquivalent,
			Expected: schemas.Varchar,
			Actual:   "VARCHAR(255)",
			Reason:   "base sql type name",
		},
		Comment:  dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"},
		Expected: &schemas.Column{Name: "col", Length: 0},
		Actual:   &schemas.Column{Name: "col", Length: 255},
	}

	decision := buildColumnSyncDecision(nil, features, comparison)

	if decision.typeAction != columnTypeSyncActionNone {
		t.Fatalf("decision.typeAction = %v, want columnTypeSyncActionNone", decision.typeAction)
	}
	if decision.modifyComment {
		t.Fatalf("decision.modifyComment = true, want false: the bare-VARCHAR arm must suppress comment sync")
	}
}

// TestBuildColumnSyncDecisionVarcharShrinkSuppressesCommentSync is the
// companion regression for v1's first outer-switch case: once the type
// comparison is Different, v1's outer switch commits to that case even
// when the inner switch performs no action (a shrink), so comment sync
// must stay unreachable there too.
func TestBuildColumnSyncDecisionVarcharShrinkSuppressesCommentSync(t *testing.T) {
	features := dialects.ColumnSyncFeatures{ColumnComment: true, VarcharLengthChange: true}
	comparison := typeDiffersComparison("VARCHAR(50)", "VARCHAR(100)", 50, 100)
	comparison.Comment = dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"}

	decision := buildColumnSyncDecision(nil, features, comparison)

	if decision.typeAction != columnTypeSyncActionNone {
		t.Fatalf("decision.typeAction = %v, want columnTypeSyncActionNone", decision.typeAction)
	}
	if decision.modifyComment {
		t.Fatalf("decision.modifyComment = true, want false: a varchar shrink must suppress comment sync")
	}
}

// TestApplyColumnSyncDecisionBareVarcharRunsNoSQL exercises the executed
// path, not just the decision struct: applyColumnSyncDecision must not
// call session.exec or log anything for the bare-VARCHAR-plus-differing-
// comment shape. session.exec would panic against this test engine's
// unconnected database, so a silent pass here is a genuine guarantee that
// no SQL was executed, not just an untested assumption.
func TestApplyColumnSyncDecisionBareVarcharRunsNoSQL(t *testing.T) {
	engine := newTestEngine(t)
	logger := &recordingSyncLogger{}
	engine.logger = logger
	session := engine.NewSession()
	defer session.Close()

	features := dialects.ColumnSyncFeatures{ColumnComment: true, VarcharLengthChange: true}
	comparison := dialects.ColumnComparison{
		Type: dialects.ColumnCompareField{
			Status:   dialects.ColumnCompareEquivalent,
			Expected: schemas.Varchar,
			Actual:   "VARCHAR(255)",
			Reason:   "base sql type name",
		},
		Comment:  dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"},
		Expected: &schemas.Column{Name: "col", Length: 0},
		Actual:   &schemas.Column{Name: "col", Length: 255},
	}
	decision := buildColumnSyncDecision(nil, features, comparison)

	if err := applyColumnSyncDecision(session, "tbl", "tbl", comparison, decision); err != nil {
		t.Fatalf("applyColumnSyncDecision() error = %v", err)
	}

	if len(logger.warnfFormats) != 0 {
		t.Fatalf("logger.warnfFormats = %v, want none", logger.warnfFormats)
	}
	if len(logger.infofFormats) != 0 {
		t.Fatalf("logger.infofFormats = %v, want none", logger.infofFormats)
	}
}

// TestApplyColumnSyncDecisionVarcharShrinkRunsNoSQL is the shrink-input
// companion of TestApplyColumnSyncDecisionBareVarcharRunsNoSQL, again
// exercising the executed path rather than only the decision struct.
func TestApplyColumnSyncDecisionVarcharShrinkRunsNoSQL(t *testing.T) {
	engine := newTestEngine(t)
	logger := &recordingSyncLogger{}
	engine.logger = logger
	session := engine.NewSession()
	defer session.Close()

	features := dialects.ColumnSyncFeatures{ColumnComment: true, VarcharLengthChange: true}
	comparison := typeDiffersComparison("VARCHAR(50)", "VARCHAR(100)", 50, 100)
	comparison.Comment = dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "new", Actual: "old"}
	decision := buildColumnSyncDecision(nil, features, comparison)

	if err := applyColumnSyncDecision(session, "tbl", "tbl", comparison, decision); err != nil {
		t.Fatalf("applyColumnSyncDecision() error = %v", err)
	}

	if len(logger.warnfFormats) != 0 {
		t.Fatalf("logger.warnfFormats = %v, want none", logger.warnfFormats)
	}
	if len(logger.infofFormats) != 0 {
		t.Fatalf("logger.infofFormats = %v, want none", logger.infofFormats)
	}
}

// TestApplyColumnSyncDecisionKeepsWarningsWhenExecFails is the Fix-2
// regression: v1 stored the exec result in `err` and only returned it
// after emitting the default/nullable warnings, so a failing ALTER still
// produced both warnings. Using a real sqlite3 database so the ALTER
// genuinely fails (sqlite has no MODIFY COLUMN syntax) instead of relying
// on a mocked exec.
func TestApplyColumnSyncDecisionKeepsWarningsWhenExecFails(t *testing.T) {
	engine, err := NewEngine("sqlite3", filepath.Join(t.TempDir(), "sync-exec-fail.db"))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	defer engine.Close()

	if _, err = engine.Exec("CREATE TABLE sync_exec_fail (col INTEGER)"); err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}

	logger := &recordingSyncLogger{}
	engine.SetLogger(logger)

	session := engine.NewSession()
	defer session.Close()

	comparison := dialects.ColumnComparison{
		Type:     dialects.ColumnCompareField{Status: dialects.ColumnCompareDifferent, Expected: "INT", Actual: "BIGINT"},
		Expected: &schemas.Column{Name: "col", Default: "1", Nullable: true},
		Actual:   &schemas.Column{Name: "col", Default: "2", Nullable: false},
	}
	decision := columnSyncDecision{
		typeAction:   columnTypeSyncActionModify,
		warnDefault:  true,
		warnNullable: true,
	}

	execErr := applyColumnSyncDecision(session, "sync_exec_fail", "sync_exec_fail", comparison, decision)
	if execErr == nil {
		t.Fatalf("applyColumnSyncDecision() error = nil, want an error from the failing ALTER")
	}

	if len(logger.infofFormats) != 1 {
		t.Fatalf("len(logger.infofFormats) = %d, want 1 (the type-change announcement)", len(logger.infofFormats))
	}
	if len(logger.warnfFormats) != 2 {
		t.Fatalf("len(logger.warnfFormats) = %d, want 2 (default and nullable)", len(logger.warnfFormats))
	}

	wantDefaultWarn := "Table %s Column %s db default is %s, struct default is %s"
	if logger.warnfFormats[0] != wantDefaultWarn {
		t.Fatalf("default warn format = %q, want %q", logger.warnfFormats[0], wantDefaultWarn)
	}

	wantNullableWarn := "Table %s Column %s db nullable is %v, struct nullable is %v"
	if logger.warnfFormats[1] != wantNullableWarn {
		t.Fatalf("nullable warn format = %q, want %q", logger.warnfFormats[1], wantNullableWarn)
	}
}
