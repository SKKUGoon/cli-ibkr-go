package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"ibkr-go/ibkr"
	"ibkr-go/internal/database"
)

func (app *application) connectOptionalDatabase(ctx context.Context) *pgxpool.Pool {
	if !app.useDatabase {
		return nil
	}
	connection := app.environment["IBKR_DATABASE"]
	if connection == "" {
		app.warn("IBKR_DATABASE is not set; skipping local database lookup or persistence")
		return nil
	}
	timeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	settings, err := pgxpool.ParseConfig(connection)
	if err != nil {
		app.warn("IBKR_DATABASE is invalid; skipping local database lookup or persistence")
		return nil
	}
	settings.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(timeout, settings)
	if err == nil {
		err = pool.Ping(timeout)
	}
	if err != nil {
		if pool != nil {
			pool.Close()
		}
		app.warn("IBKR_DATABASE is not connectable; skipping local database lookup or persistence")
		return nil
	}
	return pool
}
func (app *application) warn(message string) {
	if !shouldLogWarning(app.environment["IBKR_LOG"]) {
		return
	}
	fmt.Fprintln(app.root.ErrOrStderr(), "warning:", message)
}
func (app *application) addMarketCommands() {
	app.addHistoryCommand()
	stock := app.command("stock-conid", "Resolve a stock contract; use --database for DB lookup/storage", nil)
	symbol := stringFlag(stock, "symbol", "Stock symbol", true)
	stockExchange := stringFlag(stock, "exchange", "Exchange filter", false)
	filtering := stock.Flags().Bool("default-filtering", true, "Select US contracts by default")
	stock.Flags().Lookup("default-filtering").NoOptDefVal = ""
	stock.RunE = func(command *cobra.Command, _ []string) error {
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			request := ibkr.StockRequest{Symbol: strings.ToUpper(*symbol), Exchange: optionalString(command, "exchange", stockExchange), DefaultFiltering: *filtering}
			pool := app.connectOptionalDatabase(ctx)
			if pool != nil {
				defer pool.Close()
				row, err := database.FindActiveConid(ctx, pool, request.Symbol, request.Exchange)
				if err != nil {
					var ambiguous *database.AmbiguousConidError
					if errors.As(err, &ambiguous) {
						return nil, err
					}
					app.warn("conid database lookup failed; falling back to IBKR API")
				} else if row != nil {
					return row, nil
				}
			}
			result, err := client.LookupStock(ctx, request)
			if err != nil {
				return nil, err
			}
			if pool != nil {
				row, err := database.UpsertConid(ctx, pool, result)
				if err == nil {
					return row, nil
				}
				app.warn("conid database upsert failed; returning IBKR API result")
			}
			return result, nil
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	app.root.AddCommand(stock)
}
