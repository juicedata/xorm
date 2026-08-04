// Copyright 2023 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"strings"

	"xorm.io/xorm/dialects"
	"xorm.io/xorm/internal/utils"
	"xorm.io/xorm/schemas"
)

// columnTypeSyncAction is the action to take when a struct column's type
// does not render identically to the database column's type.
type columnTypeSyncAction int

const (
	// columnTypeSyncActionNone means the difference is silently accepted.
	columnTypeSyncActionNone columnTypeSyncAction = iota
	// columnTypeSyncActionWarn means a warning is logged but nothing changes.
	columnTypeSyncActionWarn
	// columnTypeSyncActionWarnTextFromVarchar means a warning is logged for
	// an unsupported text-from-varchar conversion. v1 logs this with a
	// trailing newline, unlike the plain columnTypeSyncActionWarn message,
	// so it needs its own action to reproduce that format exactly.
	columnTypeSyncActionWarnTextFromVarchar
	// columnTypeSyncActionModify means ModifyColumnSQL converts the column type.
	columnTypeSyncActionModify
	// columnTypeSyncActionModifyVarcharExpand means ModifyColumnSQL widens a varchar column.
	columnTypeSyncActionModifyVarcharExpand
)

// columnSyncDecision is what Sync should do for one column once its
// comparison against the database column is known.
type columnSyncDecision struct {
	typeAction    columnTypeSyncAction
	modifyComment bool
	// commentSkippedTypeMismatch is set when the comment differs and the
	// dialect supports comment sync, but resolveCommentSyncDecision
	// refused to run it because the rendered types are not byte-identical.
	// applyColumnSyncDecision logs this at Infof (not Warnf) so the
	// trade-off is visible instead of silent, without resurfacing the
	// warnings xorm/xorm#2583 and go-gitea/gitea#22275 removed: every row
	// this can fire on is ColumnCompareEquivalent, a pair
	// compareColumnTypes itself considers the same type (xorm/xorm#2591).
	commentSkippedTypeMismatch bool
	warnDefault                bool
	warnNullable               bool
}

// resolveColumnTypeSyncAction decides what to do about a type difference.
// It mirrors the inner switch of v1's Sync column loop:
//   - text-from-varchar always gets a decision (modify if the dialect
//     supports it, otherwise warn);
//   - varchar-to-varchar only modifies when the dialect supports widening
//     and the database column is actually shorter, and stays silent
//     otherwise (no warning for a shrink or an unsupported dialect);
//   - anything else warns, unless the actual type is exactly the expected
//     type's base name - resolved through the dialect's synonym alias in
//     either direction, so both "NUMERIC" and "DECIMAL" struct types match
//     a literal "NUMERIC(10,2)" column type the same way - followed by
//     "(...)", in which case it stays silent.
func resolveColumnTypeSyncAction(alias func(string) string, features dialects.ColumnSyncFeatures, comparison dialects.ColumnComparison) columnTypeSyncAction {
	if !comparison.Type.IsDifferent() {
		return columnTypeSyncActionNone
	}

	expectedType := comparison.Type.Expected
	actualType := comparison.Type.Actual

	switch {
	case expectedType == schemas.Text && strings.HasPrefix(actualType, schemas.Varchar):
		if features.TextFromVarchar {
			return columnTypeSyncActionModify
		}
		return columnTypeSyncActionWarnTextFromVarchar
	case isVarcharToVarchar(expectedType, actualType):
		if features.VarcharLengthChange && comparison.Actual.Length < comparison.Expected.Length {
			return columnTypeSyncActionModifyVarcharExpand
		}
		return columnTypeSyncActionNone
	default:
		if columnTypeBaseNameMatchesPrefix(alias, expectedType, actualType) {
			return columnTypeSyncActionNone
		}
		return columnTypeSyncActionWarn
	}
}

// isVarcharToVarchar reports whether both rendered types are varchar,
// checked as a raw string prefix rather than through compareColumnTypes's
// alias/normalisation logic: neither resolveColumnTypeSyncAction (above)
// nor resolveVarcharWidenSyncAction (below) needs anything past this,
// since no dialect aliases varchar to or from another type name.
func isVarcharToVarchar(expectedType, actualType string) bool {
	return strings.HasPrefix(actualType, schemas.Varchar) && strings.HasPrefix(expectedType, schemas.Varchar)
}

// columnTypeBaseNameMatchesPrefix reports whether actualType is exactly
// expectedType's base name, or a dialect synonym of it (checked in either
// direction so it does not matter which of two synonymous struct tags -
// e.g. "NUMERIC" or "DECIMAL" - the caller used), followed by a
// parenthesized length/precision suffix. expectedType itself must not carry
// its own suffix: an already-sized expected type (such as "DECIMAL(19,4)")
// is a genuine precision mismatch against a differently-sized actual type,
// not this bare-declaration case.
//
// This intentionally diverges from v1's byte-exact
// strings.HasPrefix(actualType, expectedType) && actualType[len(expectedType)] == '('
// check, which 20b43165 otherwise preserves byte-for-byte. An exhaustive
// sweep of all schemas.SqlTypes names on both sides, across four length
// shapes and mysql/postgres/mssql/sqlite3/oracle, found exactly one
// divergent class: a bare "DECIMAL" expected type against a sized
// "NUMERIC(...)" actual type, on mysql and postgres only (the only two
// dialects that alias "numeric" to "decimal" at all); every other
// combination, and every other dialect, matches v1 unchanged. That one
// class is a direct consequence of the numeric_precision/numeric_scale
// read-back fix earlier in this same commit: populating a real column's
// Length/Length2 is what first let a bare "DECIMAL" struct tag and a sized
// "NUMERIC(p,s)" column diverge from a bare "NUMERIC" struct tag against
// the same column, which v1's unaliased check already silenced.
func columnTypeBaseNameMatchesPrefix(alias func(string) string, expectedType, actualType string) bool {
	parenIdx := strings.Index(actualType, "(")
	if parenIdx <= 0 {
		return false
	}
	actualBase := actualType[:parenIdx]

	if strings.EqualFold(actualBase, expectedType) {
		return true
	}
	if alias == nil {
		return false
	}
	return strings.EqualFold(actualBase, alias(expectedType)) || strings.EqualFold(alias(actualBase), expectedType)
}

// resolveBareVarcharSyncAction handles v1's second outer-switch case,
// `case expectedType == schemas.Varchar`, which only fires once the first
// case (`!columnTypesMatch`, i.e. comparison.Type.IsDifferent()) is false.
// mysql/postgres/gbase8s only render a bare "VARCHAR" (no length suffix)
// when Length == 0, so `actual.Length < expected.Length` can never hold
// here and this arm can never actually widen a column - but v1 still
// dedicates an outer-switch case to it, and reaching that case, taken or
// not, prevents the next case (comment sync) from ever running for that
// column. It must be kept as its own branch so that exclusivity holds.
func resolveBareVarcharSyncAction(features dialects.ColumnSyncFeatures, comparison dialects.ColumnComparison) columnTypeSyncAction {
	if features.VarcharLengthChange && comparison.Actual.Length < comparison.Expected.Length {
		return columnTypeSyncActionModifyVarcharExpand
	}
	return columnTypeSyncActionNone
}

// resolveVarcharWidenSyncAction decides what to do once buildColumnSyncDecision
// has confirmed both rendered types are varchar and the type comparison
// already matches: compareColumnTypes's level 4 base-sql-type-name
// fallback reports any varchar/varchar pair as ColumnCompareEquivalent
// regardless of length - the same fallback that lets "NUMERIC" match a
// literal "DECIMAL(10,2)" column - so this is the only place a varchar
// length difference is visible to the sync policy in practice (see
// isVarcharToVarchar's case in resolveColumnTypeSyncAction above, which
// requires a genuine ColumnCompareDifferent and is therefore unreachable
// from a real dialect for a varchar/varchar pair).
//
// It widens only when the dialect supports it and the database column is
// both bounded and shorter than the struct wants. The bounded check
// matters: postgres reports an unbounded "character varying" column with
// Length == 0, and 0 < N would otherwise read as "shorter" and narrow an
// unbounded column into a bounded one - a silent capacity reduction, or a
// failing ALTER against data that no longer fits (xorm/xorm#2588 review).
// Otherwise it returns columnTypeSyncActionNone, never a warning, which is
// what keeps xorm/xorm#2583's suppression for a shrink or an unsupported
// dialect.
func resolveVarcharWidenSyncAction(features dialects.ColumnSyncFeatures, comparison dialects.ColumnComparison) columnTypeSyncAction {
	if features.VarcharLengthChange && comparison.Actual.Length > 0 && comparison.Actual.Length < comparison.Expected.Length {
		return columnTypeSyncActionModifyVarcharExpand
	}
	return columnTypeSyncActionNone
}

// resolveCommentSyncDecision is the single place that decides whether a
// differing comment should be synced. applyColumnSyncDecision's
// modifyComment branch runs a full ModifyColumnSQL using the column's
// entire rendered Expected definition, not just its comment, so syncing a
// comment when the rendered types are merely ColumnCompareEquivalent (or
// worse, ColumnCompareDifferent) silently rewrites the column's type as a
// side effect - see xorm/xorm#2591, where a real MySQL/MariaDB
// DECIMAL(19,4) column against a struct tagged DECIMAL(10,2) plus a
// comment change rounded already-stored data with Sync reporting success.
// Requiring ColumnCompareEqual makes ModifyColumnSQL's own type text a
// no-op, since it reissues the column's own already-matching rendered
// type.
//
// Every switch arm in buildColumnSyncDecision that might reach a comment
// sync MUST call this function rather than checking
// comparison.Comment.IsDifferent() directly - that is what makes the gate
// structurally hard to bypass: a second, ad hoc comment-sync site cannot
// reopen this hole as long as it goes through here instead of
// reimplementing the check.
//
// The cost is real and not marginal - an Equivalent-but-not-Equal pair
// with a differing comment no longer gets its comment synced, on any
// dialect and struct tag combination that lands in ColumnCompareEquivalent
// rather than ColumnCompareEqual. Three shapes are common enough to name:
//   - MariaDB 10.6 and MySQL <= 8.0.18 report a plain INT column as
//     "int(11)"; a bare `xorm:"INT"` struct tag never renders a width, so
//     every commented int-family column stops syncing its comment, on
//     every table, including ones xorm itself created.
//   - MySQL 8.0 (all versions, not just old ones) reports a TEXT column as
//     "text(65535)"; every commented TEXT/BLOB-family column has the same
//     problem there.
//   - PostgreSQL reports a column declared DECIMAL(p,s) back as
//     NUMERIC(p,s) (its own catalog normalizes the spelling); every
//     commented DECIMAL/NUMERIC column loses comment sync as a result.
//
// In all three cases the reissued ModifyColumnSQL type text would have
// been a provable no-op ("INT(11)" == "INT", "TEXT(65535)" == "TEXT",
// "DECIMAL(10,2)" == "NUMERIC(10,2)" for Sync's purposes) - the gate is
// deliberately conservative and blocks these harmless cases along with
// the genuinely dangerous ones (a real precision or length mismatch)
// because compareColumnTypes' Equivalent status does not distinguish
// "same type, different but harmless spelling" from "same base type,
// different and unsafe size". Losing a comment update is preferable to
// silently narrowing or rounding a column, so this trade is accepted as
// the current default; distinguishing the harmless subset would need a
// richer status than ColumnCompareEquivalent and is a possible follow-up,
// not attempted here.
func resolveCommentSyncDecision(features dialects.ColumnSyncFeatures, comparison dialects.ColumnComparison) (modifyComment, skippedTypeMismatch bool) {
	if !features.ColumnComment || !comparison.Comment.IsDifferent() {
		return false, false
	}
	if comparison.Type.Status != dialects.ColumnCompareEqual {
		return false, true
	}
	return true, false
}

// buildColumnSyncDecision turns a ColumnComparison into a columnSyncDecision.
// It mirrors v1's single outer switch, which has exactly one of four
// mutually exclusive outcomes for a column:
//  1. the type comparison differs (resolveColumnTypeSyncAction decides
//     what, if anything, to do);
//  2. the type comparison already matches and the expected type renders as
//     a bare "VARCHAR" (resolveBareVarcharSyncAction, effectively a no-op);
//  3. the type comparison already matches, both rendered types are
//     varchar (resolveVarcharWidenSyncAction decides what, if anything, to
//     do about the length; either way it then falls through to
//     resolveCommentSyncDecision, same as outcome 4);
//  4. the type comparison already matches, the expected type is not a bare
//     "VARCHAR" nor a varchar/varchar pair, and the comment differs
//     (resolveCommentSyncDecision decides whether to sync it - see that
//     function for why "already matches" here means ColumnCompareEqual,
//     not merely not-Different);
//  5. none of the above (nothing to do for the type/comment).
//
// Because Go's switch takes the first matching case, v1 could reach the
// comment case only through outcome 4, never through 1 or 2 - even when
// those arms performed no SQL and logged nothing. Comment sync must stay
// unreachable from 1 and 2 for the same reason, which is why it is a
// separate switch case here rather than a condition on the resolved
// typeAction.
//
// xorm/xorm#2588 added outcome 3: without it, a varchar/varchar pair with
// a length difference is always ColumnCompareEquivalent (level 4's
// base-sql-type-name fallback), never Different, so it silently fell into
// outcome 4 or 5 and Sync stopped widening varchar columns. Outcome 3 is
// NOT exclusivity-equivalent to outcome 1 (a genuine ColumnCompareDifferent
// shadows the comment case unconditionally): when
// resolveVarcharWidenSyncAction resolves to no action (a shrink, an
// unbounded actual column, or a dialect without VarcharLengthChange, such
// as gbase8s), outcome 3 falls through to resolveCommentSyncDecision
// instead of shadowing it - and that shared gate is what stops a P1
// regression found in review: an earlier revision of this fix synced the
// comment unconditionally on that fall-through, without checking whether
// the type comparison was ColumnCompareEqual. Since a varchar/varchar pair
// with a length difference is never Equal, that unconditional sync still
// ran ModifyColumnSQL with the struct's full (narrower) column definition,
// silently shrinking the actual column as a side effect of "just" syncing
// its comment - exactly the bug resolveCommentSyncDecision exists to
// prevent. Routing outcome 3's fall-through through the same function
// closes that hole structurally instead of relying on this arm
// remembering to re-check it.
//
// Outcome 3 must be checked AFTER outcome 2: a bare "VARCHAR" expected
// type against a sized actual varchar satisfies both this case's
// condition and outcome 2's, and outcome 2's no-op-that-still-shadows-
// comment-sync behavior must keep taking priority for that shape, exactly
// as before this fix.
//
// This is the third time this switch's exclusivity has mattered (see the
// #2585 and #2586 history above); the next person changing it should read
// this whole comment, not just the case they are touching.
func buildColumnSyncDecision(alias func(string) string, features dialects.ColumnSyncFeatures, comparison dialects.ColumnComparison) columnSyncDecision {
	decision := columnSyncDecision{
		warnDefault:  comparison.Default.IsDifferent(),
		warnNullable: comparison.Nullable.IsDifferent(),
	}

	switch {
	case comparison.Type.IsDifferent():
		decision.typeAction = resolveColumnTypeSyncAction(alias, features, comparison)
	case comparison.Type.Expected == schemas.Varchar:
		decision.typeAction = resolveBareVarcharSyncAction(features, comparison)
	case isVarcharToVarchar(comparison.Type.Expected, comparison.Type.Actual):
		decision.typeAction = resolveVarcharWidenSyncAction(features, comparison)
		if decision.typeAction == columnTypeSyncActionNone {
			decision.modifyComment, decision.commentSkippedTypeMismatch = resolveCommentSyncDecision(features, comparison)
		}
	case comparison.Comment.IsDifferent():
		decision.modifyComment, decision.commentSkippedTypeMismatch = resolveCommentSyncDecision(features, comparison)
	}

	return decision
}

// applyColumnSyncDecision executes at most one ModifyColumnSQL for the
// column, then logs the default/nullable warnings. It mirrors v1's
// structure of assigning any exec error to a shared local, always running
// the default/nullable checks, and only returning the error afterwards -
// so a failing ALTER still produces both warnings before Sync aborts.
func applyColumnSyncDecision(session *Session, tableName, tableNameWithSchema string, comparison dialects.ColumnComparison, decision columnSyncDecision) error {
	engine := session.engine
	expected := comparison.Expected
	actual := comparison.Actual

	var err error

	switch decision.typeAction {
	case columnTypeSyncActionModifyVarcharExpand:
		engine.logger.Infof("Table %s column %s change type from varchar(%d) to varchar(%d)\n",
			tableNameWithSchema, expected.Name, actual.Length, expected.Length)
		_, err = session.exec(engine.dialect.ModifyColumnSQL(tableNameWithSchema, expected))
	case columnTypeSyncActionModify:
		engine.logger.Infof("Table %s column %s change type from %s to %s\n",
			tableNameWithSchema, expected.Name, comparison.Type.Actual, comparison.Type.Expected)
		_, err = session.exec(engine.dialect.ModifyColumnSQL(tableNameWithSchema, expected))
	case columnTypeSyncActionWarn:
		engine.logger.Warnf("Table %s column %s db type is %s, struct type is %s",
			tableNameWithSchema, expected.Name, comparison.Type.Actual, comparison.Type.Expected)
	case columnTypeSyncActionWarnTextFromVarchar:
		engine.logger.Warnf("Table %s column %s db type is %s, struct type is %s\n",
			tableNameWithSchema, expected.Name, comparison.Type.Actual, comparison.Type.Expected)
	}

	if decision.modifyComment {
		_, err = session.exec(engine.dialect.ModifyColumnSQL(tableNameWithSchema, expected))
	} else if decision.commentSkippedTypeMismatch {
		engine.logger.Infof("Table %s column %s comment not synced because db type is %s, struct type is %s",
			tableNameWithSchema, expected.Name, comparison.Type.Actual, comparison.Type.Expected)
	}

	if decision.warnDefault {
		engine.logger.Warnf("Table %s Column %s db default is %s, struct default is %s",
			tableName, expected.Name, actual.Default, expected.Default)
	}

	if decision.warnNullable {
		engine.logger.Warnf("Table %s Column %s db nullable is %v, struct nullable is %v",
			tableName, expected.Name, actual.Nullable, expected.Nullable)
	}

	return err
}

type SyncOptions struct {
	WarnIfDatabaseColumnMissed bool
	// IgnoreConstrains will not add, delete or update unique constrains
	IgnoreConstrains bool
	// IgnoreIndices will not add or delete indices
	IgnoreIndices bool
	// IgnoreDropIndices will not delete indices
	IgnoreDropIndices bool
}

type SyncResult struct{}

func shouldSyncColumn(col *schemas.Column) bool {
	return col.MapType != schemas.ONLYFROMDB
}

func schemaTableForSync(table *schemas.Table) *schemas.Table {
	filtered := schemas.NewTable(table.Name, table.Type)
	filtered.StoreEngine = table.StoreEngine
	filtered.Charset = table.Charset
	filtered.Comment = table.Comment
	filtered.Collation = table.Collation

	includedColumns := make(map[string]struct{}, len(table.Columns()))
	for _, col := range table.Columns() {
		if !shouldSyncColumn(col) {
			continue
		}

		clone := *col
		clone.FieldIndex = append([]int(nil), col.FieldIndex...)
		clone.Indexes = make(map[string]int, len(col.Indexes))
		for name, idxType := range col.Indexes {
			clone.Indexes[name] = idxType
		}

		filtered.AddColumn(&clone)
		includedColumns[strings.ToLower(col.Name)] = struct{}{}
	}

	for _, index := range table.Indexes {
		keep := true
		for _, colName := range index.Cols {
			if _, ok := includedColumns[strings.ToLower(colName)]; !ok {
				keep = false
				break
			}
		}
		if !keep {
			continue
		}

		filtered.AddIndex(&schemas.Index{
			IsRegular: index.IsRegular,
			Name:      index.Name,
			Type:      index.Type,
			Cols:      append([]string(nil), index.Cols...),
		})
	}

	return filtered
}

// Sync the new struct changes to database, this method will automatically add
// table, column, index, unique. but will not delete or change anything.
// If you change some field, you should change the database manually.
func (engine *Engine) Sync(beans ...any) error {
	session := engine.NewSession()
	defer session.Close()
	return session.Sync(beans...)
}

// SyncWithOptions sync the database schemas according options and table structs
func (engine *Engine) SyncWithOptions(opts SyncOptions, beans ...any) (*SyncResult, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.SyncWithOptions(opts, beans...)
}

// Sync2 synchronize structs to database tables
// Depricated
func (engine *Engine) Sync2(beans ...any) error {
	return engine.Sync(beans...)
}

// Sync2 synchronize structs to database tables
// Depricated
func (session *Session) Sync2(beans ...any) error {
	return session.Sync(beans...)
}

// Sync synchronize structs to database tables
func (session *Session) Sync(beans ...any) error {
	_, err := session.SyncWithOptions(SyncOptions{
		WarnIfDatabaseColumnMissed: false,
		IgnoreConstrains:           false,
		IgnoreIndices:              false,
		IgnoreDropIndices:          false,
	}, beans...)
	return err
}

func (session *Session) SyncWithOptions(opts SyncOptions, beans ...any) (*SyncResult, error) {
	engine := session.engine

	if session.isAutoClose {
		session.isAutoClose = false
		defer session.Close()
	}

	tables, err := engine.dialect.GetTables(session.getQueryer(), session.ctx)
	if err != nil {
		return nil, err
	}

	session.autoResetStatement = false
	defer func() {
		session.autoResetStatement = true
		session.resetStatement()
	}()

	var syncResult SyncResult

	for _, bean := range beans {
		v := utils.ReflectValue(bean)
		table, err := engine.tagParser.ParseWithCache(v)
		if err != nil {
			return nil, err
		}
		syncTable := schemaTableForSync(table)
		var tbName string
		if len(session.statement.AltTableName) > 0 {
			tbName = session.statement.AltTableName
		} else {
			tbName = engine.TableName(bean)
		}
		tbNameWithSchema := engine.tbNameWithSchema(tbName)

		var oriTable *schemas.Table
		for _, tb := range tables {
			if strings.EqualFold(engine.tbNameWithSchema(tb.Name), engine.tbNameWithSchema(tbName)) {
				oriTable = tb
				break
			}
		}

		// this is a new table
		if oriTable == nil {
			syncTable.StoreEngine = session.statement.StoreEngine
			syncTable.Charset = session.statement.Charset
			session.statement.RefTable = syncTable
			session.statement.SetTableName(tbNameWithSchema)

			err = session.createCurrentTable()
			if err != nil {
				return nil, err
			}

			if !opts.IgnoreConstrains {
				err = session.createCurrentUniques()
				if err != nil {
					return nil, err
				}
			}

			if !opts.IgnoreIndices {
				err = session.createCurrentIndexes()
				if err != nil {
					return nil, err
				}
			}

			continue
		}

		// this will modify an old table
		if err = engine.loadTableInfo(session.ctx, oriTable); err != nil {
			return nil, err
		}

		// check columns
		for _, col := range syncTable.Columns() {
			var oriCol *schemas.Column
			for _, col2 := range oriTable.Columns() {
				if strings.EqualFold(col.Name, col2.Name) {
					oriCol = col2
					break
				}
			}

			// column is not exist on table
			if oriCol == nil {
				session.statement.RefTable = syncTable
				session.statement.SetTableName(tbNameWithSchema)
				if err = session.addColumn(col.Name); err != nil {
					return nil, err
				}
				continue
			}

			comparison := engine.dialect.CompareColumns(col, oriCol)
			decision := buildColumnSyncDecision(engine.dialect.Alias, engine.dialect.Features().ColumnSync, comparison)
			if err = applyColumnSyncDecision(session, tbName, tbNameWithSchema, comparison, decision); err != nil {
				return nil, err
			}
		}

		// indices found in orig table
		foundIndexNames := make(map[string]bool)
		// indices to be added
		addedNames := make(map[string]*schemas.Index)

		// drop indices that exist in orig and new table schema but are not equal
		for name, index := range syncTable.Indexes {
			var oriIndex *schemas.Index
			for name2, index2 := range oriTable.Indexes {
				if index.Equal(index2) {
					oriIndex = index2
					foundIndexNames[name2] = true
					break
				}
			}

			if oriIndex == nil {
				addedNames[name] = index
			}
		}

		// drop all indices that do not exist in new schema or have changed
		for name2, index2 := range oriTable.Indexes {
			if _, ok := foundIndexNames[name2]; !ok {
				// ignore based on there type
				if (index2.Type == schemas.IndexType && (opts.IgnoreIndices || opts.IgnoreDropIndices)) ||
					(index2.Type == schemas.UniqueType && opts.IgnoreConstrains) {
					// make sure we do not add a index with same name later
					delete(addedNames, name2)
					continue
				}

				sql := engine.dialect.DropIndexSQL(tbNameWithSchema, index2)
				_, err = session.exec(sql)
				if err != nil {
					return nil, err
				}
			}
		}

		// Add new indices because either they did not exist before or were dropped to update them
		for name, index := range addedNames {
			if index.Type == schemas.UniqueType && !opts.IgnoreConstrains {
				session.statement.RefTable = syncTable
				session.statement.SetTableName(tbNameWithSchema)
				err = session.addUnique(tbNameWithSchema, name)
			} else if index.Type == schemas.IndexType && !opts.IgnoreIndices {
				session.statement.RefTable = syncTable
				session.statement.SetTableName(tbNameWithSchema)
				err = session.addIndex(tbNameWithSchema, name)
			}
			if err != nil {
				return nil, err
			}
		}

		if opts.WarnIfDatabaseColumnMissed {
			// check all the columns which removed from struct fields but left on database tables.
			for _, colName := range oriTable.ColumnsSeq() {
				if table.GetColumn(colName) == nil {
					engine.logger.Warnf("Table %s has column %s but struct has not related field", engine.TableName(oriTable.Name, true), colName)
				}
			}
		}
	}

	return &syncResult, nil
}
