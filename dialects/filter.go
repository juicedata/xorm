// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Filter is an interface to filter SQL
type Filter interface {
	Do(ctx context.Context, sql string) string
}

// postgresSeqFilter filter SQL replace ?, ? ... to $1, $2 ...
type postgresSeqFilter struct {
	Prefix string
	Start  int
}

func postgresSeqFilterConvertQuestionMark(sql, prefix string, start int) string {
	var buf strings.Builder
	var beginSingleQuote bool
	var isLineComment bool
	var isComment bool
	var isMaybeLineComment bool
	var isMaybeComment bool
	var isMaybeCommentEnd bool
	var isMaybeJsonbQuestion bool
	var inDollarQuote bool
	var dollarQuoteTag string
	index := start
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if inDollarQuote {
			if c == '$' && strings.HasPrefix(sql[i:], dollarQuoteTag) {
				buf.WriteString(dollarQuoteTag)
				i += len(dollarQuoteTag) - 1
				inDollarQuote = false
				continue
			}
			buf.WriteByte(c)
			continue
		}
		// Only a "?" right after "::jsonb " is the jsonb operator.
		if isMaybeJsonbQuestion && c != ' ' && c != '?' {
			isMaybeJsonbQuestion = false
		}
		if !beginSingleQuote && !isLineComment && !isComment && !isMaybeJsonbQuestion && c == '?' {
			buf.WriteString(fmt.Sprintf("%s%v", prefix, index))
			index++
			continue
		}
		switch {
		case isMaybeJsonbQuestion && c == '?':
			isMaybeJsonbQuestion = false
		case isMaybeLineComment:
			if c == '-' {
				isLineComment = true
			}
			isMaybeLineComment = false
		case isMaybeComment:
			if c == '*' {
				isComment = true
			}
			isMaybeComment = false
		case isMaybeCommentEnd:
			if c == '/' {
				isComment = false
			}
			isMaybeCommentEnd = false
		case isLineComment:
			if c == '\n' {
				isLineComment = false
			}
		case isComment:
			if c == '*' {
				isMaybeCommentEnd = true
			}
		case !beginSingleQuote && c == '-':
			isMaybeLineComment = true
		case !beginSingleQuote && c == '/':
			isMaybeComment = true
		case !beginSingleQuote && c == ' ' && i >= 7 && strings.TrimSpace(sql[i-7:i]) == "::jsonb":
			isMaybeJsonbQuestion = true
		case !beginSingleQuote && c == '$':
			if tag, ok := extractDollarQuoteTag(sql, i); ok {
				buf.WriteString(tag)
				i += len(tag) - 1
				inDollarQuote = true
				dollarQuoteTag = tag
				continue
			}
		case c == '\'':
			beginSingleQuote = !beginSingleQuote
		}
		buf.WriteByte(c)
	}
	return buf.String()
}

func extractDollarQuoteTag(sql string, start int) (string, bool) {
	if start >= len(sql) || sql[start] != '$' {
		return "", false
	}
	for i := start + 1; i < len(sql); i++ {
		switch {
		case sql[i] == '$':
			return sql[start : i+1], true
		case !isDollarQuoteTagChar(sql[i]):
			return "", false
		}
	}
	return "", false
}

func isDollarQuoteTagChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_'
}

// Do implements Filter
func (s *postgresSeqFilter) Do(ctx context.Context, sql string) string {
	return postgresSeqFilterConvertQuestionMark(sql, s.Prefix, s.Start)
}

// oracleSeqFilter filter SQL replace ?, ? ... to :1, :2 ...
type oracleSeqFilter struct {
	Prefix string
	Start  int
}

func oracleSeqFilterConvertQuestionMark(sql, prefix string, start int) string {
	var buf strings.Builder
	var beginSingleQuote bool
	var isLineComment bool
	var isComment bool
	var isMaybeLineComment bool
	var isMaybeComment bool
	var isMaybeCommentEnd bool
	index := start
	for _, c := range sql {
		if !beginSingleQuote && !isLineComment && !isComment && c == '?' {
			buf.WriteString(prefix)
			buf.WriteString(strconv.Itoa(index))
			index++
		} else {
			switch {
			case isMaybeLineComment:
				if c == '-' {
					isLineComment = true
				}
				isMaybeLineComment = false
			case isMaybeComment:
				if c == '*' {
					isComment = true
				}
				isMaybeComment = false
			case isMaybeCommentEnd:
				if c == '/' {
					isComment = false
				}
				isMaybeCommentEnd = false
			case isLineComment:
				if c == '\n' {
					isLineComment = false
				}
			case isComment:
				if c == '*' {
					isMaybeCommentEnd = true
				}
			case !beginSingleQuote && c == '-':
				isMaybeLineComment = true
			case !beginSingleQuote && c == '/':
				isMaybeComment = true
			case c == '\'':
				beginSingleQuote = !beginSingleQuote
			}
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

// Do implements Filter
func (s *oracleSeqFilter) Do(ctx context.Context, sql string) string {
	return oracleSeqFilterConvertQuestionMark(sql, s.Prefix, s.Start)
}
