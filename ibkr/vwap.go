package ibkr

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type VwapInput struct {
	Conid                                                          int64
	Side                                                           string
	Quantity, LimitPrice                                           float64
	AccountID, StartTime, EndTime, MaxPercentVolume, ClientOrderID string
}

func BuildVwapOrder(input VwapInput) (PlaceOrdersRequest, error) {
	if input.Conid <= 0 {
		return PlaceOrdersRequest{}, fmt.Errorf("conid must be positive")
	}
	for _, value := range []float64{input.Quantity, input.LimitPrice} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return PlaceOrdersRequest{}, fmt.Errorf("quantity and limit price must be positive finite numbers")
		}
	}
	percent, err := strconv.ParseFloat(input.MaxPercentVolume, 64)
	if err != nil || percent <= 0 || math.IsNaN(percent) || math.IsInf(percent, 0) {
		return PlaceOrdersRequest{}, fmt.Errorf("max percent volume must be positive and numeric")
	}
	for _, value := range []string{input.AccountID, input.StartTime, input.EndTime, input.ClientOrderID} {
		if strings.TrimSpace(value) == "" {
			return PlaceOrdersRequest{}, fmt.Errorf("VWAP order fields cannot be empty")
		}
	}
	if input.Side != "BUY" && input.Side != "SELL" {
		return PlaceOrdersRequest{}, fmt.Errorf("side must be BUY or SELL")
	}
	raw, err := json.Marshal(map[string]any{
		"conid": input.Conid, "side": input.Side, "quantity": input.Quantity, "orderType": "LMT", "price": input.LimitPrice, "acctId": input.AccountID, "tif": "DAY", "strategy": "Vwap", "cOID": input.ClientOrderID,
		"strategyParameters": map[string]string{"maxPctVol": input.MaxPercentVolume, "startTime": input.StartTime, "endTime": input.EndTime, "allowPastEndTime": "0", "noTakeLiq": "0", "speedUp": "0"},
	})
	if err != nil {
		return PlaceOrdersRequest{}, err
	}
	return ParseOrders(string(raw))
}
func VwapClientOrderID(prefix, symbol string, date time.Time, side string, quantity, price float64) string {
	sanitize := func(value string) string {
		return strings.Trim(strings.Map(func(character rune) rune {
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' {
				return character
			}
			return '-'
		}, value), "-")
	}
	quantityText := strings.ReplaceAll(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", quantity), "0"), "."), ".", "p")
	return fmt.Sprintf("%s-%s-%s-%s-new-%s-%.0f", sanitize(prefix), sanitize(strings.ToUpper(symbol)), date.Format("20060102"), strings.ToLower(side), quantityText, price*100)
}
