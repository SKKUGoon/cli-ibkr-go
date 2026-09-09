package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"ibkr-go/internal/config"
	"ibkr-go/internal/testsupport"
)

func TestConfigureImportsFilesAndPreservesActiveSetupOnFailure(t *testing.T) {
	fixture := testsupport.NewServer(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourceDirectory := t.TempDir()
	answers := filepath.Join(sourceDirectory, "order_answers.json")
	if err := os.WriteFile(answers, []byte(`{"test":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	imported := fixture.Environment()
	imported["IBKR_ORDERS_ANSWER_JSON"] = "order_answers.json"
	imported["UNRELATED_SECRET"] = "do-not-import"
	source := filepath.Join(sourceDirectory, ".env")
	if err := godotenv.Write(imported, source); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := NewRoot(strings.NewReader(source+"\n\n\n\n\n"), &output, &output)
	command.SetArgs([]string{"configure"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	destination, err := config.DefaultEnvFile()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := godotenv.Read(destination)
	if err != nil {
		t.Fatal(err)
	}
	if saved["IBKR_ACCESS_TOKEN"] != imported["IBKR_ACCESS_TOKEN"] || saved["UNRELATED_SECRET"] != "" {
		t.Fatal("incorrect credential import")
	}
	for _, value := range []string{imported["IBKR_ACCESS_TOKEN"], imported["IBKR_ACCESS_TOKEN_SECRET"], "do-not-import"} {
		if strings.Contains(output.String(), value) {
			t.Fatal("secret leaked in output")
		}
	}
	for _, file := range configurationFiles {
		path := saved[file.key]
		if !filepath.IsAbs(path) || !strings.HasPrefix(path, filepath.Dir(destination)+string(os.PathSeparator)) {
			t.Fatalf("unmanaged path: %s", path)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("file permissions: %v", err)
		}
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("dotenv permissions")
	}
	originalEnv, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	command = NewRoot(strings.NewReader("\n\n\n\n\n"), &output, &output)
	command.SetArgs([]string{"configure"})
	if err := command.Execute(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	// Invalid copied materials must never replace the last working configuration.
	activeEnv, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(activeEnv, originalEnv) {
		t.Fatal("expected new materials generation")
	}
	saved, err = godotenv.Read(destination)
	if err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(sourceDirectory, "invalid.pem")
	if err := os.WriteFile(invalid, []byte("not a PEM"), 0600); err != nil {
		t.Fatal(err)
	}
	saved["IBKR_SIGNATURE_KEY_PATH"] = invalid
	if err := saveValidatedConfiguration(destination, saved); err == nil {
		t.Fatal("invalid private key accepted")
	}
	afterFailure, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(afterFailure, activeEnv) {
		t.Fatal("active configuration changed on failure")
	}

	saved, err = godotenv.Read(destination)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalidJSON := range []string{`null`, `[]`, `{"question":null}`, `{"question":"yes"}`} {
		if err := os.WriteFile(answers, []byte(invalidJSON), 0600); err != nil {
			t.Fatal(err)
		}
		saved["IBKR_ORDERS_ANSWER_JSON"] = answers
		if err := saveValidatedConfiguration(destination, saved); err == nil {
			t.Fatal("invalid answers accepted")
		}
		unchanged, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(unchanged, activeEnv) {
			t.Fatal("invalid answers changed active setup")
		}
	}
	// An explicit destination need not exist before configure starts.
	if err := os.WriteFile(answers, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	customDestination := filepath.Join(t.TempDir(), "custom.env")
	command = NewRoot(strings.NewReader(source+"\n\n\n\n\n"), &output, &output)
	command.SetArgs([]string{"--env-file", customDestination, "configure"})
	if err := command.Execute(); err != nil {
		t.Fatalf("custom destination: %v", err)
	}
	if _, err := os.Stat(customDestination); err != nil {
		t.Fatal(err)
	}
	if len(fixture.SnapshotRequests()) != 0 {
		t.Fatal("configure made network requests")
	}
}

func TestConfigureRejectsMissingCredentialsAndInterruptedInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(source, []byte("IBKR_CONSUMER_KEY=example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", source + "\n"} {
		var output bytes.Buffer
		command := NewRoot(strings.NewReader(input), &output, &output)
		command.SetArgs([]string{"configure"})
		if err := command.Execute(); err == nil {
			t.Fatal("incomplete setup accepted")
		}
	}
	destination, _ := config.DefaultEnvFile()
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("incomplete setup persisted")
	}
}
