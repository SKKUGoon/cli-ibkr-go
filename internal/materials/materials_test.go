package materials

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGenerationSequencePermissionsAndOverwrite(t *testing.T) {
	directory := t.TempDir()
	var calls [][]string
	var stderr bytes.Buffer
	runner := func(_ context.Context, cwd string, args []string) error {
		calls = append(calls, args)
		for index, arg := range args {
			if arg == "-out" {
				return os.WriteFile(filepath.Join(cwd, args[index+1]), []byte("synthetic test material"), 0600)
			}
		}
		return nil
	}
	if err := GenerateWithRunner(context.Background(), directory, false, &stderr, runner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, Steps) {
		t.Fatal(calls)
	}
	for _, file := range []string{"private_signature.pem", "private_encryption.pem", "private_signature.pk8", "private_encryption.pk8"} {
		info, err := os.Stat(filepath.Join(directory, file))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal(file, err)
		}
	}
	if err := GenerateWithRunner(context.Background(), directory, false, &stderr, runner); err == nil {
		t.Fatal("overwrote existing keys")
	}
	if err := GenerateWithRunner(context.Background(), directory, true, &stderr, runner); err != nil {
		t.Fatal(err)
	}
}
func TestGenerationFailureDoesNotReplaceExistingKeys(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "private_signature.pem")
	_ = os.WriteFile(key, []byte("keep-existing"), 0600)
	err := GenerateWithRunner(context.Background(), directory, true, &bytes.Buffer{}, func(context.Context, string, []string) error { return fmt.Errorf("test failure") })
	if err == nil {
		t.Fatal("missing failure")
	}
	raw, _ := os.ReadFile(key)
	if string(raw) != "keep-existing" {
		t.Fatal("key was changed")
	}
}

func TestOpenSSLIntegration(t *testing.T) {
	if os.Getenv("IBKR_TEST_OPENSSL") != "1" {
		t.Skip("set IBKR_TEST_OPENSSL=1 to generate temporary 2048-bit materials")
	}
	directory := t.TempDir()
	var stderr bytes.Buffer
	if err := Generate(context.Background(), directory, false, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, file := range GeneratedFiles {
		info, err := os.Stat(filepath.Join(directory, file))
		if err != nil || info.Size() == 0 {
			t.Fatal(file, err)
		}
	}
	for _, name := range []string{"private_signature.pem", "private_encryption.pem", "private_signature.pk8", "private_encryption.pk8"} {
		if err := RunOpenSSL(context.Background(), directory, []string{"pkey", "-in", name, "-check", "-noout"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunOpenSSL(context.Background(), directory, []string{"dhparam", "-in", "dhparam.pem", "-check", "-noout"}); err != nil {
		t.Fatal(err)
	}
}
