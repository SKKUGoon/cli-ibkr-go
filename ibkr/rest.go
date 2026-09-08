package ibkr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (client *Client) AuthStatus(ctx context.Context) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", AuthStatusPath, nil, nil)
}
func (client *Client) InitSession(ctx context.Context, compete bool) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", InitSessionPath, nil, map[string]bool{"publish": true, "compete": compete})
}
func (client *Client) Tickle(ctx context.Context) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", TicklePath, nil, nil)
}
func (client *Client) Accounts(ctx context.Context) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", PortfolioAccountsPath, nil, nil)
}
func (client *Client) BrokerageAccounts(ctx context.Context) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", BrokerageAccountsPath, nil, nil)
}
func (client *Client) AccountPnL(ctx context.Context) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", AccountPnLPath, nil, nil)
}
func (client *Client) AccountSummary(ctx context.Context, account string) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", AccountSummaryPath(account), nil, nil)
}
func (client *Client) PortfolioSummary(ctx context.Context, account string) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", PortfolioSummaryPath(account), nil, nil)
}
func (client *Client) Ledger(ctx context.Context, account string) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", LedgerPath(account), nil, nil)
}
func (client *Client) Positions(ctx context.Context, account string, page uint32) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", PositionsPath(account, strconv.FormatUint(uint64(page), 10)), nil, nil)
}

type LivePositionsRequest struct {
	AccountID              string
	Model, Sort, Direction *string
}

func (client *Client) LivePositions(ctx context.Context, request LivePositionsRequest) (json.RawMessage, error) {
	query := url.Values{}
	for key, value := range map[string]*string{"model": request.Model, "sort": request.Sort, "direction": request.Direction} {
		if value != nil {
			query.Set(key, *value)
		}
	}
	if request.Direction != nil && *request.Direction != "a" && *request.Direction != "d" {
		return nil, fmt.Errorf("direction must be a or d")
	}
	return client.RequestJSON(ctx, "GET", LivePositionsPath(request.AccountID), query, nil)
}
func (client *Client) Trades(ctx context.Context, account *string, days *uint8) (json.RawMessage, error) {
	query := url.Values{}
	if account != nil {
		query.Set("accountId", *account)
	}
	if days != nil {
		query.Set("days", strconv.Itoa(int(*days)))
	}
	return client.RequestJSON(ctx, "GET", TradesPath, query, nil)
}
func (client *Client) LiveOrders(ctx context.Context, account *string, force bool) (json.RawMessage, error) {
	query := url.Values{"force": {strconv.FormatBool(force)}}
	if account != nil {
		query.Set("accountId", *account)
	}
	return client.RequestJSON(ctx, "GET", LiveOrdersPath, query, nil)
}
func (client *Client) PlaceOrders(ctx context.Context, account string, request PlaceOrdersRequest) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", PlaceOrdersPath(account), nil, request)
}
func (client *Client) WhatifOrders(ctx context.Context, account string, request PlaceOrdersRequest) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", WhatifOrdersPath(account), nil, request)
}
func (client *Client) ModifyOrder(ctx context.Context, account, id string, request Order) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "POST", SingleOrderPath(account, id), nil, request)
}
func (client *Client) CancelOrder(ctx context.Context, account, id string) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "DELETE", SingleOrderPath(account, id), nil, nil)
}
func (client *Client) OrderStatus(ctx context.Context, id string) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", OrderStatusPath(id), nil, nil)
}

type AlgoRequest struct {
	Conid                     string
	Algos                     []string
	AddDescription, AddParams bool
}

func (request AlgoRequest) Query() (url.Values, error) {
	if strings.TrimSpace(request.Conid) == "" || len(request.Algos) > 8 {
		return nil, fmt.Errorf("conid must be nonempty and algos cannot contain more than 8 ids")
	}
	query := url.Values{"addDescription": {"0"}, "addParams": {"0"}}
	if request.AddDescription {
		query.Set("addDescription", "1")
	}
	if request.AddParams {
		query.Set("addParams", "1")
	}
	if len(request.Algos) > 0 {
		query.Set("algos", strings.Join(request.Algos, ";"))
	}
	return query, nil
}
func (client *Client) OrderAlgos(ctx context.Context, request AlgoRequest) (json.RawMessage, error) {
	query, err := request.Query()
	if err != nil {
		return nil, err
	}
	return client.RequestJSON(ctx, "GET", AlgosPath(strings.TrimSpace(request.Conid)), query, nil)
}
