package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"ibkr-go/ibkr"
	"ibkr-go/internal/config"
	"ibkr-go/internal/materials"
)

type application struct {
	environment     config.Environment
	envFile, output string
	pretty          bool
	timeout         uint64
	root            *cobra.Command
}

func NewRoot(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	app := &application{}
	root := &cobra.Command{Use: "ibkr", Short: "OAuth-only IBKR REST CLI for Airflow and operators", Version: ibkr.Version, SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	root.RunE = func(command *cobra.Command, args []string) error {
		return fmt.Errorf("a command is required; see ibkr --help")
	}
	root.Flags().BoolP("version", "V", false, "Print version")
	app.root = root
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetVersionTemplate("ibkr {{.Version}}\n")
	root.PersistentFlags().StringVar(&app.envFile, "env-file", "", "Load a specific dotenv file (process environment takes precedence)")
	root.PersistentFlags().StringVar(&app.output, "output", "", "Write result to file")
	root.PersistentFlags().BoolVar(&app.pretty, "pretty", false, "Pretty-print JSON")
	root.PersistentFlags().Uint64Var(&app.timeout, "timeout-seconds", 30, "HTTP request timeout")
	root.PersistentPreRunE = func(command *cobra.Command, _ []string) error {
		var err error
		app.environment, err = config.LoadEnvironment(app.envFile)
		return err
	}
	root.CompletionOptions.DisableDefaultCmd = true
	env := app.command("env", "Show configuration with secrets redacted", func(_ *cobra.Command) (any, error) { return nil, nil })
	env.RunE = func(_ *cobra.Command, _ []string) error { return app.writeText(app.environment.Report()) }
	oauth := &cobra.Command{Use: "oauth", Short: "Manage OAuth key materials", Args: cobra.NoArgs}
	generate := &cobra.Command{Use: "generate-materials", Short: "Generate RSA keys and DH parameters with OpenSSL", Args: cobra.NoArgs}
	var outDir string
	var force bool
	generate.Flags().StringVar(&outDir, "out-dir", "", "Destination directory")
	generate.Flags().BoolVar(&force, "force", false, "Replace existing generated files")
	_ = generate.MarkFlagRequired("out-dir")
	generate.RunE = func(command *cobra.Command, _ []string) error {
		return materials.Generate(command.Context(), outDir, force, root.ErrOrStderr())
	}
	oauth.RunE = func(*cobra.Command, []string) error { return fmt.Errorf("an oauth subcommand is required") }
	oauth.AddCommand(generate)
	root.AddCommand(env, oauth)
	app.addAccountCommands()
	app.addMarketCommands()
	app.addOrderCommands()
	app.addQuickVwapCommand()
	return root
}
func (app *application) command(name, description string, run func(*cobra.Command) (any, error)) *cobra.Command {
	command := &cobra.Command{Use: name, Short: description, Args: cobra.NoArgs}
	command.RunE = func(command *cobra.Command, _ []string) error {
		response, err := run(command)
		if err != nil {
			return err
		}
		return app.writeJSON(response)
	}
	return command
}
func (app *application) withClient(command *cobra.Command, run func(context.Context, *ibkr.Client) (any, error)) (any, error) {
	var override *uint64
	if app.root.PersistentFlags().Changed("timeout-seconds") {
		override = &app.timeout
	}
	configuration, err := app.environment.OAuth(override)
	if err != nil {
		return nil, err
	}
	client, err := ibkr.NewClient(configuration)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return run(command.Context(), client)
}
func (app *application) writeText(text string) error {
	if app.output != "" {
		return os.WriteFile(app.output, []byte(text), 0600)
	}
	_, err := io.WriteString(app.root.OutOrStdout(), text)
	return err
}
func (app *application) writeJSON(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if app.pretty {
		var formatted bytes.Buffer
		if err = json.Indent(&formatted, raw, "", "  "); err != nil {
			return err
		}
		raw = formatted.Bytes()
	}
	if app.output != "" {
		return os.WriteFile(app.output, raw, 0600)
	}
	_, err = fmt.Fprintln(app.root.OutOrStdout(), string(raw))
	return err
}
func stringFlag(command *cobra.Command, name, description string, required bool) *string {
	value := command.Flags().String(name, "", description)
	if required {
		_ = command.MarkFlagRequired(name)
	}
	return value
}
func optionalString(command *cobra.Command, name string, value *string) *string {
	if command.Flags().Changed(name) {
		return value
	}
	return nil
}
