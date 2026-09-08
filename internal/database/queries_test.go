package database

import (
	"context"
	"errors"
	"testing"

	"ibkr-go/ibkr"
)

func TestConidQueriesAndAmbiguity(t *testing.T) {
	pool := newQueryRecorder(t)
	pool.ExpectQuery(`SELECT symbol, name_, conid, exchange FROM warehouse.conids`).WithArgs("AAPL", (*string)(nil)).WillReturnRows(newRows([]string{"symbol", "name_", "conid", "exchange"}).AddRow("AAPL", "Apple", int64(1), "NASDAQ"))
	row, err := FindActiveConid(context.Background(), pool, "aapl", nil)
	if err != nil || row.Conid != 1 {
		t.Fatal(row, err)
	}
	pool.ExpectQuery(`SELECT symbol, name_, conid, exchange FROM warehouse.conids`).WithArgs("AAPL", (*string)(nil)).WillReturnRows(newRows([]string{"symbol", "name_", "conid", "exchange"}).AddRow("AAPL", "Apple", int64(1), "NASDAQ").AddRow("AAPL", "Apple", int64(2), "NYSE"))
	_, err = FindActiveConid(context.Background(), pool, "AAPL", nil)
	var ambiguous *AmbiguousConidError
	if !errors.As(err, &ambiguous) {
		t.Fatal(err)
	}
	stock := ibkr.StockResult{Symbol: "AAPL", Conid: 1}
	pool.ExpectQuery(`INSERT INTO warehouse.conids`).WithArgs("AAPL", (*string)(nil), int64(1), (*string)(nil)).WillReturnRows(newRows([]string{"symbol", "name_", "conid", "exchange"}).AddRow("AAPL", nil, int64(1), nil))
	_, err = UpsertConid(context.Background(), pool, stock)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestBarUpsertBatchPreservesLastDuplicate(t *testing.T) {
	pool := newQueryRecorder(t)
	bars := []HistoryBar{{Conid: 1, TimestampMS: 1000, BarLengthSeconds: 60, Open: "1", High: "2", Low: "0", Close: "1"}, {Conid: 1, TimestampMS: 1000, BarLengthSeconds: 60, Open: "2", High: "3", Low: "1", Close: "2"}}
	pool.ExpectExec(`INSERT INTO warehouse.ibkr_bars`).WithArgs([]int64{1}, []int64{1000}, []float64{60}, []string{"2"}, []string{"3"}, []string{"1"}, []string{"2"}, []*int64{nil}).WillReturnResult(int64(1))
	count, err := UpsertHistoryBars(context.Background(), pool, bars)
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
