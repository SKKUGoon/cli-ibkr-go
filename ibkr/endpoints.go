package ibkr

import "net/url"

const (
	AuthStatusPath        = "iserver/auth/status"
	InitSessionPath       = "iserver/auth/ssodh/init"
	TicklePath            = "tickle"
	HistoryPath           = "iserver/marketdata/history"
	StocksPath            = "trsrv/stocks"
	PortfolioAccountsPath = "portfolio/accounts"
	BrokerageAccountsPath = "iserver/accounts"
	AccountPnLPath        = "iserver/account/pnl/partitioned"
	TradesPath            = "iserver/account/trades/"
	LiveOrdersPath        = "iserver/account/orders"
)

func AccountSummaryPath(account string) string {
	return "iserver/account/" + url.PathEscape(account) + "/summary"
}
func PortfolioSummaryPath(account string) string {
	return "portfolio/" + url.PathEscape(account) + "/summary"
}
func LedgerPath(account string) string { return "portfolio/" + url.PathEscape(account) + "/ledger" }
func PositionsPath(account, page string) string {
	return "portfolio/" + url.PathEscape(account) + "/positions/" + page
}
func LivePositionsPath(account string) string {
	return "portfolio2/" + url.PathEscape(account) + "/positions"
}
func PlaceOrdersPath(account string) string {
	return "iserver/account/" + url.PathEscape(account) + "/orders"
}
func WhatifOrdersPath(account string) string { return PlaceOrdersPath(account) + "/whatif" }
func ReplyPath(id string) string             { return "iserver/reply/" + url.PathEscape(id) }
func SingleOrderPath(account, id string) string {
	return "iserver/account/" + url.PathEscape(account) + "/order/" + url.PathEscape(id)
}
func OrderStatusPath(id string) string { return "iserver/account/order/status/" + url.PathEscape(id) }
func AlgosPath(conid string) string    { return "iserver/contract/" + url.PathEscape(conid) + "/algos" }
