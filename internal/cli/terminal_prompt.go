package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type terminalPrompt struct {
	label     string
	input     textinput.Model
	cancelled bool
	submitted bool
}

func newTerminalPrompt(label string) terminalPrompt {
	input := textinput.New()
	input.Prompt = "> "
	input.Width = 72
	input.Focus()
	return terminalPrompt{label: label, input: input}
}

func (model terminalPrompt) Init() tea.Cmd { return textinput.Blink }

func (model terminalPrompt) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		switch message.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			model.cancelled = true
			return model, tea.Quit
		case tea.KeyCtrlD:
			if model.input.Value() == "" {
				model.cancelled = true
				return model, tea.Quit
			}
		case tea.KeyEnter:
			model.submitted = true
			return model, tea.Quit
		}
	case tea.WindowSizeMsg:
		model.input.Width = max(1, message.Width-3)
	}
	var command tea.Cmd
	model.input, command = model.input.Update(message)
	return model, command
}

func (model terminalPrompt) View() string {
	if model.cancelled {
		return ""
	}
	if model.submitted {
		return model.label + "\n> " + model.input.Value() + "\n"
	}
	return model.label + "\n" + model.input.View()
}

func (prompt prompter) readTerminalLine(label string) (string, error) {
	program := tea.NewProgram(newTerminalPrompt(label), tea.WithInput(prompt.terminal), tea.WithOutput(prompt.output), tea.WithContext(prompt.context), tea.WithoutSignalHandler())
	result, err := program.Run()
	if prompt.context.Err() != nil {
		return "", prompt.context.Err()
	}
	if err != nil {
		return "", fmt.Errorf("terminal input: %w", err)
	}
	model := result.(terminalPrompt)
	if model.cancelled {
		return "", context.Canceled
	}
	return model.input.Value(), nil
}
