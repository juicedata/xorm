// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm/schemas"
)

type dialect struct {
	Dialect
	dbType schemas.DBType
}

func (d dialect) URI() *URI {
	return &URI{
		DBType: d.dbType,
	}
}

func TestFormatColumnTime(t *testing.T) {
	date := time.Date(2020, 10, 23, 10, 14, 15, 123456, time.Local)
	tests := []struct {
		name     string
		dialect  Dialect
		location *time.Location
		column   *schemas.Column
		time     time.Time
		wantRes  any
		wantErr  error
	}{
		{
			name:     "nullable",
			dialect:  nil,
			location: nil,
			column:   &schemas.Column{Nullable: true},
			time:     time.Time{},
			wantRes:  nil,
			wantErr:  nil,
		},
		{
			name:     "invalid sqltype",
			dialect:  nil,
			location: nil,
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Bit}},
			time:     time.Time{},
			wantRes:  0,
			wantErr:  nil,
		},
		{
			name:     "return default",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Bit}},
			time:     date,
			wantRes:  date,
			wantErr:  nil,
		},
		{
			name:     "return default (set timezone)",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Bit}, TimeZone: time.UTC},
			time:     date,
			wantRes:  date.In(time.UTC),
			wantErr:  nil,
		},
		{
			name:     "format date",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Date}},
			time:     date,
			wantRes:  date.Format("2006-01-02"),
			wantErr:  nil,
		},
		{
			name:     "format time",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Time}},
			time:     date,
			wantRes:  date.Format("15:04:05"),
			wantErr:  nil,
		},
		{
			name:     "format time (set length)",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Time}, Length: 64},
			time:     date,
			wantRes:  date.Format("15:04:05.999999999"),
			wantErr:  nil,
		},
		{
			name:     "format datetime",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.DateTime}},
			time:     date,
			wantRes:  date.Format("2006-01-02 15:04:05"),
			wantErr:  nil,
		},
		{
			name:     "format datetime (set length)",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.DateTime}, Length: 64},
			time:     date,
			wantRes:  date.Format("2006-01-02 15:04:05.999999999"),
			wantErr:  nil,
		},
		{
			name:     "format timestamp",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.TimeStamp}},
			time:     date,
			wantRes:  date.Format("2006-01-02 15:04:05"),
			wantErr:  nil,
		},
		{
			name:     "format timestamp (set length)",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.TimeStamp}, Length: 64},
			time:     date,
			wantRes:  date.Format("2006-01-02 15:04:05.999999999"),
			wantErr:  nil,
		},
		{
			name:     "format varchar",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Varchar}},
			time:     date,
			wantRes:  date.Format("2006-01-02 15:04:05"),
			wantErr:  nil,
		},
		{
			name:     "format timestampz",
			dialect:  dialect{},
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.TimeStampz}},
			time:     date,
			wantRes:  date.Format(time.RFC3339Nano),
			wantErr:  nil,
		},
		{
			name:     "format timestampz (mssql)",
			dialect:  dialect{dbType: schemas.MSSQL},
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.TimeStampz}},
			time:     date,
			wantRes:  date.Format("2006-01-02T15:04:05.9999999Z07:00"),
			wantErr:  nil,
		},
		{
			name:     "format int",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Int}},
			time:     date,
			wantRes:  date.Unix(),
			wantErr:  nil,
		},
		{
			name:     "format bigint",
			dialect:  nil,
			location: date.Location(),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.BigInt}},
			time:     date,
			wantRes:  date.Unix(),
			wantErr:  nil,
		},
		{
			name:     "format zero int in non-utc timezone",
			dialect:  nil,
			location: time.FixedZone("UTC+8", 8*60*60),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.Int}},
			time:     time.Date(1, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
			wantRes:  0,
			wantErr:  nil,
		},
		{
			name:     "format zero bigint in non-utc timezone",
			dialect:  nil,
			location: time.FixedZone("UTC+8", 8*60*60),
			column:   &schemas.Column{SQLType: schemas.SQLType{Name: schemas.BigInt}},
			time:     time.Date(1, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
			wantRes:  0,
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatColumnTime(tt.dialect, tt.location, tt.column, tt.time)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantRes, got)
		})
	}
}
