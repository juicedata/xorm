// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type (
	MyInt      int
	ZeroStruct struct{}
)

func TestZero(t *testing.T) {
	zeroValues := []any{
		int8(0),
		int16(0),
		int(0),
		int32(0),
		int64(0),
		uint8(0),
		uint16(0),
		uint(0),
		uint32(0),
		uint64(0),
		MyInt(0),
		reflect.ValueOf(0),
		nil,
		time.Time{},
		&time.Time{},
		nilTime,
		ZeroStruct{},
		&ZeroStruct{},
	}

	for _, v := range zeroValues {
		t.Run(fmt.Sprintf("%#v", v), func(t *testing.T) {
			assert.True(t, IsZero(v))
		})
	}
}

type PrimitivePtr struct {
	Int       *int
	Int8Ptr   *int8
	Int16Ptr  *int16
	Int32Ptr  *int32
	Int64Ptr  *int64
	UInt      *uint
	UInt8Ptr  *uint8
	UInt16Ptr *uint16
	UInt32Ptr *uint32
	UInt64Ptr *uint64
	StringPtr *string
}

type NonZeroStruct struct {
	PrimitivePtr *PrimitivePtr
}

func NewNonZeroStruct() NonZeroStruct {
	i := 1
	i8 := int8(1)
	i16 := int16(1)
	i32 := int32(1)
	i64 := int64(1)
	u := uint(1)
	u8 := uint8(1)
	u16 := uint16(1)
	u32 := uint32(1)
	u64 := uint64(1)
	s := "s"
	return NonZeroStruct{
		PrimitivePtr: &PrimitivePtr{
			Int:       &i,
			Int8Ptr:   &i8,
			Int16Ptr:  &i16,
			Int32Ptr:  &i32,
			Int64Ptr:  &i64,
			UInt:      &u,
			UInt8Ptr:  &u8,
			UInt16Ptr: &u16,
			UInt32Ptr: &u32,
			UInt64Ptr: &u64,
			StringPtr: &s,
		},
	}
}

func TestNoZero(t *testing.T) {
	now := time.Now()
	nonZeroStruct := NewNonZeroStruct()
	nonZeroValues := []any{
		int8(1),
		int16(1),
		int(1),
		int32(1),
		int64(1),
		uint8(1),
		uint16(1),
		uint(1),
		uint32(1),
		uint64(1),
		MyInt(1),
		reflect.ValueOf(1),
		now,
		&now,
		&nonZeroStruct,
		nonZeroStruct,
	}
	for _, v := range nonZeroValues {
		t.Run(fmt.Sprintf("%#v", v), func(t *testing.T) {
			assert.False(t, IsZero(v))
		})
	}
}

func TestIsValueZero(t *testing.T) {
	zeroReflectValues := []reflect.Value{
		reflect.ValueOf(int8(0)),
		reflect.ValueOf(int16(0)),
		reflect.ValueOf(int(0)),
		reflect.ValueOf(int32(0)),
		reflect.ValueOf(int64(0)),
		reflect.ValueOf(uint8(0)),
		reflect.ValueOf(uint16(0)),
		reflect.ValueOf(uint(0)),
		reflect.ValueOf(uint32(0)),
		reflect.ValueOf(uint64(0)),
		reflect.ValueOf(MyInt(0)),
		reflect.ValueOf(time.Time{}),
		reflect.ValueOf(&time.Time{}),
		reflect.ValueOf(nilTime),
		reflect.ValueOf(ZeroStruct{}),
		reflect.ValueOf(&ZeroStruct{}),
	}

	for _, v := range zeroReflectValues {
		t.Run(fmt.Sprintf("%#v", v), func(t *testing.T) {
			assert.True(t, IsValueZero(v))
		})
	}
}

func TestIsTimeZero(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)

	zeroTimes := []time.Time{
		{},
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1, 1, 1, 0, 0, 0, 0, shanghai),
		time.Date(1, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
	}
	for _, zeroTime := range zeroTimes {
		t.Run(zeroTime.String(), func(t *testing.T) {
			assert.True(t, IsTimeZero(zeroTime))
		})
	}

	nonZeroTimes := []time.Time{
		time.Date(1, 1, 2, 0, 0, 0, 0, shanghai),
		time.Date(1, 1, 1, 0, 0, 0, 1, shanghai),
		time.Date(2020, 10, 23, 10, 14, 15, 0, shanghai),
	}
	for _, nonZeroTime := range nonZeroTimes {
		t.Run(nonZeroTime.String(), func(t *testing.T) {
			assert.False(t, IsTimeZero(nonZeroTime))
		})
	}
}
