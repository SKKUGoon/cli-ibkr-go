package cli

import (
	"bufio"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

type prompter struct {
	input    *bufio.Reader
	output   io.Writer
	context  context.Context
	terminal *os.File
}

func (prompt prompter) text(label, initial string, required bool) (string, error) {
	for {
		line, err := prompt.readLine(label, initial)
		if err != nil {
			return "", fmt.Errorf("operator input ended: %w", err)
		}
		value := strings.TrimSpace(line)
		if value == "" {
			value = initial
		}
		if required && value == "" {
			fmt.Fprintln(prompt.output, "Value cannot be empty.")
			continue
		}
		return value, nil
	}
}
func (prompt prompter) positiveNumber(label, initial string) (float64, error) {
	for {
		text, err := prompt.text(label, initial, true)
		if err != nil {
			return 0, err
		}
		number, err := strconv.ParseFloat(text, 64)
		if err == nil && number > 0 && !math.IsInf(number, 0) && !math.IsNaN(number) {
			return number, nil
		}
		fmt.Fprintln(prompt.output, "Value must be a positive finite number.")
	}
}
func (prompt prompter) confirm(message string) (bool, error) {
	value, err := prompt.text(message+" (yes/no)", "no", false)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(value, "yes") || strings.EqualFold(value, "y"), nil
}

func newPrompter(command *cobra.Command) prompter {
	input := command.InOrStdin()
	prompt := prompter{input: bufio.NewReader(input), output: command.ErrOrStderr(), context: command.Context()}
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		prompt.terminal = file
	}
	return prompt
}

func (prompt prompter) readLine(label, initial string) (string, error) {
	if err := prompt.context.Err(); err != nil {
		return "", err
	}
	if initial != "" {
		label += " [" + initial + "]"
	}
	if prompt.terminal != nil {
		return prompt.readTerminalLine(label)
	}
	fmt.Fprintf(prompt.output, "%s: ", label)
	type lineResult struct {
		line string
		err  error
	}
	result := make(chan lineResult, 1)
	go func() {
		line, err := prompt.input.ReadString('\n')
		result <- lineResult{line, err}
	}()
	select {
	case <-prompt.context.Done():
		return "", prompt.context.Err()
	case line := <-result:
		return line.line, line.err
	}
}
