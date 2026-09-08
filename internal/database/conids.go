// Package database holds the optional personal warehouse integration, separate from the REST client.
package database

import (
	"context"
	"fmt"
	"strings"

	"ibkr-go/ibkr"
)

type AmbiguousConidError struct {
	Symbol string
	Count  int
}

func (err *AmbiguousConidError) Error() string {
	return fmt.Sprintf("%s matched %d active database rows", err.Symbol, err.Count)
}
func FindActiveConid(ctx context.Context, pool Connection, symbol string, exchange *string) (*ibkr.StockResult, error) {
	rows, err := pool.Query(ctx, `SELECT symbol, name_, conid, exchange FROM warehouse.conids
 WHERE upper(symbol) = $1 AND active = true
 AND ($2::text IS NULL OR exchange IS NOT NULL AND lower(exchange) = lower($2))
 ORDER BY symbol, conid`, strings.ToUpper(symbol), exchange)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result *ibkr.StockResult
	count := 0
	for rows.Next() {
		var row ibkr.StockResult
		if err = rows.Scan(&row.Symbol, &row.Name, &row.Conid, &row.Exchange); err != nil {
			return nil, err
		}
		result = &row
		count++
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if count > 1 {
		return nil, &AmbiguousConidError{strings.ToUpper(symbol), count}
	}
	return result, nil
}
func UpsertConid(ctx context.Context, pool Connection, stock ibkr.StockResult) (ibkr.StockResult, error) {
	var row ibkr.StockResult
	err := pool.QueryRow(ctx, `INSERT INTO warehouse.conids (symbol, name_, conid, exchange, active)
 VALUES ($1, $2, $3, $4, true) ON CONFLICT (conid) DO UPDATE
 SET symbol = EXCLUDED.symbol, name_ = EXCLUDED.name_, exchange = EXCLUDED.exchange,
 active = true, updated_at = now() RETURNING symbol, name_, conid, exchange`, stock.Symbol, stock.Name, stock.Conid, stock.Exchange).Scan(&row.Symbol, &row.Name, &row.Conid, &row.Exchange)
	return row, err
}
