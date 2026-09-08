package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"ibkr-go/internal/testsupport"
)

func TestHistoryFlagsAndInteractiveInputs(t *testing.T) {
	server := testsupport.NewServer(t)
	setupEnvironment(t, server)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	server.SetResponse(func(testsupport.Request) (int, string) {
		return 200, fmt.Sprintf(`{"barLength":86400,"data":[{"t":%d},{"t":%d},{"t":%d}]}`, start.UnixMilli(), start.Add(24*time.Hour).UnixMilli(), start.Add(48*time.Hour).UnixMilli())
	})
	for _, test := range []struct {
		name, input string
		args        []string
		count       int
		prompt      bool
	}{
		{"inclusive default", "", []string{"--conid", "1", "--start-date", "2026-09-01", "--end-date", "2026-09-02", "--bar", "1d"}, 2, false},
		{"exclusive and underscore", "", []string{"--conid", "1", "--start_date", "2026-09-01", "--end_date", "2026-09-02", "--inclusive=false", "--bar", "1d"}, 1, false},
		{"interactive", "1\n2026-09-01\n2026-09-02\n\n\n\n", nil, 2, true},
		{"partial flags", "2026-09-01\n2026-09-02\nfalse\n\n", []string{"--conid", "1", "--bar", "1d"}, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := NewRoot(strings.NewReader(test.input), &stdout, &stderr)
			command.SetArgs(append([]string{"fetch-history"}, test.args...))
			if err := command.Execute(); err != nil {
				t.Fatal(err, stderr.String())
			}
			var response struct{ Data []json.RawMessage }
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || len(response.Data) != test.count {
				t.Fatal(stdout.String(), err)
			}
			if strings.Contains(stderr.String(), "Include end date") != test.prompt {
				t.Fatal(stderr.String())
			}
		})
	}
}

func TestHistoryInvalidInputDoesNotRequestData(t *testing.T) {
	server := testsupport.NewServer(t)
	setupEnvironment(t, server)
	for _, args := range [][]string{
		{"--start-date", "2026-09-01", "--end-date", "2026-09-02", "--period", "1d"},
		{"--start-date", "2026-09-01", "--end-date", "2026-09-02", "--start-time", "x"},
		{"--start-date", "2026-09-02", "--end-date", "2026-09-01"},
		{"--start-date", "2026-09-01", "--end-date", "2026-09-01", "--inclusive=false"},
		{"--period", "1d", "--inclusive=false"},
	} {
		_, _, err := execute(append([]string{"fetch-history", "--conid", "1", "--bar", "1d"}, args...)...)
		if err == nil {
			t.Fatal(args)
		}
	}
	if len(server.SnapshotRequests()) != 0 || server.SessionCount() != 0 {
		t.Fatal("invalid inputs made network requests")
	}
}
