package dialects

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeqFilter(t *testing.T) {
	kases := map[string]string{
		"SELECT * FROM TABLE1 WHERE a=? AND b=?":                               "SELECT * FROM TABLE1 WHERE a=$1 AND b=$2",
		"SELECT 1, '???', '2006-01-02 15:04:05' FROM TABLE1 WHERE a=? AND b=?": "SELECT 1, '???', '2006-01-02 15:04:05' FROM TABLE1 WHERE a=$1 AND b=$2",
		"select '1''?' from issue":                                             "select '1''?' from issue",
		"select '1\\??' from issue":                                            "select '1\\??' from issue",
		"select '1\\\\',? from issue":                                          "select '1\\\\',$1 from issue",
		"select '1\\''?',? from issue":                                         "select '1\\''?',$1 from issue",
	}
	for sql, result := range kases {
		assert.EqualValues(t, result, postgresSeqFilterConvertQuestionMark(sql, "$", 1))
	}
}

func TestSeqFilterLineComment(t *testing.T) {
	kases := map[string]string{
		`SELECT *
		FROM TABLE1
		WHERE foo='bar'
		AND a=? -- it's a comment
		AND b=?`: `SELECT *
		FROM TABLE1
		WHERE foo='bar'
		AND a=$1 -- it's a comment
		AND b=$2`,
		`SELECT *
		FROM TABLE1
		WHERE foo='bar'
		AND a=? -- it's a comment?
		AND b=?`: `SELECT *
		FROM TABLE1
		WHERE foo='bar'
		AND a=$1 -- it's a comment?
		AND b=$2`,
		`SELECT *
		FROM TABLE1
		WHERE a=? -- it's a comment? and that's okay?
		AND b=?`: `SELECT *
		FROM TABLE1
		WHERE a=$1 -- it's a comment? and that's okay?
		AND b=$2`,
	}
	for sql, result := range kases {
		assert.EqualValues(t, result, postgresSeqFilterConvertQuestionMark(sql, "$", 1))
	}
}

func TestSeqFilterComment(t *testing.T) {
	kases := map[string]string{
		`SELECT *
		FROM TABLE1
		WHERE a=? /* it's a comment */
		AND b=?`: `SELECT *
		FROM TABLE1
		WHERE a=$1 /* it's a comment */
		AND b=$2`,
		`SELECT /* it's a comment * ?
		More comment on the next line! */ *
		FROM TABLE1
		WHERE a=? /**/
		AND b=?`: `SELECT /* it's a comment * ?
		More comment on the next line! */ *
		FROM TABLE1
		WHERE a=$1 /**/
		AND b=$2`,
	}
	for sql, result := range kases {
		assert.EqualValues(t, result, postgresSeqFilterConvertQuestionMark(sql, "$", 1))
	}
}

func TestSeqFilterPostgresQuestions(t *testing.T) {
	kases := map[string]string{
		`SELECT '{"a":1, "b":2}'::jsonb ? 'b'
		FROM table1 WHERE c = ?`: `SELECT '{"a":1, "b":2}'::jsonb ? 'b'
		FROM table1 WHERE c = $1`,
		`CREATE FUNCTION _example_function(table_name text, config jsonb)
		RETURNS jsonb
		AS $$
		BEGIN
			IF NOT config #> array['tables'] ? table_name THEN
				RETURN config;
			END IF;
		END;
		$$ LANGUAGE plpgsql;
		SELECT * FROM table1 WHERE c = ?`: `CREATE FUNCTION _example_function(table_name text, config jsonb)
		RETURNS jsonb
		AS $$
		BEGIN
			IF NOT config #> array['tables'] ? table_name THEN
				RETURN config;
			END IF;
		END;
		$$ LANGUAGE plpgsql;
		SELECT * FROM table1 WHERE c = $1`,
		`SELECT CASE WHEN a THEN 'null'::jsonb ELSE to_jsonb(?) END
		FROM table1 WHERE c = ?`: `SELECT CASE WHEN a THEN 'null'::jsonb ELSE to_jsonb($1) END
		FROM table1 WHERE c = $2`,
		`UPDATE table1 SET d = '{}'::jsonb || ? WHERE c = ?`: `UPDATE table1 SET d = '{}'::jsonb || $1 WHERE c = $2`,
		`SELECT d::jsonb  ? 'b' FROM table1 WHERE c = ?`:     `SELECT d::jsonb  ? 'b' FROM table1 WHERE c = $1`,
	}
	for sql, result := range kases {
		assert.EqualValues(t, result, postgresSeqFilterConvertQuestionMark(sql, "$", 1))
	}
}
