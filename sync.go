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
	warnDefault   bool
	warnNullable  bool
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
	case strings.HasPrefix(actualType, schemas.Varchar) && strings.HasPrefix(expectedType, schemas.Varchar):
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

// buildColumnSyncDecision turns a ColumnComparison into a columnSyncDecision.
// It mirrors v1's single outer switch, which has exactly one of four
// mutually exclusive outcomes for a column:
//  1. the type comparison differs (resolveColumnTypeSyncAction decides
//     what, if anything, to do);
//  2. the type comparison already matches and the expected type renders as
//     a bare "VARCHAR" (resolveBareVarcharSyncAction, effectively a no-op);
//  3. the type comparison already matches, the expected type is not a bare
//     "VARCHAR", and the comment differs (sync the comment);
//  4. none of the above (nothing to do for the type/comment).
//
// Because Go's switch takes the first matching case, v1 could reach the
// comment case only through outcome 3, never through 1 or 2 - even when
// those arms performed no SQL and logged nothing. Comment sync must stay
// unreachable from 1 and 2 for the same reason, which is why it is a
// separate switch case here rather than a condition on the resolved
// typeAction.
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
	case comparison.Comment.IsDifferent():
		decision.modifyComment = features.ColumnComment
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
