package ibkr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type HistoryRequest struct {
	Conid, Period, Bar  string
	Exchange, StartTime *string
	OutsideRTH          *bool
}

func (request HistoryRequest) Query() url.Values {
	query := url.Values{"conid": {request.Conid}, "period": {request.Period}, "bar": {request.Bar}}
	if request.Exchange != nil {
		query.Set("exchange", *request.Exchange)
	}
	if request.StartTime != nil {
		query.Set("startTime", *request.StartTime)
	}
	if request.OutsideRTH != nil {
		query.Set("outsideRth", strconv.FormatBool(*request.OutsideRTH))
	}
	return query
}
func (client *Client) FetchHistory(ctx context.Context, request HistoryRequest) (json.RawMessage, error) {
	return client.RequestJSON(ctx, "GET", HistoryPath, request.Query(), nil)
}

type StockRequest struct {
	Symbol           string
	Exchange         *string
	DefaultFiltering bool
}
type StockResult struct {
	Symbol   string  `json:"symbol"`
	Name     *string `json:"name_"`
	Conid    int64   `json:"conid"`
	Exchange *string `json:"exchange"`
}

func (client *Client) LookupStock(ctx context.Context, request StockRequest) (StockResult, error) {
	response, err := client.RequestJSON(ctx, "GET", StocksPath, url.Values{"symbols": {request.Symbol}}, nil)
	if err != nil {
		return StockResult{}, err
	}
	return SelectStock(response, request)
}
func SelectStock(response json.RawMessage, request StockRequest) (StockResult, error) {
	var instruments map[string][]struct {
		Name      *string `json:"name"`
		Contracts []struct {
			Conid    *int64  `json:"conid"`
			Exchange *string `json:"exchange"`
			IsUS     bool    `json:"isUS"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(response, &instruments); err != nil {
		return StockResult{}, err
	}
	rows, found := instruments[request.Symbol]
	if !found {
		rows, found = instruments[strings.ToUpper(request.Symbol)]
	}
	if !found {
		return StockResult{}, fmt.Errorf("stock %s: symbol not found in IBKR stock response", request.Symbol)
	}
	matches := []StockResult{}
	for _, instrument := range rows {
		for _, contract := range instrument.Contracts {
			if contract.Conid == nil || (request.DefaultFiltering && !contract.IsUS) {
				continue
			}
			if request.Exchange != nil && (contract.Exchange == nil || !strings.EqualFold(*request.Exchange, *contract.Exchange)) {
				continue
			}
			matches = append(matches, StockResult{strings.ToUpper(request.Symbol), instrument.Name, *contract.Conid, contract.Exchange})
		}
	}
	if len(matches) != 1 {
		return StockResult{}, fmt.Errorf("stock %s: %d contracts matched; specify --exchange or change filters", request.Symbol, len(matches))
	}
	return matches[0], nil
}
