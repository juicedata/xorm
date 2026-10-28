// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestString2Time(t *testing.T) {
	expectedLoc, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)

	cases := map[string]time.Time{
		"2021-08-10":                          time.Date(2021, 8, 10, 8, 0, 0, 0, expectedLoc),
		"2021-07-11 10:44:00":                 time.Date(2021, 7, 11, 18, 44, 0, 0, expectedLoc),
		"2021-07-11 10:44:00.999":             time.Date(2021, 7, 11, 18, 44, 0, 999000000, expectedLoc),
		"2021-07-11 10:44:00.999999":          time.Date(2021, 7, 11, 18, 44, 0, 999999000, expectedLoc),
		"2021-07-11 10:44:00.999999999":       time.Date(2021, 7, 11, 18, 44, 0, 999999999, expectedLoc),
		"2021-06-06T22:58:20+08:00":           time.Date(2021, 6, 6, 22, 58, 20, 0, expectedLoc),
		"2021-06-06T22:58:20.999+08:00":       time.Date(2021, 6, 6, 22, 58, 20, 999000000, expectedLoc),
		"2021-06-06T22:58:20.999999+08:00":    time.Date(2021, 6, 6, 22, 58, 20, 999999000, expectedLoc),
		"2021-06-06T22:58:20.999999999+08:00": time.Date(2021, 6, 6, 22, 58, 20, 999999999, expectedLoc),
		"2021-08-10T10:33:04Z":                time.Date(2021, 8, 10, 18, 33, 0o4, 0, expectedLoc),
		"2021-08-10T10:33:04.999Z":            time.Date(2021, 8, 10, 18, 33, 0o4, 999000000, expectedLoc),
		"2021-08-10T10:33:04.999999Z":         time.Date(2021, 8, 10, 18, 33, 0o4, 999999000, expectedLoc),
		"2021-08-10T10:33:04.999999999Z":      time.Date(2021, 8, 10, 18, 33, 0o4, 999999999, expectedLoc),
		"10:22:33":                            time.Date(0, 1, 1, 18, 22, 33, 0, expectedLoc),
	}
	for layout, tm := range cases {
		t.Run(layout, func(t *testing.T) {
			target, err := String2Time(layout, time.UTC, expectedLoc)
			assert.NoError(t, err)
			assert.EqualValues(t, tm, *target)
		})
	}
}

func TestString2TimeUTCWallClock(t *testing.T) {
	dbLoc := time.FixedZone("UTC+9", 9*60*60)
	// A "Z" time is the driver's view of a timestamp without time zone,
	// with or without a fraction.
	cases := map[string]time.Time{
		"2021-08-10T10:33:04Z":     time.Date(2021, 8, 10, 1, 33, 4, 0, time.UTC),
		"2021-08-10T10:33:04.5Z":   time.Date(2021, 8, 10, 1, 33, 4, 500000000, time.UTC),
		"2021-08-10T10:33:04.123Z": time.Date(2021, 8, 10, 1, 33, 4, 123000000, time.UTC),
	}
	for s, tm := range cases {
		t.Run(s, func(t *testing.T) {
			target, err := String2Time(s, dbLoc, time.UTC)
			assert.NoError(t, err)
			assert.EqualValues(t, tm, *target)
		})
	}
}
