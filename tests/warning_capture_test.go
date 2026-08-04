// Copyright 2026 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"xorm.io/xorm/log"
)

// warningRecorder is a log.Logger that records every Infof/Warnf call
// instead of writing it anywhere, so tests can assert on Sync's warnings
// without scraping stdout.
type warningRecorder struct {
	mu           sync.Mutex
	entries      []string
	warnfEntries []string
	infofEntries []string
}

var _ log.Logger = (*warningRecorder)(nil)

func (r *warningRecorder) record(dst *[]string, format string, v ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	message := fmt.Sprintf(format, v...)
	r.entries = append(r.entries, message)
	*dst = append(*dst, message)
}

func (r *warningRecorder) Debug(v ...any)                 {}
func (r *warningRecorder) Debugf(format string, v ...any) {}
func (r *warningRecorder) Error(v ...any)                 {}
func (r *warningRecorder) Errorf(format string, v ...any) {}
func (r *warningRecorder) Info(v ...any)                  {}
func (r *warningRecorder) Infof(format string, v ...any)  { r.record(&r.infofEntries, format, v...) }
func (r *warningRecorder) Warn(v ...any)                  {}
func (r *warningRecorder) Warnf(format string, v ...any)  { r.record(&r.warnfEntries, format, v...) }
func (r *warningRecorder) Level() log.LogLevel            { return log.LOG_DEBUG }
func (r *warningRecorder) SetLevel(l log.LogLevel)        {}
func (r *warningRecorder) ShowSQL(show ...bool)           {}
func (r *warningRecorder) IsShowSQL() bool                { return false }

// messages returns a snapshot of every message recorded so far.
func (r *warningRecorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.entries))
	copy(out, r.entries)
	return out
}

// warnfMessages returns a snapshot of every message recorded via Warnf,
// so a test can assert that a shape it expects to be silenceable at
// Infof (never Warnf) really is - hasMessageContaining alone cannot tell
// the two levels apart.
func (r *warningRecorder) warnfMessages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.warnfEntries))
	copy(out, r.warnfEntries)
	return out
}

// infofMessages returns a snapshot of every message recorded via Infof.
func (r *warningRecorder) infofMessages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.infofEntries))
	copy(out, r.infofEntries)
	return out
}

// hasMessageContaining reports whether any recorded message contains substr.
func (r *warningRecorder) hasMessageContaining(substr string) bool {
	for _, m := range r.messages() {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

// warnfHasMessageContaining reports whether any Warnf-recorded message
// contains substr, so a test can pin that a particular notice logs at
// Infof and never escalates to Warnf.
func (r *warningRecorder) warnfHasMessageContaining(substr string) bool {
	for _, m := range r.warnfMessages() {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

// loggerGetter is implemented by *xorm.Engine, and promoted onto
// *xorm.EngineGroup through its embedded *xorm.Engine, so it lets
// captureWarnings save the logger that was installed before the test ran.
type loggerGetter interface {
	Logger() log.ContextLogger
}

// captureWarnings installs a warningRecorder as testEngine's logger for the
// duration of the calling test and restores the previous logger via
// t.Cleanup. It skips the test instead of failing when the active engine
// cannot report its current logger, since EngineInterface has no such
// getter and a future engine implementation might not support it.
func captureWarnings(t *testing.T) *warningRecorder {
	t.Helper()

	getter, ok := testEngine.(loggerGetter)
	if !ok {
		t.Skip("current engine does not expose its logger, cannot capture warnings")
	}

	previous := getter.Logger()
	recorder := &warningRecorder{}
	testEngine.SetLogger(recorder)
	t.Cleanup(func() {
		testEngine.SetLogger(previous)
	})

	return recorder
}
