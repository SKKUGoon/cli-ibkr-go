package database

import (
	"context"
)

const upsertHistorySQL = `
INSERT INTO warehouse.ibkr_bars (
    conid, bar_start, bar_end, open, high, low, close, volume
)
SELECT
    u.conid,
    to_timestamp(u.ts_ms::double precision / 1000.0),
    to_timestamp(u.ts_ms::double precision / 1000.0) + make_interval(secs => u.bar_secs),
    u.open::numeric,
    u.high::numeric,
    u.low::numeric,
    u.close::numeric,
    u.volume
FROM UNNEST(
    $1::bigint[],
    $2::bigint[],
    $3::double precision[],
    $4::text[],
    $5::text[],
    $6::text[],
    $7::text[],
    $8::bigint[]
) AS u(conid, ts_ms, bar_secs, open, high, low, close, volume)
ON CONFLICT (conid, bar_start, bar_end) DO UPDATE
SET open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close,
    volume = EXCLUDED.volume
`

func UpsertHistoryBars(ctx context.Context, pool Connection, bars []HistoryBar) (int64, error) {
	if len(bars) == 0 {
		return 0, nil
	}
	unique := DeduplicateBars(bars)
	conids := []int64{}
	timestamps := []int64{}
	seconds := []float64{}
	opens := []string{}
	highs := []string{}
	lows := []string{}
	closes := []string{}
	volumes := []*int64{}
	for _, bar := range unique {
		conids = append(conids, bar.Conid)
		timestamps = append(timestamps, bar.TimestampMS)
		seconds = append(seconds, float64(bar.BarLengthSeconds))
		opens = append(opens, bar.Open)
		highs = append(highs, bar.High)
		lows = append(lows, bar.Low)
		closes = append(closes, bar.Close)
		volumes = append(volumes, bar.Volume)
	}
	result, err := pool.Exec(ctx, upsertHistorySQL, conids, timestamps, seconds, opens, highs, lows, closes, volumes)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
