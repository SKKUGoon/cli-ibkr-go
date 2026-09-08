package database

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// This test double embeds unused pgx methods so tests remain independent of pgx's expanding Rows interface.
type recordedRows struct {
	pgx.Rows
	values [][]any
	index  int
}

func newRows(_ []string) *recordedRows { return &recordedRows{index: -1} }
func (rows *recordedRows) AddRow(values ...any) *recordedRows {
	rows.values = append(rows.values, values)
	return rows
}
func (rows *recordedRows) Close()     {}
func (rows *recordedRows) Err() error { return nil }
func (rows *recordedRows) Next() bool { rows.index++; return rows.index < len(rows.values) }
func (rows *recordedRows) Scan(targets ...any) error {
	if rows.index < 0 || rows.index >= len(rows.values) {
		return fmt.Errorf("no current row")
	}
	for index, target := range targets {
		value := rows.values[rows.index][index]
		switch pointer := target.(type) {
		case *string:
			*pointer = value.(string)
		case *int64:
			*pointer = value.(int64)
		case **string:
			if value == nil {
				*pointer = nil
			} else {
				text := value.(string)
				*pointer = &text
			}
		default:
			return fmt.Errorf("unexpected scan target %T", target)
		}
	}
	return nil
}

type queryExpectation struct {
	sql   string
	args  []any
	rows  *recordedRows
	count int64
}

func (expectation *queryExpectation) WithArgs(args ...any) *queryExpectation {
	expectation.args = args
	return expectation
}
func (expectation *queryExpectation) WillReturnRows(rows *recordedRows) { expectation.rows = rows }
func (expectation *queryExpectation) WillReturnResult(count int64)      { expectation.count = count }

type queryRecorder struct {
	test    *testing.T
	pending []*queryExpectation
}

func newQueryRecorder(test *testing.T) *queryRecorder { return &queryRecorder{test: test} }
func (recorder *queryRecorder) ExpectQuery(sql string) *queryExpectation {
	expectation := &queryExpectation{sql: sql}
	recorder.pending = append(recorder.pending, expectation)
	return expectation
}
func (recorder *queryRecorder) ExpectExec(sql string) *queryExpectation {
	return recorder.ExpectQuery(sql)
}
func (recorder *queryRecorder) consume(sql string, args []any) *queryExpectation {
	recorder.test.Helper()
	if len(recorder.pending) == 0 {
		recorder.test.Fatal("unexpected query")
	}
	expected := recorder.pending[0]
	recorder.pending = recorder.pending[1:]
	if !strings.Contains(sql, expected.sql) || !reflect.DeepEqual(args, expected.args) {
		recorder.test.Fatalf("query %s arguments %#v; expected %s %#v", sql, args, expected.sql, expected.args)
	}
	return expected
}
func (recorder *queryRecorder) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	return recorder.consume(sql, args).rows, nil
}
func (recorder *queryRecorder) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	rows := recorder.consume(sql, args).rows
	rows.Next()
	return rows
}
func (recorder *queryRecorder) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(fmt.Sprintf("INSERT 0 %d", recorder.consume(sql, args).count)), nil
}
func (recorder *queryRecorder) ExpectationsWereMet() error {
	if len(recorder.pending) != 0 {
		return fmt.Errorf("%d queries not executed", len(recorder.pending))
	}
	return nil
}
