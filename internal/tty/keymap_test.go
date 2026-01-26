package tty

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMapKeyToTmux_Printable(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	key, literal := MapKeyToTmux(msg)
	if key != "a" {
		t.Errorf("expected key='a', got '%s'", key)
	}
	if !literal {
		t.Error("expected literal=true for printable character")
	}
}

func TestMapKeyToTmux_Enter(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	key, literal := MapKeyToTmux(msg)
	if key != "Enter" {
		t.Errorf("expected key='Enter', got '%s'", key)
	}
	if literal {
		t.Error("expected literal=false for Enter key")
	}
}

func TestMapKeyToTmux_Backspace(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	key, literal := MapKeyToTmux(msg)
	if key != "BSpace" {
		t.Errorf("expected key='BSpace', got '%s'", key)
	}
	if literal {
		t.Error("expected literal=false for Backspace")
	}
}

func TestMapKeyToTmux_Escape(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	key, literal := MapKeyToTmux(msg)
	if key != "Escape" {
		t.Errorf("expected key='Escape', got '%s'", key)
	}
	if literal {
		t.Error("expected literal=false for Escape")
	}
}

func TestMapKeyToTmux_ArrowKeys(t *testing.T) {
	tests := []struct {
		keyType tea.KeyType
		want    string
	}{
		{tea.KeyUp, "Up"},
		{tea.KeyDown, "Down"},
		{tea.KeyLeft, "Left"},
		{tea.KeyRight, "Right"},
	}

	for _, tt := range tests {
		msg := tea.KeyMsg{Type: tt.keyType}
		key, literal := MapKeyToTmux(msg)
		if key != tt.want {
			t.Errorf("expected key='%s', got '%s'", tt.want, key)
		}
		if literal {
			t.Errorf("expected literal=false for %s", tt.want)
		}
	}
}

func TestMapKeyToTmux_CtrlKeys(t *testing.T) {
	tests := []struct {
		keyType tea.KeyType
		want    string
	}{
		{tea.KeyCtrlA, "C-a"},
		{tea.KeyCtrlC, "C-c"},
		{tea.KeyCtrlD, "C-d"},
		{tea.KeyCtrlZ, "C-z"},
	}

	for _, tt := range tests {
		msg := tea.KeyMsg{Type: tt.keyType}
		key, literal := MapKeyToTmux(msg)
		if key != tt.want {
			t.Errorf("expected key='%s', got '%s'", tt.want, key)
		}
		if literal {
			t.Errorf("expected literal=false for %s", tt.want)
		}
	}
}

func TestMapKeyToTmux_FunctionKeys(t *testing.T) {
	tests := []struct {
		keyType tea.KeyType
		want    string
	}{
		{tea.KeyF1, "F1"},
		{tea.KeyF5, "F5"},
		{tea.KeyF12, "F12"},
	}

	for _, tt := range tests {
		msg := tea.KeyMsg{Type: tt.keyType}
		key, literal := MapKeyToTmux(msg)
		if key != tt.want {
			t.Errorf("expected key='%s', got '%s'", tt.want, key)
		}
		if literal {
			t.Errorf("expected literal=false for %s", tt.want)
		}
	}
}

func TestMapKeyToTmux_NavigationKeys(t *testing.T) {
	tests := []struct {
		keyType tea.KeyType
		want    string
	}{
		{tea.KeyHome, "Home"},
		{tea.KeyEnd, "End"},
		{tea.KeyPgUp, "PPage"},
		{tea.KeyPgDown, "NPage"},
		{tea.KeyInsert, "IC"},
		{tea.KeyDelete, "DC"},
	}

	for _, tt := range tests {
		msg := tea.KeyMsg{Type: tt.keyType}
		key, literal := MapKeyToTmux(msg)
		if key != tt.want {
			t.Errorf("expected key='%s', got '%s'", tt.want, key)
		}
		if literal {
			t.Errorf("expected literal=false for %s", tt.want)
		}
	}
}

func TestMapKeyToTmux_Space(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeySpace}
	key, literal := MapKeyToTmux(msg)
	if key != "Space" {
		t.Errorf("expected key='Space', got '%s'", key)
	}
	if literal {
		t.Error("expected literal=false for Space")
	}
}

func TestMapKeyToTmux_Tab(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyTab}
	key, literal := MapKeyToTmux(msg)
	if key != "Tab" {
		t.Errorf("expected key='Tab', got '%s'", key)
	}
	if literal {
		t.Error("expected literal=false for Tab")
	}
}
