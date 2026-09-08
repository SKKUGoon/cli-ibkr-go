package cli

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type prompter struct {
	input  *bufio.Reader
	output io.Writer
}

func (prompt prompter) text(label, initial string, required bool) (string, error) {
	for {
		if initial != "" {
			fmt.Fprintf(prompt.output, "%s [%s]: ", label, initial)
		} else {
			fmt.Fprintf(prompt.output, "%s: ", label)
		}
		line, err := prompt.input.ReadString('\n')
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
