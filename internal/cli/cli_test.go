package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ibkr-go/internal/config"
	"ibkr-go/internal/testsupport"
)

func setupEnvironment(t *testing.T, server *testsupport.Server) {
	t.Helper()
	for _, key := range config.ReportKeys {
		t.Setenv(key, "")
	}
	for key, value := range server.Environment() {
		t.Setenv(key, value)
	}
	t.Setenv("IBKR_LST_REFRESH_SKEW_SECONDS", "60")
	t.Setenv("IBKR_LST_LOCK_TTL_SECONDS", "15")
	t.Setenv("IBKR_REDIS_KEY_PREFIX", "ibkr:oauth:lst")
}
func execute(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	command := NewRoot(strings.NewReader(""), &stdout, &stderr)
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String(), stderr.String(), err
}
func TestRESTCommandSurface(t *testing.T) {
	server := testsupport.NewServer(t)
	setupEnvironment(t, server)
	server.SetResponse(func(request testsupport.Request) (int, string) {
		if strings.HasSuffix(request.Path, "trsrv/stocks") {
			return 200, `{"AAPL":[{"name":"Apple","contracts":[{"conid":265598,"exchange":"NASDAQ","isUS":true}]}]}`
		}
		if strings.Contains(request.Path, "/orders") || strings.Contains(request.Path, "/order/") || strings.Contains(request.Path, "/reply/") {
			return 200, `[{"order_id":"42"}]`
		}
		return 200, `{"ok":true}`
	})
	tests := []struct {
		args, method, path string
		query              map[string]string
		body               string
	}{
		{"auth-status", "POST", "iserver/auth/status", nil, ""},
		{"init-session", "POST", "iserver/auth/ssodh/init", nil, `{"publish":true,"compete":true}`},
		{"init-session --compete=false", "POST", "iserver/auth/ssodh/init", nil, `{"publish":true,"compete":false}`},
		{"tickle", "POST", "tickle", nil, ""},
		{"accounts", "GET", "portfolio/accounts", nil, ""},
		{"brokerage-accounts", "GET", "iserver/accounts", nil, ""},
		{"account-pnl", "GET", "iserver/account/pnl/partitioned", nil, ""},
		{"account-summary --account-id DU1", "GET", "iserver/account/DU1/summary", nil, ""},
		{"portfolio-summary --account-id DU1", "GET", "portfolio/DU1/summary", nil, ""},
		{"ledger --account-id DU1", "GET", "portfolio/DU1/ledger", nil, ""},
		{"positions --account-id DU1 --page 2", "GET", "portfolio/DU1/positions/2", nil, ""},
		{"positions-live --account-id DU1 --model test --sort position --direction d", "GET", "portfolio2/DU1/positions", map[string]string{"model": "test", "sort": "position", "direction": "d"}, ""},
		{"trades --account-id DU1 --days 7", "GET", "iserver/account/trades/", map[string]string{"accountId": "DU1", "days": "7"}, ""},
		{"live-orders --account-id DU1", "GET", "iserver/account/orders", map[string]string{"accountId": "DU1", "force": "true"}, ""},
		{"live-orders --account-id DU1 --force=false", "GET", "iserver/account/orders", map[string]string{"accountId": "DU1", "force": "false"}, ""},
		{"stock-conid --symbol aapl --exchange NASDAQ --default-filtering=false", "GET", "trsrv/stocks", map[string]string{"symbols": "AAPL"}, ""},
		{"fetch-history --conid 1 --period 1d --bar 1min --outside-rth=false --exchange NYSE --start-time test", "GET", "iserver/marketdata/history", map[string]string{"conid": "1", "period": "1d", "bar": "1min", "outsideRth": "false", "exchange": "NYSE", "startTime": "test"}, ""},
		{`order place --account-id DU1 --orders-json {"conid":1,"order_type":"LMT"}`, "POST", "iserver/account/DU1/orders", nil, `{"orders":[{"conid":1,"orderType":"LMT"}]}`},
		{`order whatif --account-id DU1 --orders-json [{"conid":1}]`, "POST", "iserver/account/DU1/orders/whatif", nil, `{"orders":[{"conid":1}]}`},
		{`order modify --account-id DU1 --order-id 42 --order-json {"price":10}`, "POST", "iserver/account/DU1/order/42", nil, `{"price":10}`},
		{"order cancel --account-id DU1 --order-id 42", "DELETE", "iserver/account/DU1/order/42", nil, ""},
		{"order status --order-id 42", "GET", "iserver/account/order/status/42", nil, ""},
		{"order reply --reply-id 123 --confirmed", "POST", "iserver/reply/123", nil, `{"confirmed":true}`},
		{"order algos --conid 1 --algo Adaptive --algo Vwap --add-description --add-params", "GET", "iserver/contract/1/algos", map[string]string{"algos": "Adaptive;Vwap", "addDescription": "1", "addParams": "1"}, ""},
	}
	for _, test := range tests {
		t.Run(test.args, func(t *testing.T) {
			before := len(server.SnapshotRequests())
			stdout, _, err := execute(strings.Fields(test.args)...)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid([]byte(stdout)) {
				t.Fatalf("non-JSON stdout: %s", stdout)
			}
			if len(server.SnapshotRequests()) != before+1 {
				t.Fatal("wrong request count")
			}
			request := server.SnapshotRequests()[before]
			if request.Method != test.method || request.Path != "/v1/api/"+test.path {
				t.Fatal(request)
			}
			if len(request.Query) != len(test.query) {
				t.Fatal(request.Query, test.query)
			}
			for key, value := range test.query {
				if request.Query.Get(key) != value {
					t.Fatal(request.Query)
				}
			}
			if test.body != "" {
				var actual, expected any
				_ = json.Unmarshal(request.Body, &actual)
				_ = json.Unmarshal([]byte(test.body), &expected)
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("body %s expected %s", request.Body, test.body)
				}
			} else if len(request.Body) != 0 {
				t.Fatal(string(request.Body))
			}

			if binary := os.Getenv("IBKR_RUST_REFERENCE_BINARY"); binary != "" && test.args != "init-session --compete=false" && !strings.Contains(test.args, "--force=false") {
				rustCommand := exec.Command(binary, strings.Fields(test.args)...)
				rustCommand.Dir = t.TempDir()
				var rustStdout, rustStderr bytes.Buffer
				rustCommand.Stdout = &rustStdout
				rustCommand.Stderr = &rustStderr
				if err := rustCommand.Run(); err != nil {
					t.Fatalf("Rust reference: %v: %s", err, rustStderr.String())
				}
				var goJSON, rustJSON any
				if err := json.Unmarshal([]byte(stdout), &goJSON); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(rustStdout.Bytes(), &rustJSON); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(goJSON, rustJSON) {
					t.Fatalf("Go output %s; Rust output %s", stdout, rustStdout.String())
				}
				rustRequest := server.SnapshotRequests()[len(server.SnapshotRequests())-1]
				var goBody, rustBody any
				if len(request.Body) > 0 {
					_ = json.Unmarshal(request.Body, &goBody)
				}
				if len(rustRequest.Body) > 0 {
					_ = json.Unmarshal(rustRequest.Body, &rustBody)
				}
				if request.Method != rustRequest.Method || request.Path != rustRequest.Path || !reflect.DeepEqual(request.Query, rustRequest.Query) || !reflect.DeepEqual(goBody, rustBody) {
					t.Fatalf("Go request %+v; Rust request %+v", request, rustRequest)
				}
			}
		})
	}
}
func TestLocalCommandsOutputAndValidation(t *testing.T) {
	stdout, stderr, err := execute("--version")
	if err != nil || !strings.HasPrefix(stdout, "ibkr ") || stderr != "" {
		t.Fatal(stdout, stderr, err)
	}
	stdout, _, err = execute("--help")
	if err != nil || !strings.Contains(stdout, "quick-vwap-order") {
		t.Fatal(stdout, err)
	}
	for _, args := range [][]string{{"positions"}, {"positions-live", "--account-id", "DU1", "--direction", "x"}, {"order", "place"}, {"order", "fee-plan", "--orders-json", "[]"}, {"fetch-history", "--conid", "1"}, {"unknown"}} {
		stdout, _, err := execute(args...)
		if err == nil || stdout != "" {
			t.Fatal(args, stdout, err)
		}
	}
	directory := t.TempDir()
	output := filepath.Join(directory, "fee.json")
	stdout, stderr, err = execute("order", "fee-plan", "--orders-json", `[{"quantity":100,"price":100},{"quantity":"101","price":"100"}]`, "--pretty", "--output", output)
	if err != nil || stdout != "" || stderr != "" {
		t.Fatal(stdout, stderr, err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"feePlan": "Tiered"`)) || !bytes.Contains(raw, []byte(`"feePlan": "Fixed"`)) {
		t.Fatal(string(raw))
	}
	t.Setenv("IBKR_ACCESS_TOKEN_SECRET", "never-print-this-secret")
	stdout, _, err = execute("env")
	if err != nil || strings.Contains(stdout, "never-print") {
		t.Fatal(stdout, err)
	}
}
func TestOrderConfirmationFlows(t *testing.T) {
	server := testsupport.NewServer(t)
	setupEnvironment(t, server)
	for _, test := range []struct {
		name, messageID, overrides string
		wantError                  bool
		wantReplies                int
	}{
		{"default accepted", "o354", "", false, 1}, {"unknown", "unknown", "", true, 0}, {"explicit rejection", "o354", `{"o354":false}`, true, 0}, {"explicit acceptance", "custom", `{"custom":true}`, false, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			server.SetResponse(func(request testsupport.Request) (int, string) {
				if strings.Contains(request.Path, "/reply/") {
					return 200, `[{"order_id":"42"}]`
				}
				return 200, `[{"id":"warning","message":["test warning"],"messageIds":["` + test.messageID + `"]}]`
			})
			before := len(server.SnapshotRequests())
			args := []string{"order", "place", "--account-id", "DU1", "--orders-json", `{"conid":1}`}
			if test.overrides != "" {
				args = append(args, "--answers-json", test.overrides)
			}
			stdout, _, err := execute(args...)
			if (err != nil) != test.wantError {
				t.Fatal(stdout, err)
			}
			if len(server.SnapshotRequests())-before != 1+test.wantReplies {
				t.Fatal("wrong reply count")
			}
			if !test.wantError && strings.TrimSpace(stdout) != `{"order_id":"42"}` {
				t.Fatal(stdout)
			}
		})
	}
}
func TestInteractiveCancelAndSubmit(t *testing.T) {
	server := testsupport.NewServer(t)
	setupEnvironment(t, server)
	server.SetResponse(func(request testsupport.Request) (int, string) {
		if strings.HasSuffix(request.Path, "trsrv/stocks") {
			return 200, `{"AAPL":[{"contracts":[{"conid":1,"isUS":true}]}]}`
		}
		return 200, `[{"order_id":"42"}]`
	})
	for _, confirmed := range []bool{false, true} {
		input := "DU1\nAAPL\n\nBUY\n1\n100\n\n\n\n\n"
		if confirmed {
			input += "yes\n"
		} else {
			input += "\n"
		}
		var stdout, stderr bytes.Buffer
		command := NewRoot(strings.NewReader(input), &stdout, &stderr)
		command.SetArgs([]string{"quick-vwap-order"})
		before := len(server.SnapshotRequests())
		err := command.Execute()
		if confirmed {
			if err != nil || len(server.SnapshotRequests()) != before+2 || !json.Valid(stdout.Bytes()) {
				t.Fatal(stdout.String(), err)
			}
		} else {
			if err == nil || len(server.SnapshotRequests()) != before+1 || stdout.Len() != 0 {
				t.Fatal("cancel submitted order", err)
			}
		}
		if !strings.Contains(stderr.String(), "VWAP order payload") {
			t.Fatal(stderr.String())
		}
	}
}

func TestWarningLogLevels(t *testing.T) {
	for filter, want := range map[string]bool{"": true, "warn": true, "debug": true, "error": false, "off": false, "worker=off": false, "error,worker=warn": true} {
		if actual := shouldLogWarning(filter); actual != want {
			t.Errorf("%s: got %v, want %v", filter, actual, want)
		}
	}
}
