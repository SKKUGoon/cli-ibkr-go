package ibkr

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
	_ "time/tzdata"
)

type HistoryDateRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Inclusive bool   `json:"inclusive"`
	Timezone  string `json:"timezone"`
}

func (dates HistoryDateRange) Bounds() (time.Time, time.Time, error) {
	location, err := time.LoadLocation(dates.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid timezone: %w", err)
	}
	start, err := time.ParseInLocation("2006-01-02", dates.StartDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start-date must be YYYY-MM-DD: %w", err)
	}
	end, err := time.ParseInLocation("2006-01-02", dates.EndDate, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end-date must be YYYY-MM-DD: %w", err)
	}
	if dates.Inclusive {
		end = end.AddDate(0, 0, 1)
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("date range must contain at least one day")
	}
	return start, end, nil
}

type historyWindow struct {
	end    time.Time
	period string
}

func planHistoryWindows(request HistoryRequest, dates HistoryDateRange) ([]historyWindow, error) {
	start, end, err := dates.Bounds()
	if err != nil {
		return nil, err
	}
	if request.Period != "" || request.StartTime != nil {
		return nil, fmt.Errorf("date range cannot be combined with period or start-time")
	}
	var step time.Duration
	switch request.Bar {
	case "1min":
		step = 8 * time.Hour
	case "2min", "3min", "5min", "10min", "15min", "30min", "1h", "2h", "3h", "4h", "8h":
		step = 24 * time.Hour
	case "1d", "1w", "1m":
		step = 365 * 24 * time.Hour
	default:
		return nil, fmt.Errorf("unsupported date-range bar %q; use 1min through 8h, 1d, 1w, or 1m", request.Bar)
	}
	windows := []historyWindow{}
	for cursor := end; cursor.After(start); cursor = cursor.Add(-step) {
		period := "1d"
		if step == 8*time.Hour {
			period = "8h"
		}
		if step == 365*24*time.Hour {
			days := int(cursor.Sub(start).Hours()/24) + 1
			if days < 7 {
				days = 7
			}
			if request.Bar == "1m" && days < 31 {
				days = 31
			}
			if days > 365 {
				days = 365
			}
			period = fmt.Sprintf("%dd", days)
		}
		windows = append(windows, historyWindow{cursor, period})
	}
	return windows, nil
}

// FetchHistoryRange returns one sorted data array, retaining bar scale metadata for DB consumers.
func (client *Client) FetchHistoryRange(ctx context.Context, request HistoryRequest, dates HistoryDateRange) (json.RawMessage, error) {
	windows, err := planHistoryWindows(request, dates)
	if err != nil {
		return nil, err
	}
	start, end, _ := dates.Bounds()
	merged := map[string]json.RawMessage{}
	bars := map[int64]json.RawMessage{}
	for _, window := range windows {
		reference := window.end.UTC().Format("20060102-15:04:05")
		part := request
		part.StartTime, part.Period = &reference, window.period
		query := part.Query()
		query.Set("direction", "-1")
		raw, err := client.RequestJSON(ctx, "GET", HistoryPath, query, nil)
		if err != nil {
			return nil, fmt.Errorf("history ending %s: %w", reference, err)
		}
		var response map[string]json.RawMessage
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, err
		}
		var rows []json.RawMessage
		if err := json.Unmarshal(response["data"], &rows); err != nil || rows == nil {
			return nil, fmt.Errorf("history ending %s: response missing data array", reference)
		}
		if len(rows) >= 1000 {
			return nil, fmt.Errorf("history ending %s reached 1000 bars; refusing potentially truncated data", reference)
		}
		for _, key := range []string{"barLength", "priceFactor", "volumeFactor", "symbol", "text"} {
			if value, ok := response[key]; ok {
				if previous, exists := merged[key]; exists && string(previous) != string(value) {
					return nil, fmt.Errorf("history metadata %s changed between requests", key)
				}
				merged[key] = value
			}
		}
		for _, row := range rows {
			var bar struct {
				Timestamp *int64 `json:"t"`
			}
			if err := json.Unmarshal(row, &bar); err != nil || bar.Timestamp == nil {
				return nil, fmt.Errorf("history bar missing integer timestamp t")
			}
			timestamp := *bar.Timestamp
			if timestamp >= start.UnixMilli() && timestamp < end.UnixMilli() {
				if _, exists := bars[timestamp]; !exists {
					bars[timestamp] = row
				}
			}
		}
	}
	timestamps := make([]int64, 0, len(bars))
	for timestamp := range bars {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	rows := make([]json.RawMessage, 0, len(timestamps))
	for _, timestamp := range timestamps {
		rows = append(rows, bars[timestamp])
	}
	merged["data"], _ = json.Marshal(rows)
	merged["range"], _ = json.Marshal(dates)
	merged["conid"], _ = json.Marshal(request.Conid)
	merged["requestCount"], _ = json.Marshal(len(windows))
	return json.Marshal(merged)
}
