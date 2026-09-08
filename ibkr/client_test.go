package ibkr_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"

	"ibkr-go/ibkr"
	"ibkr-go/internal/testsupport"
)

func TestOAuthHandshakeAndProtectedRequests(t *testing.T) {
	server := testsupport.NewServer(t)
	client, err := ibkr.NewClient(server.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for index := 0; index < 2; index++ {
		response, err := client.RequestJSON(context.Background(), "GET", ibkr.TradesPath, url.Values{"accountId": {"DU123"}, "special": {"a b&c;d"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(response) != `{"ok":true}` {
			t.Fatal(string(response))
		}
	}
	if server.SessionCount() != 1 || len(server.SnapshotRequests()) != 2 {
		t.Fatalf("sessions=%d requests=%d", server.SessionCount(), len(server.SnapshotRequests()))
	}
}
func TestConcurrentMemoryCacheHasSingleHandshake(t *testing.T) {
	server := testsupport.NewServer(t)
	client, err := ibkr.NewClient(server.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var wait sync.WaitGroup
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := client.RequestJSON(context.Background(), "POST", ibkr.TicklePath, nil, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	if server.SessionCount() != 1 {
		t.Fatalf("sessions=%d", server.SessionCount())
	}
}
func TestDisabledCacheAndInvalidTokenProof(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		server := testsupport.NewServer(t)
		server.Config.Cache.Mode = "disabled"
		client, _ := ibkr.NewClient(server.Config)
		defer client.Close()
		for index := 0; index < 2; index++ {
			if _, err := client.RequestJSON(context.Background(), "POST", ibkr.TicklePath, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
		if server.SessionCount() != 2 {
			t.Fatal(server.SessionCount())
		}
	})
	t.Run("invalid proof", func(t *testing.T) {
		server := testsupport.NewServer(t)
		server.SetBadTokenSignature()
		client, _ := ibkr.NewClient(server.Config)
		defer client.Close()
		_, err := client.RequestJSON(context.Background(), "GET", ibkr.PortfolioAccountsPath, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "validation failed") {
			t.Fatal(err)
		}
		if len(server.SnapshotRequests()) != 0 {
			t.Fatal("protected request after failed authentication")
		}
	})
}
func TestUpstreamErrorsAndInvalidJSON(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{{"http", 410, `{"error":"gone"}`}, {"json", 200, "not json"}, {"redirect", 302, "redirect"}} {
		t.Run(test.name, func(t *testing.T) {
			server := testsupport.NewServer(t)
			server.SetResponse(func(_ testsupport.Request) (int, string) { return test.status, test.body })
			client, _ := ibkr.NewClient(server.Config)
			defer client.Close()
			_, err := client.RequestJSON(context.Background(), "GET", ibkr.PortfolioAccountsPath, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if test.status != 200 {
				var upstream *ibkr.HTTPError
				if !errors.As(err, &upstream) || upstream.Status != test.status || upstream.Body != test.body {
					t.Fatal(err)
				}
			}
			if strings.Contains(err.Error(), server.Config.AccessTokenSecret) {
				t.Fatal("secret leaked")
			}
		})
	}
}
