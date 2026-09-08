package ibkr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Order preserves unknown IBKR fields and numeric strings at the wire boundary.
// ParseOrder normalizes the accepted Rust snake_case aliases and validates exclusions.
type Order map[string]json.RawMessage
type PlaceOrdersRequest struct {
	Orders []Order `json:"orders"`
}

var orderAliases = map[string]string{
	"sec_type": "secType", "listing_exchange": "listingExchange", "order_type": "orderType", "acct_id": "acctId",
	"cash_qty": "cashQty", "fx_qty": "fxQty", "aux_price": "auxPrice", "trailing_amt": "trailingAmt", "trailing_type": "trailingType",
	"coid": "cOID", "parent_id": "parentId", "is_single_group": "isSingleGroup", "outside_rth": "outsideRTH", "use_adaptive": "useAdaptive",
	"is_ccy_conv": "isCcyConv", "is_close": "isClose", "manual_indicator": "manualIndicator", "ext_operator": "extOperator",
	"customer_account": "customerAccount", "is_pro_customer": "isProCustomer", "allocation_method": "allocationMethod", "manual_order_time": "manualOrderTime", "strategy_parameters": "strategyParameters",
}

func ParseOrder(input []byte) (Order, error) {
	var order Order
	if err := json.Unmarshal(input, &order); err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("order must be a JSON object")
	}
	for alias, field := range orderAliases {
		if value, ok := order[alias]; ok {
			if _, duplicate := order[field]; duplicate {
				return nil, fmt.Errorf("duplicate order field: %s", field)
			}
			order[field] = value
			delete(order, alias)
		}
	}
	for field, value := range order {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			delete(order, field)
		}
	}
	for _, pair := range [][2]string{{"conid", "conidex"}, {"quantity", "cashQty"}, {"quantity", "fxQty"}, {"cashQty", "fxQty"}} {
		if order[pair[0]] != nil && order[pair[1]] != nil {
			return nil, fmt.Errorf("%s and %s cannot both be provided", pair[0], pair[1])
		}
	}
	if order["strategyParameters"] != nil && order["strategy"] == nil {
		return nil, fmt.Errorf("strategyParameters cannot be provided without strategy")
	}
	for _, field := range []string{"conidex", "secType", "ticker", "listingExchange", "side", "orderType", "acctId", "tif", "trailingType", "cOID", "parentId", "extOperator", "customerAccount", "referrer", "allocationMethod", "strategy"} {
		if raw := order[field]; raw != nil {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				return nil, fmt.Errorf("%s must be a string", field)
			}
		}
	}
	for _, field := range []string{"isSingleGroup", "outsideRTH", "useAdaptive", "isCcyConv", "deactivated", "isClose", "manualIndicator", "isProCustomer"} {
		if raw := order[field]; raw != nil {
			var value bool
			if json.Unmarshal(raw, &value) != nil {
				return nil, fmt.Errorf("%s must be a boolean", field)
			}
		}
	}
	return order, nil
}
func ParseOrders(input string) (PlaceOrdersRequest, error) {
	bytesInput := bytes.TrimSpace([]byte(input))
	var entries []json.RawMessage
	if len(bytesInput) > 0 && bytesInput[0] == '[' {
		if err := json.Unmarshal(bytesInput, &entries); err != nil {
			return PlaceOrdersRequest{}, err
		}
	} else {
		entries = []json.RawMessage{bytesInput}
	}
	if len(entries) == 0 {
		return PlaceOrdersRequest{}, fmt.Errorf("orders must contain at least one order")
	}
	request := PlaceOrdersRequest{Orders: make([]Order, 0, len(entries))}
	for _, entry := range entries {
		order, err := ParseOrder(entry)
		if err != nil {
			return request, err
		}
		request.Orders = append(request.Orders, order)
	}
	return request, nil
}
func numericOrderValue(raw json.RawMessage) (float64, error) {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		text = string(raw)
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("fee-plan requires numeric quantity and limit price fields on every order")
	}
	return number, nil
}

// CalculateFeePlan preserves the existing local 10,000 threshold heuristic; it does not change an IBKR account's pricing plan.
func CalculateFeePlan(request PlaceOrdersRequest) (any, error) {
	decisions := make([]map[string]any, 0, len(request.Orders))
	for index, order := range request.Orders {
		quantity, err := numericOrderValue(order["quantity"])
		if err != nil {
			return nil, err
		}
		price, err := numericOrderValue(order["price"])
		if err != nil {
			return nil, err
		}
		notional := math.Abs(quantity * price)
		if math.IsInf(notional, 0) {
			return nil, fmt.Errorf("order notional overflow")
		}
		plan := "Tiered"
		if notional > 10000 {
			plan = "Fixed"
		}
		decisions = append(decisions, map[string]any{"index": index, "notional": notional, "threshold": 10000, "feePlan": plan})
	}
	return map[string]any{"orders": decisions}, nil
}
