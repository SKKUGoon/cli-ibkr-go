package ibkr

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
	"time"
)

func TestSignatureBaseGolden(t *testing.T) {
	actual := signatureBase("post", "https://example.test/x", map[string]string{"z": "last", "a": "first"}, nil, "")
	if actual != "POST&https%3A%2F%2Fexample.test%2Fx&a%3Dfirst%26z%3Dlast" {
		t.Fatal(actual)
	}
	if percentEncode(" ~!+:/é") != "%20~%21%2B%3A%2F%C3%A9" {
		t.Fatal(percentEncode(" ~!+:/é"))
	}
}
func TestOrderAliasesAndValidation(t *testing.T) {
	request, err := ParseOrders(`{"conid":1,"order_type":"LMT","quantity":"2","price":10,"outside_rth":false,"coid":"x","custom":{"a":1}}`)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(request)
	var wire map[string][]map[string]any
	_ = json.Unmarshal(raw, &wire)
	order := wire["orders"][0]
	if order["orderType"] != "LMT" || order["cOID"] != "x" || order["outsideRTH"] != false || order["custom"] == nil {
		t.Fatal(string(raw))
	}
	for _, input := range []string{`[]`, `null`, `{"conid":1,"conidex":"x"}`, `{"quantity":1,"cash_qty":1}`, `{"fxQty":1,"cashQty":1}`, `{"strategy_parameters":{}}`, `{"order_type":"LMT","orderType":"MKT"}`, `{"side":1}`, `{"outsideRTH":"false"}`} {
		if _, err := ParseOrders(input); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}
func TestStockSelection(t *testing.T) {
	response := json.RawMessage(`{"AAPL":[{"name":"Apple","contracts":[{"conid":1,"exchange":"NASDAQ","isUS":true},{"conid":2,"exchange":"MEXI","isUS":false}]}]}`)
	result, err := SelectStock(response, StockRequest{Symbol: "aapl", DefaultFiltering: true})
	if err != nil || result.Conid != 1 || result.Symbol != "AAPL" {
		t.Fatal(result, err)
	}
	if _, err = SelectStock(response, StockRequest{Symbol: "AAPL"}); err == nil {
		t.Fatal("expected ambiguity")
	}
}
func TestHistoryQueryAndVwap(t *testing.T) {
	outside := false
	start := "20260908-10:00:00"
	query := HistoryRequest{Conid: "1", Period: "1d", Bar: "1min", OutsideRTH: &outside, StartTime: &start}.Query()
	if query.Get("outsideRth") != "false" || query.Get("startTime") != start {
		t.Fatal(query)
	}
	input := VwapInput{Conid: 72539702, Side: "BUY", Quantity: 10, LimitPrice: 70, AccountID: "DU123", StartTime: "15:30:00 US/Eastern", EndTime: "16:00:00 US/Eastern", MaxPercentVolume: "0.1", ClientOrderID: "x"}
	request, err := BuildVwapOrder(input)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(request)
	var wire map[string][]map[string]any
	_ = json.Unmarshal(raw, &wire)
	if wire["orders"][0]["strategy"] != "Vwap" {
		t.Fatal(string(raw))
	}
	id := VwapClientOrderID("kappa-k1", "tqqq", time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC), "BUY", 10, 70)
	if id != "kappa-k1-TQQQ-20260723-buy-new-10-7000" {
		t.Fatal(id)
	}
	input.LimitPrice = 0
	if _, err := BuildVwapOrder(input); err == nil {
		t.Fatal("accepted zero price")
	}
}
func TestPromptAndAnswers(t *testing.T) {
	prompt, err := ParseOrderPrompt(json.RawMessage(`[{"id":123,"message":[" warning\n "],"messageIds":["o354"]}]`))
	if err != nil || prompt.ReplyID != "123" || prompt.Message != "warning" || prompt.MessageID != "o354" {
		t.Fatal(prompt, err)
	}
	answer, known := Answers{"o354": false, "warning": true}.Find(prompt.Message, prompt.MessageID)
	if !known || answer {
		t.Fatal("message ID must win")
	}
	for _, raw := range []string{`{}`, `[{"message":[]}]`, `[{"message":"warning"}]`} {
		if _, err := ParseOrderPrompt(json.RawMessage(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}
func TestSessionMathAndCacheIdentity(t *testing.T) {
	// 5^3 mod 257 = 125, while 6^3 mod 257 = 216 requires the Java positive sign byte.
	first, err := computeSessionToken(big.NewInt(257), big.NewInt(3), "06", []byte{0xab, 0xcd})
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := base64.StdEncoding.DecodeString(first); err != nil || len(decoded) != 20 {
		t.Fatal(first, err)
	}
	if _, err = computeSessionToken(big.NewInt(23), big.NewInt(7), "zz", nil); err == nil {
		t.Fatal("bad hex")
	}
	config := DefaultConfig()
	config.ConsumerKey = "consumer"
	config.AccessToken = "token"
	if len(cacheKey(config)) == 0 {
		t.Fatal("empty key")
	}
}
