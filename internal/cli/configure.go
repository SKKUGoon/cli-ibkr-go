package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"ibkr-go/ibkr"
	"ibkr-go/internal/config"
)

type configurationFile struct {
	key  string
	name string
}

var configurationFiles = []configurationFile{
	{"IBKR_DH_PARAM_PATH", "dhparam.pem"},
	{"IBKR_ENCRYPTION_KEY_PATH", "private_encryption.pem"},
	{"IBKR_SIGNATURE_KEY_PATH", "private_signature.pem"},
	{"IBKR_ORDERS_ANSWER_JSON", "order_answers.json"},
}

func (app *application) addConfigureCommand() {
	command := &cobra.Command{
		Use: "configure", Short: "Import an existing .env and OAuth files interactively", Args: cobra.NoArgs,
		// Setup must also work before the destination dotenv file exists.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE:              func(command *cobra.Command, _ []string) error { return app.importAndSaveConfiguration(command) },
	}
	app.root.AddCommand(command)
}

func expandConfigurationPath(path, base string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return filepath.Abs(path)
}

func (app *application) importAndSaveConfiguration(command *cobra.Command) error {
	destination := app.envFile
	if destination == "" {
		var err error
		destination, err = config.DefaultEnvFile()
		if err != nil {
			return err
		}
	}
	destination, err := expandConfigurationPath(destination, ".")
	if err != nil {
		return err
	}
	prompt := newPrompter(command)
	fmt.Fprintf(prompt.output, "설정 저장 위치: %s\n인증 정보는 기존 .env에서 가져옵니다.\n", destination)
	initialSource := ""
	if _, err := os.Stat(destination); err == nil {
		initialSource = destination
	}
	sourceInput, err := prompt.text("기존 .env 파일 경로", initialSource, true)
	if err != nil {
		return err
	}
	source, err := expandConfigurationPath(sourceInput, ".")
	if err != nil {
		return err
	}
	imported, err := godotenv.Read(source)
	if err != nil {
		return fmt.Errorf(".env 파일을 읽거나 해석할 수 없습니다: %s", source)
	}
	// Only recognized IBKR settings belong in the dedicated configuration.
	environment := config.Environment{}
	for _, key := range config.ReportKeys {
		if value, exists := imported[key]; exists {
			environment[key] = value
		}
	}
	for _, key := range []string{"IBKR_CONSUMER_KEY", "IBKR_ACCESS_TOKEN", "IBKR_ACCESS_TOKEN_SECRET"} {
		if strings.TrimSpace(environment[key]) == "" {
			return fmt.Errorf("기존 .env에 %s 값이 필요합니다", key)
		}
	}
	for _, file := range configurationFiles {
		initialPath := environment[file.key]
		if initialPath == "" {
			initialPath = file.name
		}
		initialPath, err = expandConfigurationPath(initialPath, filepath.Dir(source))
		if err != nil {
			return err
		}
		input, err := prompt.text(file.name+" 경로", initialPath, true)
		if err != nil {
			return err
		}
		environment[file.key], err = expandConfigurationPath(input, filepath.Dir(source))
		if err != nil {
			return err
		}
	}
	if err := saveValidatedConfiguration(destination, environment); err != nil {
		return err
	}
	fmt.Fprintf(prompt.output, "설정과 파일을 저장했습니다: %s\n로컬 검증이 완료되었습니다. 실제 연결은 ibkr init-session으로 확인하십시오.\n", destination)
	return nil
}

func saveValidatedConfiguration(destination string, environment config.Environment) error {
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	// A new generation keeps the active configuration intact until all files validate.
	materialsDirectory, err := os.MkdirTemp(directory, "materials-")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(materialsDirectory)
		}
	}()
	saved := config.Environment{}
	for key, value := range environment {
		saved[key] = value
	}
	for _, file := range configurationFiles {
		contents, err := os.ReadFile(environment[file.key])
		if err != nil {
			return fmt.Errorf("read %s: %w", file.name, err)
		}
		saved[file.key] = filepath.Join(materialsDirectory, file.name)
		if err := os.WriteFile(saved[file.key], contents, 0600); err != nil {
			return err
		}
	}
	oauthConfig, err := saved.OAuth(nil)
	if err != nil {
		return err
	}
	if err := ibkr.ValidateOAuthKeyFiles(oauthConfig); err != nil {
		return err
	}
	validator := application{environment: saved}
	if _, err := validator.readAnswers("", ""); err != nil {
		return fmt.Errorf("invalid order_answers.json: %w", err)
	}
	serialized, err := godotenv.Marshal(map[string]string(saved))
	if err != nil {
		return err
	}
	temporaryEnv, err := os.CreateTemp(directory, ".env-")
	if err != nil {
		return err
	}
	defer os.Remove(temporaryEnv.Name())
	if _, err := temporaryEnv.WriteString(serialized + "\n"); err != nil {
		_ = temporaryEnv.Close()
		return err
	}
	if err := temporaryEnv.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryEnv.Name(), destination); err != nil {
		return err
	}
	committed = true
	return nil
}
