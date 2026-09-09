package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"ibkr-go/ibkr"
	"ibkr-go/internal/database"
)

func (app *application) addHistoryCommand() {
	command := app.command("fetch-history", "Fetch historical bars by period or date range; prompt for missing inputs", nil)
	command.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})
	conid := stringFlag(command, "conid", "Contract ID", false)
	period := stringFlag(command, "period", "History period (cannot combine with dates)", false)
	bar := stringFlag(command, "bar", "Bar interval, e.g. 1min, 1h, 1d", false)
	exchange := stringFlag(command, "exchange", "Exchange", false)
	startTime := stringFlag(command, "start-time", "IBKR UTC reference time (period mode only)", false)
	startDate := stringFlag(command, "start-date", "First date, YYYY-MM-DD (always included)", false)
	endDate := stringFlag(command, "end-date", "Last date, YYYY-MM-DD", false)
	inclusive := command.Flags().Bool("inclusive", true, "Include end-date")
	timezone := command.Flags().String("timezone", "UTC", "Timezone for date boundaries, e.g. America/New_York")
	outside := command.Flags().Bool("outside-rth", false, "Include outside regular trading hours")
	interactive := command.Flags().Bool("interactive", false, "Prompt for missing inputs even when other flags are supplied")
	command.RunE = func(command *cobra.Command, _ []string) error {
		datesMode := *startDate != "" || *endDate != "" || *period == ""
		if datesMode && (*period != "" || command.Flags().Changed("start-time")) {
			return fmt.Errorf("start-date/end-date cannot be combined with period/start-time")
		}
		if !datesMode && (command.Flags().Changed("inclusive") || command.Flags().Changed("timezone")) {
			return fmt.Errorf("inclusive and timezone require a date range")
		}
		prompt := newPrompter(command)
		missing := *conid == "" || *bar == "" || (datesMode && (*startDate == "" || *endDate == ""))
		if missing || *interactive {
			fields := []struct {
				target          *string
				label, fallback string
			}{{conid, "Contract ID", ""}}
			if datesMode {
				fields = append(fields, struct {
					target          *string
					label, fallback string
				}{startDate, "Start date (YYYY-MM-DD)", ""}, struct {
					target          *string
					label, fallback string
				}{endDate, "End date (YYYY-MM-DD)", ""})
			}
			fields = append(fields, struct {
				target          *string
				label, fallback string
			}{bar, "Bar interval", "1d"})
			for _, field := range fields {
				if *field.target == "" {
					value, err := prompt.text(field.label, field.fallback, true)
					if err != nil {
						return err
					}
					*field.target = value
				}
			}
			if datesMode {
				if !command.Flags().Changed("inclusive") {
					for {
						value, err := prompt.text("Include end date (true/false)", "true", true)
						if err != nil {
							return err
						}
						parsed, err := strconv.ParseBool(value)
						if err == nil {
							*inclusive = parsed
							break
						}
						fmt.Fprintln(command.ErrOrStderr(), "Enter true or false.")
					}
				}
				if !command.Flags().Changed("timezone") {
					value, err := prompt.text("Date timezone", *timezone, true)
					if err != nil {
						return err
					}
					*timezone = value
				}
			}
		}
		contract, err := strconv.ParseInt(*conid, 10, 64)
		if err != nil || contract <= 0 {
			return fmt.Errorf("conid must be a positive integer")
		}
		request := ibkr.HistoryRequest{Conid: *conid, Period: *period, Bar: *bar, Exchange: optionalString(command, "exchange", exchange), StartTime: optionalString(command, "start-time", startTime)}
		if command.Flags().Changed("outside-rth") {
			request.OutsideRTH = outside
		}
		dates := ibkr.HistoryDateRange{StartDate: *startDate, EndDate: *endDate, Inclusive: *inclusive, Timezone: *timezone}
		if datesMode {
			if _, _, err := dates.Bounds(); err != nil {
				return err
			}
		}
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			var response json.RawMessage
			var err error
			if datesMode {
				response, err = client.FetchHistoryRange(ctx, request, dates)
			} else {
				response, err = client.FetchHistory(ctx, request)
			}
			if err != nil {
				return nil, err
			}
			pool := app.connectOptionalDatabase(ctx)
			if pool != nil {
				defer pool.Close()
				bars, err := database.ParseHistoryBars(*conid, response)
				if err != nil {
					app.warn("historical bars parsing failed; returning IBKR API result")
				} else if _, err = database.UpsertHistoryBars(ctx, pool, bars); err != nil {
					app.warn("historical bars database upsert failed; returning IBKR API result")
				}
			}
			return response, nil
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	app.root.AddCommand(command)
}
