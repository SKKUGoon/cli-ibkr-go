package materials

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var GeneratedFiles = []string{"dhparam.pem", "private_signature.pem", "private_encryption.pem", "public_signature.pem", "public_encryption.pem", "private_encryption.pk8", "private_signature.pk8"}
var Steps = [][]string{
	{"dhparam", "-out", "dhparam.pem", "-outform", "PEM", "2048"},
	{"genrsa", "-out", "private_signature.pem", "2048"},
	{"genrsa", "-out", "private_encryption.pem", "2048"},
	{"rsa", "-in", "private_signature.pem", "-outform", "PEM", "-pubout", "-out", "public_signature.pem"},
	{"rsa", "-in", "private_encryption.pem", "-outform", "PEM", "-pubout", "-out", "public_encryption.pem"},
	{"pkcs8", "-topk8", "-inform", "PEM", "-outform", "PEM", "-in", "private_encryption.pem", "-out", "private_encryption.pk8", "-nocrypt"},
	{"pkcs8", "-topk8", "-inform", "PEM", "-outform", "PEM", "-in", "private_signature.pem", "-out", "private_signature.pk8", "-nocrypt"},
}

type Runner func(context.Context, string, []string) error

func RunOpenSSL(ctx context.Context, cwd string, args []string) error {
	command := exec.CommandContext(ctx, "openssl", args...)
	command.Dir = cwd
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openssl %s: %w: %s", args[0], err, output)
	}
	return nil
}
func Generate(ctx context.Context, outDir string, force bool, stderr io.Writer) error {
	if err := RunOpenSSL(ctx, "", []string{"version"}); err != nil {
		return err
	}
	return GenerateWithRunner(ctx, outDir, force, stderr, RunOpenSSL)
}
func GenerateWithRunner(ctx context.Context, outDir string, force bool, stderr io.Writer, runner Runner) error {
	if outDir == "" {
		return fmt.Errorf("out-dir cannot be empty")
	}
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return err
	}
	for _, name := range GeneratedFiles {
		path := filepath.Join(outDir, name)
		info, err := os.Lstat(path)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("refusing non-regular output: %s", path)
			}
			if !force {
				return fmt.Errorf("refusing to overwrite %s; use --force", path)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	staging, err := os.MkdirTemp(outDir, ".oauth-materials-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	for index, args := range Steps {
		fmt.Fprintf(stderr, "Generating OAuth materials (%d/%d): %s\n", index+1, len(Steps), args[0])
		if err := runner(ctx, staging, args); err != nil {
			return err
		}
	}
	for _, name := range GeneratedFiles {
		source := filepath.Join(staging, name)
		mode := os.FileMode(0644)
		if name[:7] == "private" {
			mode = 0600
		}
		if err := os.Chmod(source, mode); err != nil {
			return err
		}
		destination := filepath.Join(outDir, name)
		if force {
			if err := os.Rename(source, destination); err != nil {
				return err
			}
		} else {
			// Linking prevents a concurrent generator from overwriting a newly created key.
			if err := os.Link(source, destination); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(stderr, "Generated OAuth materials in %s\nSend public_signature.pem, public_encryption.pem, and dhparam.pem to IBKR.\nKeep private_*.pem and *.pk8 private.\n", outDir)
	return nil
}
