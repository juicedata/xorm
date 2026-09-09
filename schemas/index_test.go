// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schemas

import "testing"

func TestIndexEqualColumnOrder(t *testing.T) {
	base := &Index{Name: "first", Type: IndexType, Cols: []string{"user_id", "is_deleted", "created_unix"}}
	for _, tc := range []struct {
		name  string
		other *Index
		want  bool
	}{
		{"same order different name", &Index{Name: "second", Type: IndexType, Cols: []string{"user_id", "is_deleted", "created_unix"}}, true},
		{"different order", &Index{Type: IndexType, Cols: []string{"created_unix", "user_id", "is_deleted"}}, false},
		{"different type", &Index{Type: UniqueType, Cols: base.Cols}, false},
		{"different length", &Index{Type: IndexType, Cols: []string{"user_id"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.Equal(tc.other); got != tc.want {
				t.Fatalf("Equal() = %v, want %v", got, tc.want)
			}
			if got := tc.other.Equal(base); got != tc.want {
				t.Fatalf("reverse Equal() = %v, want %v", got, tc.want)
			}
		})
	}
}
