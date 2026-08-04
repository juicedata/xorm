// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"math/big"
	"strings"

	"xorm.io/xorm/schemas"
)

// ColumnCompareStatus represents a comparison result for one column attribute.
type ColumnCompareStatus int

const (
	// ColumnCompareEqual means both sides render identically.
	ColumnCompareEqual ColumnCompareStatus = iota
	// ColumnCompareEquivalent means both sides render differently but are semantically equal.
	ColumnCompareEquivalent
	// ColumnCompareDifferent means both sides are semantically different.
	ColumnCompareDifferent
)

// ColumnCompareField stores the comparison result for one column attribute.
type ColumnCompareField struct {
	Status   ColumnCompareStatus
	Expected string
	Actual   string
	Reason   string
}

// IsDifferent reports whether the compared field is semantically different.
func (f ColumnCompareField) IsDifferent() bool {
	return f.Status == ColumnCompareDifferent
}

func newColumnCompareField(expected, actual string) ColumnCompareField {
	return ColumnCompareField{Expected: expected, Actual: actual}
}

func (f ColumnCompareField) withStatus(status ColumnCompareStatus, reason string) ColumnCompareField {
	f.Status = status
	f.Reason = reason
	return f
}

// ColumnComparison contains per-field comparison results for two columns.
type ColumnComparison struct {
	Type     ColumnCompareField
	Default  ColumnCompareField
	Nullable ColumnCompareField
	Comment  ColumnCompareField

	// Expected and Actual are shallow clones of the columns passed to
	// CompareColumns, taken after dialect.SQLType ran once on each of them.
	// SQLType has side effects on several dialects (mysql widens an unsized
	// Uuid to Varchar(40), postgres marks Serial columns non-nullable, and
	// so on), and the sync policy relies on those side effects being
	// visible to the length/default/nullable checks that follow the type
	// comparison. Callers should read from these clones instead of the
	// columns they passed in, so they see the same values the legacy code
	// observed without mutating caller-owned columns.
	Expected *schemas.Column
	Actual   *schemas.Column
}

var displayWidthNumericTypes = map[string]struct{}{
	schemas.BigInt:    {},
	schemas.Int:       {},
	schemas.Integer:   {},
	schemas.MediumInt: {},
	schemas.SmallInt:  {},
	schemas.TinyInt:   {},
}

// CompareColumns compares two columns without applying any sync policy
// decision. It never mutates expected or actual.
func (db *Base) CompareColumns(expected, actual *schemas.Column) ColumnComparison {
	expectedClone := cloneColumn(expected)
	actualClone := cloneColumn(actual)

	expectedType := db.dialect.SQLType(expectedClone)
	actualType := db.dialect.SQLType(actualClone)

	return ColumnComparison{
		Type:     compareColumnTypes(db.dialect.Alias, expectedClone, actualClone, expectedType, actualType),
		Default:  compareColumnDefaults(expectedClone, actualClone),
		Nullable: compareColumnNullable(expectedClone, actualClone),
		Comment:  compareColumnComment(expectedClone, actualClone),
		Expected: expectedClone,
		Actual:   actualClone,
	}
}

func cloneColumn(col *schemas.Column) *schemas.Column {
	cloned := *col
	return &cloned
}

func compareColumnTypes(alias func(string) string, expected, actual *schemas.Column, expectedType, actualType string) ColumnCompareField {
	if alias == nil {
		alias = func(columnType string) string {
			return columnType
		}
	}

	field := newColumnCompareField(expectedType, actualType)

	if expectedType == actualType {
		return field.withStatus(ColumnCompareEqual, "")
	}

	if columnUsesCompatibleJSONType(expected, actualType) || columnUsesCompatibleJSONType(actual, expectedType) {
		return field.withStatus(ColumnCompareEquivalent, "json semantic type")
	}

	if normalizeColumnTypeForComparison(alias, expectedType) == normalizeColumnTypeForComparison(alias, actualType) {
		return field.withStatus(ColumnCompareEquivalent, "normalized sql type")
	}

	// Alias both sides here, not just expectedType: a dialect's synonym map
	// only records one canonical direction (e.g. mssql/mysql/postgres all
	// alias "numeric" onto "decimal", never the reverse), so aliasing only
	// one side can turn a genuine match into a spurious mismatch whenever
	// the unaliased side is the one already holding the canonical name -
	// see xorm/xorm#2589's review. Aliasing must be idempotent here: it may
	// only ever turn a mismatch into a match, never the other way round.
	if strings.EqualFold(alias(schemas.SQLTypeName(actualType)), alias(schemas.SQLTypeName(expectedType))) {
		return field.withStatus(ColumnCompareEquivalent, "base sql type name")
	}

	return field.withStatus(ColumnCompareDifferent, "")
}

func compareColumnDefaults(expected, actual *schemas.Column) ColumnCompareField {
	field := newColumnCompareField(expected.Default, actual.Default)
	if expected.Default == actual.Default && expected.DefaultIsEmpty == actual.DefaultIsEmpty {
		return field.withStatus(ColumnCompareEqual, "")
	}

	if columnDefaultsMatch(expected, actual) {
		return field.withStatus(ColumnCompareEquivalent, "normalized default")
	}

	return field.withStatus(ColumnCompareDifferent, "")
}

func compareColumnNullable(expected, actual *schemas.Column) ColumnCompareField {
	field := newColumnCompareField(boolString(expected.Nullable), boolString(actual.Nullable))
	if expected.Nullable == actual.Nullable {
		return field.withStatus(ColumnCompareEqual, "")
	}

	return field.withStatus(ColumnCompareDifferent, "")
}

func compareColumnComment(expected, actual *schemas.Column) ColumnCompareField {
	field := newColumnCompareField(expected.Comment, actual.Comment)
	if expected.Comment == actual.Comment {
		return field.withStatus(ColumnCompareEqual, "")
	}

	return field.withStatus(ColumnCompareDifferent, "")
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func columnDefaultsMatch(expected, actual *schemas.Column) bool {
	switch {
	case expected.IsAutoIncrement:
		return true
	case expected.DefaultIsEmpty && actual.DefaultIsEmpty:
		return true
	case expected.DefaultIsEmpty:
		return nullableNullDefault(expected, actual)
	case actual.DefaultIsEmpty:
		return nullableNullDefault(actual, expected)
	}

	// This normalizes both sides using expected's SQLType even though actual
	// may have a different SQLType; that quirk predates this refactor and is
	// left unchanged.
	return normalizeColumnDefaultValue(expected.SQLType, expected.Default) ==
		normalizeColumnDefaultValue(expected.SQLType, actual.Default)
}

func nullableNullDefault(emptyDefaultCol, explicitDefaultCol *schemas.Column) bool {
	if !emptyDefaultCol.Nullable || !explicitDefaultCol.Nullable {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(explicitDefaultCol.Default), "NULL")
}

func normalizeColumnDefaultValue(sqlType schemas.SQLType, defaultValue string) string {
	normalized := strings.TrimSpace(defaultValue)
	if normalized == "" {
		return normalized
	}

	if sqlType.IsBool() {
		switch strings.ToLower(trimDefaultQuotes(normalized)) {
		case "1", "true":
			return "true"
		case "0", "false":
			return "false"
		}
	}

	if sqlType.IsNumeric() {
		if numeric, ok := normalizeNumericDefaultValue(normalized); ok {
			return numeric
		}
	}

	return normalized
}

func normalizeNumericDefaultValue(defaultValue string) (string, bool) {
	normalized := trimDefaultQuotes(defaultValue)
	rat, ok := new(big.Rat).SetString(normalized)
	if !ok {
		return "", false
	}

	return rat.RatString(), true
}

func trimDefaultQuotes(defaultValue string) string {
	if len(defaultValue) < 2 {
		return defaultValue
	}

	if (defaultValue[0] == '\'' && defaultValue[len(defaultValue)-1] == '\'') ||
		(defaultValue[0] == '"' && defaultValue[len(defaultValue)-1] == '"') {
		return defaultValue[1 : len(defaultValue)-1]
	}

	return defaultValue
}

func normalizeColumnTypeForComparison(alias func(string) string, columnType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(columnType))
	if normalized == "" {
		return normalized
	}

	unsignedSuffix := ""
	if strings.HasSuffix(normalized, " UNSIGNED") {
		unsignedSuffix = " UNSIGNED"
		normalized = strings.TrimSpace(strings.TrimSuffix(normalized, unsignedSuffix))
	}

	baseType := strings.ToUpper(strings.TrimSpace(schemas.SQLTypeName(normalized)))
	typeSuffix := strings.TrimPrefix(normalized, baseType)

	aliasedBaseType := baseType
	if alias != nil {
		aliasedBaseType = strings.ToUpper(alias(baseType))
	}

	if _, ok := displayWidthNumericTypes[baseType]; ok {
		normalized = aliasedBaseType
	} else {
		normalized = aliasedBaseType + typeSuffix
	}

	if unsignedSuffix != "" && !strings.HasSuffix(normalized, unsignedSuffix) {
		normalized += unsignedSuffix
	}

	return normalized
}

func columnUsesCompatibleJSONType(col *schemas.Column, currentType string) bool {
	if col == nil {
		return false
	}

	currentBaseType := strings.ToUpper(strings.TrimSpace(schemas.SQLTypeName(currentType)))
	switch {
	case col.IsJSONB:
		return currentBaseType == schemas.Jsonb
	case col.IsJSON:
		return currentBaseType == schemas.Json
	default:
		return false
	}
}
