// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

// EngineOptions holds engine level behavior switches. All of them are
// disabled by default to keep backward compatibility.
type EngineOptions struct {
	// RequireUpdateCond makes Update return ErrNeedUpdateCond when no
	// condition (Where, ID, condiBean ...) is given, instead of updating
	// every row of the table. A Limit also satisfies the requirement.
	RequireUpdateCond bool
}

// EngineOption modifies EngineOptions
type EngineOption func(*EngineOptions)

// WithRequireUpdateCond enables or disables the RequireUpdateCond option
func WithRequireUpdateCond(enable bool) EngineOption {
	return func(o *EngineOptions) {
		o.RequireUpdateCond = enable
	}
}

// SetOptions applies the options to the engine. New engine level switches only
// need a new EngineOption, no new Engine method is required.
// It should be called during initialization, before the engine is used
// concurrently.
func (engine *Engine) SetOptions(opts ...EngineOption) {
	for _, opt := range opts {
		opt(&engine.options)
	}
}

// Options returns a copy of the current engine options
func (engine *Engine) Options() EngineOptions {
	return engine.options
}
