package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"ibkr-go/ibkr"
)

func (app *application) addOrderCommands() {
	group := &cobra.Command{Use: "order", Short: "Inspect, submit, modify, and cancel orders", Args: cobra.NoArgs}
	group.RunE = func(*cobra.Command, []string) error { return fmt.Errorf("an order subcommand is required") }
	for _, name := range []string{"place", "whatif", "modify", "fee-plan"} {
		app.addOrderInputCommand(group, name)
	}
	for _, name := range []string{"reply", "cancel", "status"} {
		command := app.command(name, "Perform order "+name, nil)
		var account, id *string
		var confirmed *bool
		if name == "reply" {
			id = stringFlag(command, "reply-id", "Warning reply ID", true)
			confirmed = command.Flags().Bool("confirmed", false, "Accept the warning")
		} else {
			id = stringFlag(command, "order-id", "Order ID", true)
		}
		if name == "cancel" {
			account = stringFlag(command, "account-id", "Account ID", true)
		}
		command.RunE = func(command *cobra.Command, _ []string) error {
			value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
				switch name {
				case "reply":
					return client.Reply(ctx, *id, *confirmed)
				case "cancel":
					return client.RequestJSON(ctx, "DELETE", ibkr.SingleOrderPath(*account, *id), nil, nil)
				default:
					return client.RequestJSON(ctx, "GET", ibkr.OrderStatusPath(*id), nil, nil)
				}
			})
			if err != nil {
				return err
			}
			return app.writeJSON(value)
		}
		group.AddCommand(command)
	}
	algos := app.command("algos", "List IB Algo strategies for a contract", nil)
	conid := stringFlag(algos, "conid", "Contract ID", true)
	ids := algos.Flags().StringArray("algo", nil, "Case-sensitive algo ID; repeat up to eight times")
	description := algos.Flags().Bool("add-description", false, "Include descriptions")
	params := algos.Flags().Bool("add-params", false, "Include parameters")
	algos.RunE = func(command *cobra.Command, _ []string) error {
		if strings.TrimSpace(*conid) == "" || len(*ids) > 8 {
			return fmt.Errorf("conid must be nonempty and algos cannot contain more than 8 ids")
		}
		query := url.Values{"addDescription": {"0"}, "addParams": {"0"}}
		if *description {
			query.Set("addDescription", "1")
		}
		if *params {
			query.Set("addParams", "1")
		}
		if len(*ids) > 0 {
			query.Set("algos", strings.Join(*ids, ";"))
		}
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			return client.RequestJSON(ctx, "GET", ibkr.AlgosPath(strings.TrimSpace(*conid)), query, nil)
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	group.AddCommand(algos)
	app.root.AddCommand(group)
}
func (app *application) addOrderInputCommand(group *cobra.Command, name string) {
	command := app.command(name, "Perform order "+name, nil)
	inputName := "orders-json"
	if name == "modify" {
		inputName = "order-json"
	}
	input := stringFlag(command, inputName, "Order JSON object or array (modify requires one object)", true)
	var account, id, answersFile, answersJSON *string
	var maxReplies *uint32
	if name != "fee-plan" {
		account = stringFlag(command, "account-id", "Account ID", true)
	}
	if name == "modify" {
		id = stringFlag(command, "order-id", "Order ID", true)
	}
	if name == "place" || name == "modify" {
		answersFile = stringFlag(command, "answers-file", "Warning answers JSON file", false)
		answersJSON = stringFlag(command, "answers-json", "Inline warning answers", false)
		maxReplies = command.Flags().Uint32("max-replies", 20, "Maximum reply loop iterations")
	}
	command.RunE = func(command *cobra.Command, _ []string) error {
		var body any
		if name == "modify" {
			order, err := ibkr.ParseOrder([]byte(*input))
			if err != nil {
				return err
			}
			body = order
		} else {
			request, err := ibkr.ParseOrders(*input)
			if err != nil {
				return err
			}
			body = request
			if name == "fee-plan" {
				value, err := ibkr.CalculateFeePlan(request)
				if err != nil {
					return err
				}
				return app.writeJSON(value)
			}
		}
		var answers ibkr.Answers
		if answersFile != nil {
			var err error
			answers, err = app.readAnswers(*answersFile, *answersJSON)
			if err != nil {
				return err
			}
		}
		value, err := app.withClient(command, func(ctx context.Context, client *ibkr.Client) (any, error) {
			path := ibkr.PlaceOrdersPath(*account)
			if name == "modify" {
				path = ibkr.SingleOrderPath(*account, *id)
			}
			if name == "whatif" {
				path = ibkr.WhatifOrdersPath(*account)
			}
			response, err := client.RequestJSON(ctx, "POST", path, nil, body)
			if err != nil {
				return nil, err
			}
			if name == "whatif" {
				return response, nil
			}
			return client.HandleConfirmations(ctx, response, answers, *maxReplies)
		})
		if err != nil {
			return err
		}
		return app.writeJSON(value)
	}
	group.AddCommand(command)
}
