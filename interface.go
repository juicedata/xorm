// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"xorm.io/xorm/caches"
	"xorm.io/xorm/contexts"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/log"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

// Interface defines the interface which Engine, EngineGroup and Session will implementate.
type Interface interface {
	AllCols() *Session
	Alias(alias string) *Session
	Asc(colNames ...string) *Session
	BufferSize(size int) *Session
	Cols(columns ...string) *Session
	Count(...any) (int64, error)
	CreateIndexes(bean any) error
	CreateUniques(bean any) error
	Decr(column string, arg ...any) *Session
	Desc(...string) *Session
	Delete(...any) (int64, error)
	Truncate(...any) (int64, error)
	Distinct(columns ...string) *Session
	DropIndexes(bean any) error
	Exec(sqlOrArgs ...any) (sql.Result, error)
	Exist(bean ...any) (bool, error)
	Find(any, ...any) error
	FindAndCount(any, ...any) (int64, error)
	Get(...any) (bool, error)
	GroupBy(keys string) *Session
	ID(any) *Session
	In(string, ...any) *Session
	Incr(column string, arg ...any) *Session
	Insert(...any) (int64, error)
	InsertIgnore(...any) (int64, error)
	InsertOne(any) (int64, error)
	IsTableEmpty(bean any) (bool, error)
	IsTableExist(beanOrTableName any) (bool, error)
	Iterate(any, IterFunc) error
	Limit(int, ...int) *Session
	MustCols(columns ...string) *Session
	NoAutoCondition(...bool) *Session
	NotIn(string, ...any) *Session
	Nullable(...string) *Session
	Join(joinOperator string, tablename, condition any, args ...any) *Session
	Omit(columns ...string) *Session
	OrderBy(order any, args ...any) *Session
	Ping() error
	Query(sqlOrArgs ...any) (resultsSlice []map[string][]byte, err error)
	QueryInterface(sqlOrArgs ...any) ([]map[string]any, error)
	QueryString(sqlOrArgs ...any) ([]map[string]string, error)
	Rows(bean any) (*Rows, error)
	SetExpr(string, any) *Session
	Select(string) *Session
	SQL(any, ...any) *Session
	Sum(bean any, colName string) (float64, error)
	SumInt(bean any, colName string) (int64, error)
	Sums(bean any, colNames ...string) ([]float64, error)
	SumsInt(bean any, colNames ...string) ([]int64, error)
	Table(tableNameOrBean any) *Session
	Unscoped() *Session
	Update(bean any, condiBeans ...any) (int64, error)
	UseBool(...string) *Session
	Where(any, ...any) *Session
}

// EngineInterface defines the interface which Engine, EngineGroup will implementate.
type EngineInterface interface {
	Interface

	Before(func(any)) *Session
	After(func(any)) *Session
	Charset(charset string) *Session
	ClearCache(...any) error
	Context(context.Context) *Session
	CreateTables(...any) error
	DBMetas() ([]*schemas.Table, error)
	DBVersion() (*schemas.Version, error)
	Dialect() dialects.Dialect
	DriverName() string
	DropTables(...any) error
	DumpAllToFile(fp string, tp ...schemas.DBType) error
	GetCacher(string) caches.Cacher
	GetColumnMapper() names.Mapper
	GetDefaultCacher() caches.Cacher
	GetTableMapper() names.Mapper
	GetTZDatabase() *time.Location
	GetTZLocation() *time.Location
	ImportFile(fp string) ([]sql.Result, error)
	MapCacher(any, caches.Cacher) error
	NewSession() *Session
	NoAutoTime() *Session
	Prepare() *Session
	Quote(string) string
	SetCacher(string, caches.Cacher)
	SetConnMaxLifetime(time.Duration)
	SetColumnMapper(names.Mapper)
	SetTagIdentifier(string)
	SetDefaultCacher(caches.Cacher)
	SetLogger(logger any)
	SetLogLevel(log.LogLevel)
	SetMapper(names.Mapper)
	SetMaxOpenConns(int)
	SetMaxIdleConns(int)
	SetQuotePolicy(dialects.QuotePolicy)
	SetSchema(string)
	SetTableMapper(names.Mapper)
	SetTZDatabase(tz *time.Location)
	SetTZLocation(tz *time.Location)
	AddHook(hook contexts.Hook)
	ShowSQL(show ...bool)
	Sync(...any) error
	Sync2(...any) error
	SyncWithOptions(SyncOptions, ...any) (*SyncResult, error)
	StoreEngine(storeEngine string) *Session
	TableInfo(bean any) (*schemas.Table, error)
	TableName(any, ...bool) string
	UnMapType(reflect.Type)
	EnableSessionID(bool)
}

var (
	_ Interface       = &Session{}
	_ EngineInterface = &Engine{}
	_ EngineInterface = &EngineGroup{}
)
