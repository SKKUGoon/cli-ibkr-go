package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ibkr-go/ibkr"
)

func (app *application) addVwapCommand() {
	command := app.command("vwap-order", "Interactively review and submit one VWAP limit order", nil)
	fields := map[string]*string{}
	for _, name := range []string{"account-id", "ticker", "exchange", "side", "quantity", "limit-price", "start-time", "end-time", "max-percent-volume", "client-order-id-prefix"} {
		fields[name] = stringFlag(command, name, "Initial "+name, false)
	}
	maxReplies := command.Flags().Uint32("max-replies", 20, "Maximum reply loop iterations")
	command.RunE = func(command *cobra.Command, _ []string) error {
		if side := *fields["side"]; side != "" && side != "buy" && side != "sell" {
			return fmt.Errorf("side must be buy or sell")
		}
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			prompt := newPrompter(command)
			input, ticker, exchange, err := app.promptVwapInput(prompt, fields)
			if err != nil {
				return nil, err
			}
			stock, err := client.LookupStock(ctx, ibkr.StockRequest{Symbol: ticker, Exchange: exchange, DefaultFiltering: true})
			if err != nil {
				return nil, err
			}
			input.Conid = stock.Conid
			input.ClientOrderID = ibkr.VwapClientOrderID(input.ClientOrderID, stock.Symbol, time.Now(), input.Side, input.Quantity, input.LimitPrice)
			request, err := ibkr.BuildVwapOrder(input)
			if err != nil {
				return nil, err
			}
			payload, err := json.MarshalIndent(request, "", "  ")
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(app.root.ErrOrStderr(), "Resolved %s to conid %d.\nVWAP order payload:\n%s\n", stock.Symbol, stock.Conid, payload)
			confirmed, err := prompt.confirm("Submit this VWAP order?")
			if err != nil {
				return nil, err
			}
			if !confirmed {
				return nil, fmt.Errorf("operator cancelled")
			}
			response, err := client.RequestJSON(ctx, "POST", ibkr.PlaceOrdersPath(input.AccountID), nil, request)
			if err != nil {
				return nil, err
			}
			for count := uint32(0); count < *maxReplies; count++ {
				warning, err := ibkr.ParseOrderPrompt(response)
				if err != nil {
					return nil, err
				}
				if warning == nil {
					return ibkr.FinalOrderResponse(response), nil
				}
				confirmed, err = prompt.confirm(warning.Message + " (messageId: " + warning.MessageID + ")")
				if err != nil {
					return nil, err
				}
				response, err = client.Reply(ctx, warning.ReplyID, confirmed)
				if err != nil {
					return nil, err
				}
				if !confirmed {
					return response, nil
				}
			}
			return nil, fmt.Errorf("too many order replies (%d): %s", *maxReplies, response)
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	app.root.AddCommand(command)
}
func (app *application) promptVwapInput(prompt prompter, fields map[string]*string) (ibkr.VwapInput, string, *string, error) {
	initial := func(key, fallback string) string {
		if *fields[key] != "" {
			return *fields[key]
		}
		return fallback
	}
	accountDefault := app.environment["IBKR_ACCOUNT_ID"]
	if accountDefault == "" {
		accountDefault = app.environment["IBKRCTL_ACCOUNT_ID"]
	}
	input := ibkr.VwapInput{}
	var err error
	input.AccountID, err = prompt.text("Account ID", initial("account-id", accountDefault), true)
	if err != nil {
		return input, "", nil, err
	}
	ticker, err := prompt.text("Ticker", *fields["ticker"], true)
	if err != nil {
		return input, "", nil, err
	}
	exchangeText, err := prompt.text("Exchange (blank for automatic)", *fields["exchange"], false)
	if err != nil {
		return input, "", nil, err
	}
	var exchange *string
	if exchangeText != "" {
		exchange = &exchangeText
	}
	input.Side = strings.ToUpper(*fields["side"])
	if input.Side == "" {
		for {
			input.Side, err = prompt.text("Side (BUY/SELL)", "BUY", true)
			if err != nil {
				return input, "", nil, err
			}
			input.Side = strings.ToUpper(input.Side)
			if input.Side == "BUY" || input.Side == "SELL" {
				break
			}
		}
	}
	input.Quantity, err = prompt.positiveNumber("Quantity", *fields["quantity"])
	if err != nil {
		return input, "", nil, err
	}
	input.LimitPrice, err = prompt.positiveNumber("Limit price", *fields["limit-price"])
	if err != nil {
		return input, "", nil, err
	}
	for _, field := range []struct {
		label, key, fallback string
		target               *string
	}{
		{"VWAP start time", "start-time", "15:30:00 US/Eastern", &input.StartTime},
		{"VWAP end time", "end-time", "16:00:00 US/Eastern", &input.EndTime},
		{"Maximum percent volume", "max-percent-volume", "0.1", &input.MaxPercentVolume},
		{"Client order ID prefix", "client-order-id-prefix", app.environment["IBKR_QUICK_ORDER_PREFIX"], &input.ClientOrderID},
	} {
		fallback := field.fallback
		if field.key == "client-order-id-prefix" && fallback == "" {
			fallback = "quick-vwap"
		}
		*field.target, err = prompt.text(field.label, initial(field.key, fallback), true)
		if err != nil {
			return input, "", nil, err
		}
	}
	return input, strings.ToUpper(ticker), exchange, nil
}
