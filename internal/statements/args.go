// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

type DateTimeString struct {
	Layout string
	Str    string
}

// Value implements the driver Valuer interface.
func (n DateTimeString) Value() (driver.Value, error) {
	return n.Str, nil
}

func (d DateTimeString) MarshalText() (text []byte, err error) {
	return []byte(d.Str), nil
}

func (n DateTimeString) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Str)
}

// WriteArg writes an arg
func (statement *Statement) WriteArg(w *builder.BytesWriter, arg any) error {
	switch argv := arg.(type) {
	case *builder.Builder:
		if _, err := w.WriteString("("); err != nil {
			return err
		}
		if err := argv.WriteTo(w); err != nil {
			return err
		}
		if _, err := w.WriteString(")"); err != nil {
			return err
		}
	case *DateTimeString:
		if statement.dialect.URI().DBType == schemas.ORACLE {
			if _, err := fmt.Fprintf(w, `TO_DATE(?,'%s')`, argv.Layout); err != nil {
				return err
			}
		} else {
			if err := w.WriteByte('?'); err != nil {
				return err
			}
		}
		w.Append(arg)
	default:
		if err := w.WriteByte('?'); err != nil {
			return err
		}
		if v, ok := arg.(bool); ok && statement.dialect.URI().DBType == schemas.MSSQL {
			if v {
				w.Append(1)
			} else {
				w.Append(0)
			}
		} else {
			w.Append(arg)
		}
	}
	return nil
}

// WriteArgs writes args
func (statement *Statement) WriteArgs(w *builder.BytesWriter, args []any) error {
	for i, arg := range args {
		if err := statement.WriteArg(w, arg); err != nil {
			return err
		}

		if i+1 != len(args) {
			if _, err := w.WriteString(","); err != nil {
				return err
			}
		}
	}
	return nil
}
