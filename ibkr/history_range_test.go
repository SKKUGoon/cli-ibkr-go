package ibkr_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"ibkr-go/ibkr"
	"ibkr-go/internal/testsupport"
)

func TestHistoryDateBounds(t *testing.T) {
	for _, test := range []struct {
		name, start, end, zone string
		inclusive              bool
		hours                  float64
		invalid                bool
	}{
		{"inclusive same day", "2026-09-01", "2026-09-01", "UTC", true, 24, false},
		{"exclusive", "2026-09-01", "2026-09-03", "UTC", false, 48, false},
		{"spring DST", "2026-03-08", "2026-03-08", "America/New_York", true, 23, false},
		{"fall DST", "2026-11-01", "2026-11-01", "America/New_York", true, 25, false},
		{"empty exclusive", "2026-09-01", "2026-09-01", "UTC", false, 0, true},
		{"reversed", "2026-09-03", "2026-09-01", "UTC", true, 0, true},
		{"bad date", "2026-02-30", "2026-09-01", "UTC", true, 0, true},
		{"bad zone", "2026-09-01", "2026-09-02", "invalid", true, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := (ibkr.HistoryDateRange{StartDate: test.start, EndDate: test.end, Inclusive: test.inclusive, Timezone: test.zone}).Bounds()
			if (err != nil) != test.invalid {
				t.Fatal(err)
			}
			if !test.invalid && end.Sub(start).Hours() != test.hours {
				t.Fatal(start, end)
			}
		})
	}
}

func TestHistoryRangeMergesFiltersAndSorts(t *testing.T) {
	server := testsupport.NewServer(t)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	server.SetResponse(func(request testsupport.Request) (int, string) {
		if request.Query.Get("direction") != "-1" || request.Query.Get("period") != "8h" {
			t.Errorf("unexpected query: %v", request.Query)
		}
		return 200, fmt.Sprintf(`{"barLength":60,"volumeFactor":100,"data":[{"t":%d,"c":2},{"t":%d,"c":1},{"t":%d},{"t":%d}]}`, start.Add(time.Hour).UnixMilli(), start.UnixMilli(), start.Add(-time.Millisecond).UnixMilli(), start.Add(24*time.Hour).UnixMilli())
	})
	client, err := ibkr.NewClient(server.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	raw, err := client.FetchHistoryRange(context.Background(), ibkr.HistoryRequest{Conid: "1", Bar: "1min"}, ibkr.HistoryDateRange{StartDate: "2026-09-01", EndDate: "2026-09-01", Inclusive: true, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Data []struct {
			T int64 `json:"t"`
		}
		RequestCount int
		BarLength    int
		VolumeFactor int
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 2 || result.Data[0].T != start.UnixMilli() || result.Data[1].T != start.Add(time.Hour).UnixMilli() || result.RequestCount != 3 || result.BarLength != 60 || result.VolumeFactor != 100 {
		t.Fatal(string(raw))
	}
	requests := server.SnapshotRequests()
	for index, want := range []string{"20260902-00:00:00", "20260901-16:00:00", "20260901-08:00:00"} {
		if requests[index].Query.Get("startTime") != want {
			t.Fatal(requests)
		}
	}
}

func TestHistoryRangePropagatesFailures(t *testing.T) {
	for _, body := range []string{`{"error":"unavailable"}`, `{"data":[{"c":1}]}`, `{"data":null}`} {
		t.Run(body, func(t *testing.T) {
			server := testsupport.NewServer(t)
			server.SetResponse(func(testsupport.Request) (int, string) { return 200, body })
			client, err := ibkr.NewClient(server.Config)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			raw, err := client.FetchHistoryRange(context.Background(), ibkr.HistoryRequest{Conid: "1", Bar: "1d"}, ibkr.HistoryDateRange{StartDate: "2026-09-01", EndDate: "2026-09-01", Inclusive: true, Timezone: "UTC"})
			if err == nil || raw != nil {
				t.Fatal(string(raw), err)
			}
		})
	}
}

func TestHistoryRangeRejectsTruncatedAndPartialResults(t *testing.T) {
	for _, mode := range []string{"truncated", "later failure"} {
		t.Run(mode, func(t *testing.T) {
			server := testsupport.NewServer(t)
			count := 0
			server.SetResponse(func(testsupport.Request) (int, string) {
				count++
				if mode == "later failure" {
					if count == 2 {
						return 503, `{"error":"unavailable"}`
					}
					return 200, `{"data":[]}`
				}
				rows := make([]map[string]int64, 1000)
				for i := range rows {
					rows[i] = map[string]int64{"t": int64(i)}
				}
				raw, _ := json.Marshal(map[string]any{"data": rows})
				return 200, string(raw)
			})
			client, err := ibkr.NewClient(server.Config)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			raw, err := client.FetchHistoryRange(context.Background(), ibkr.HistoryRequest{Conid: "1", Bar: "1min"}, ibkr.HistoryDateRange{StartDate: "2026-09-01", EndDate: "2026-09-01", Inclusive: true, Timezone: "UTC"})
			if err == nil || raw != nil {
				t.Fatal(string(raw), err)
			}
		})
	}
}
