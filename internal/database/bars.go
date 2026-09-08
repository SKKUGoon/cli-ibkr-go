package database

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

type HistoryBar struct {
	Conid, TimestampMS     int64
	BarLengthSeconds       int32
	Open, High, Low, Close string
	Volume                 *int64
}

func jsonInteger(raw json.RawMessage) (int64, bool) {
	var number json.Number
	if len(raw) == 0 || raw[0] == '"' || string(raw) == "null" || json.Unmarshal(raw, &number) != nil {
		return 0, false
	}
	if integer, err := number.Int64(); err == nil {
		return integer, true
	}
	value, err := number.Float64()
	if err != nil || math.IsInf(value, 0) || value >= math.Exp2(63) || value < -math.Exp2(63) {
		return 0, false
	}
	return int64(value), true
}
func ParseHistoryBars(conid string, response json.RawMessage) ([]HistoryBar, error) {
	contract, err := strconv.ParseInt(conid, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid conid")
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal(response, &root); err != nil {
		return nil, err
	}
	seconds, ok := jsonInteger(root["barLength"])
	if !ok || seconds < math.MinInt32 || seconds > math.MaxInt32 {
		return nil, fmt.Errorf("history response missing integer barLength")
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal(root["data"], &rows) != nil || rows == nil {
		return nil, fmt.Errorf("history response missing data array")
	}
	bars := make([]HistoryBar, 0, len(rows))
	for index, row := range rows {
		timestamp, ok := jsonInteger(row["t"])
		if !ok {
			return nil, fmt.Errorf("history data[%d] missing numeric t", index)
		}
		bar := HistoryBar{Conid: contract, TimestampMS: timestamp, BarLengthSeconds: int32(seconds)}
		for _, field := range []struct {
			name   string
			target *string
		}{{"o", &bar.Open}, {"h", &bar.High}, {"l", &bar.Low}, {"c", &bar.Close}} {
			raw := row[field.name]
			var number json.Number
			if len(raw) == 0 || raw[0] == '"' || string(raw) == "null" || json.Unmarshal(raw, &number) != nil {
				return nil, fmt.Errorf("history data[%d] missing numeric %s", index, field.name)
			}
			*field.target = number.String()
		}
		if volume, ok := jsonInteger(row["v"]); ok {
			bar.Volume = &volume
		}
		bars = append(bars, bar)
	}
	return bars, nil
}
func DeduplicateBars(bars []HistoryBar) []HistoryBar {
	type key struct {
		conid, timestamp int64
		seconds          int32
	}
	seen := map[key]bool{}
	result := []HistoryBar{}
	for index := len(bars) - 1; index >= 0; index-- {
		bar := bars[index]
		identity := key{bar.Conid, bar.TimestampMS, bar.BarLengthSeconds}
		if !seen[identity] {
			seen[identity] = true
			result = append(result, bar)
		}
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}
