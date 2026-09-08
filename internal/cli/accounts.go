package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
	"ibkr-go/ibkr"
)

func (app *application) addAccountCommands() {
	for _, spec := range []struct{ name, description, method, path string }{
		{"auth-status", "Read authentication status", "POST", ibkr.AuthStatusPath},
		{"tickle", "Keep an initialized brokerage session alive", "POST", ibkr.TicklePath},
		{"accounts", "Initialize and list portfolio accounts", "GET", ibkr.PortfolioAccountsPath},
		{"brokerage-accounts", "Initialize and list brokerage accounts", "GET", ibkr.BrokerageAccountsPath},
		{"account-pnl", "Read account and model P&L", "GET", ibkr.AccountPnLPath},
	} {
		app.root.AddCommand(app.command(spec.name, spec.description, func(command *cobra.Command) (any, error) {
			return app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
				return client.RequestJSON(ctx, spec.method, spec.path, nil, nil)
			})
		}))
	}
	initCommand := app.command("init-session", "Initialize the brokerage session", nil)
	compete := initCommand.Flags().Bool("compete", true, "Compete for the brokerage session")
	initCommand.RunE = func(command *cobra.Command, _ []string) error {
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			return client.RequestJSON(ctx, "POST", ibkr.InitSessionPath, nil, map[string]bool{"publish": true, "compete": *compete})
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	app.root.AddCommand(initCommand)
	for _, name := range []string{"account-summary", "portfolio-summary", "ledger", "positions", "positions-live", "trades", "live-orders"} {
		app.addAccountQuery(name)
	}
}
func (app *application) addAccountQuery(name string) {
	command := app.command(name, "Fetch "+name+" through REST", nil)
	account := stringFlag(command, "account-id", "IBKR account ID", name != "trades" && name != "live-orders")
	var page *uint32
	var model, sort, direction *string
	var days *uint8
	var force *bool
	switch name {
	case "positions":
		page = command.Flags().Uint32("page", 0, "Page number")
	case "positions-live":
		model = stringFlag(command, "model", "Model", false)
		sort = stringFlag(command, "sort", "Sort field", false)
		direction = stringFlag(command, "direction", "a (ascending) or d (descending)", false)
	case "trades":
		days = command.Flags().Uint8("days", 0, "Number of days")
	case "live-orders":
		force = command.Flags().Bool("force", true, "Refresh live orders")
	}
	command.RunE = func(command *cobra.Command, _ []string) error {
		query := url.Values{}
		var path string
		switch name {
		case "account-summary":
			path = ibkr.AccountSummaryPath(*account)
		case "portfolio-summary":
			path = ibkr.PortfolioSummaryPath(*account)
		case "ledger":
			path = ibkr.LedgerPath(*account)
		case "positions":
			path = ibkr.PositionsPath(*account, strconv.FormatUint(uint64(*page), 10))
		case "positions-live":
			path = ibkr.LivePositionsPath(*account)
			if command.Flags().Changed("direction") && *direction != "a" && *direction != "d" {
				return fmt.Errorf("direction must be a or d")
			}
			for key, value := range map[string]*string{"model": model, "sort": sort, "direction": direction} {
				if command.Flags().Changed(key) {
					query.Set(key, *value)
				}
			}
		case "trades":
			path = ibkr.TradesPath
			if command.Flags().Changed("days") {
				query.Set("days", strconv.Itoa(int(*days)))
			}
		case "live-orders":
			path = ibkr.LiveOrdersPath
			query.Set("force", strconv.FormatBool(*force))
		}
		if (name == "trades" || name == "live-orders") && command.Flags().Changed("account-id") {
			query.Set("accountId", *account)
		}
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			return client.RequestJSON(ctx, "GET", path, query, nil)
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	app.root.AddCommand(command)
}
