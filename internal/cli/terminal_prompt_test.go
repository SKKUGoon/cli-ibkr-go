package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTerminalEditingProtectsPromptAndHandlesUnicode(t *testing.T) {
	model := newTerminalPrompt("기존 .env 파일 경로")
	update := func(message tea.Msg) { result, _ := model.Update(message); model = result.(terminalPrompt) }
	update(tea.WindowSizeMsg{Width: 20, Height: 10})
	update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("한글ab")})
	update(tea.KeyMsg{Type: tea.KeyLeft})
	update(tea.KeyMsg{Type: tea.KeyDelete})
	if model.input.Value() != "한글a" {
		t.Fatal(model.input.Value())
	}
	for range 10 {
		update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	if model.input.Value() != "" || !strings.Contains(model.View(), "기존 .env 파일 경로") {
		t.Fatal("backspace altered the prompt")
	}
	update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("한글")})
	update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.submitted || model.input.Value() != "한글" {
		t.Fatal("submission changed input")
	}
}

func TestTerminalCancellationKeys(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyCtrlC, tea.KeyEsc, tea.KeyCtrlD} {
		model := newTerminalPrompt("Path")
		result, command := model.Update(tea.KeyMsg{Type: key})
		if !result.(terminalPrompt).cancelled || command == nil {
			t.Fatal("cancel key ignored", key)
		}
	}
}

func TestPipedPromptStopsOnContextCancellation(t *testing.T) {
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := NewRoot(input, io.Discard, io.Discard)
	command.SetContext(ctx)
	prompt := newPrompter(command)
	result := make(chan error, 1)
	go func() { _, err := prompt.text("Path", "", true); result <- err }()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("input remained blocked after cancellation")
	}
}

func TestRootHelpUsesFunctionalGroups(t *testing.T) {
	root := NewRoot(strings.NewReader(""), io.Discard, io.Discard)
	expected := map[string]string{
		"configure": "setup", "env": "setup", "oauth": "setup",
		"init-session": "session", "auth-status": "session", "tickle": "session",
		"accounts": "accounts", "brokerage-accounts": "accounts", "account-summary": "accounts", "portfolio-summary": "accounts", "account-pnl": "accounts", "ledger": "accounts",
		"positions": "positions", "positions-live": "positions", "trades": "positions",
		"stock-conid": "market", "fetch-history": "market",
		"order": "orders", "live-orders": "orders", "vwap-order": "orders",
	}
	for _, command := range root.Commands() {
		if group, ok := expected[command.Name()]; !ok || command.GroupID != group {
			t.Fatalf("incorrect group for %s: %s", command.Name(), command.GroupID)
		}
	}
}
