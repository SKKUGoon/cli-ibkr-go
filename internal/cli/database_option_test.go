package cli

import (
	"strings"
	"testing"

	"ibkr-go/internal/testsupport"
)

func TestDatabaseAccessRequiresExplicitFlag(t *testing.T) {
	server := testsupport.NewServer(t)
	setupEnvironment(t, server)
	t.Setenv("IBKR_DATABASE", "postgres://invalid:port")
	t.Setenv("IBKR_LOG", "warn")
	server.SetResponse(func(request testsupport.Request) (int, string) {
		if strings.HasSuffix(request.Path, "trsrv/stocks") {
			return 200, `{"AAPL":[{"contracts":[{"conid":1,"isUS":true}]}]}`
		}
		return 200, `{"barLength":86400,"data":[]}`
	})
	for _, args := range [][]string{
		{"stock-conid", "--symbol", "AAPL"},
		{"fetch-history", "--conid", "1", "--period", "1d", "--bar", "1min"},
		{"fetch-history", "--conid", "1", "--start-date", "2026-09-01", "--end-date", "2026-09-02", "--bar", "1d"},
	} {
		for _, flag := range []string{"", "--database=false", "--database"} {
			commandArgs := append([]string(nil), args...)
			if flag != "" {
				commandArgs = append(commandArgs, flag)
			}
			stdout, stderr, err := execute(commandArgs...)
			if err != nil || stdout == "" {
				t.Fatal(commandArgs, err)
			}
			contactedDatabase := strings.Contains(stderr, "IBKR_DATABASE is invalid")
			if contactedDatabase != (flag == "--database") {
				t.Fatal(commandArgs, stderr)
			}
		}
	}
}
