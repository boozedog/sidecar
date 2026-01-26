package tty

import "testing"

func TestNew(t *testing.T) {
	// Test with nil config (uses defaults)
	m := New(nil)
	if m == nil {
		t.Fatal("expected non-nil model")
	}
	if m.Config.ExitKey != "ctrl+\\" {
		t.Errorf("expected default ExitKey, got %s", m.Config.ExitKey)
	}
	if m.Config.ScrollbackLines != 600 {
		t.Errorf("expected default ScrollbackLines=600, got %d", m.Config.ScrollbackLines)
	}
}

func TestNew_WithConfig(t *testing.T) {
	cfg := &Config{
		ExitKey:         "ctrl+q",
		ScrollbackLines: 1000,
	}
	m := New(cfg)
	if m.Config.ExitKey != "ctrl+q" {
		t.Errorf("expected ExitKey='ctrl+q', got %s", m.Config.ExitKey)
	}
	if m.Config.ScrollbackLines != 1000 {
		t.Errorf("expected ScrollbackLines=1000, got %d", m.Config.ScrollbackLines)
	}
	// Non-overridden values should be defaults
	if m.Config.AttachKey != "ctrl+]" {
		t.Errorf("expected default AttachKey, got %s", m.Config.AttachKey)
	}
}

func TestModel_IsActive(t *testing.T) {
	m := New(nil)

	// Should be inactive initially
	if m.IsActive() {
		t.Error("expected IsActive=false initially")
	}

	// After setting state
	m.State = &State{Active: true}
	if !m.IsActive() {
		t.Error("expected IsActive=true after setting state")
	}

	// After setting active=false
	m.State.Active = false
	if m.IsActive() {
		t.Error("expected IsActive=false after setting active=false")
	}
}

func TestModel_Exit(t *testing.T) {
	m := New(nil)
	m.State = &State{
		Active:        true,
		TargetSession: "test-session",
	}

	m.Exit()

	if m.State != nil {
		t.Error("expected State=nil after Exit")
	}
}

func TestModel_GetTarget(t *testing.T) {
	m := New(nil)

	// Inactive model returns empty string
	if got := m.GetTarget(); got != "" {
		t.Errorf("expected empty target for inactive model, got %s", got)
	}

	// With pane ID
	m.State = &State{
		Active:        true,
		TargetPane:    "%5",
		TargetSession: "my-session",
	}
	if got := m.GetTarget(); got != "%5" {
		t.Errorf("expected pane ID '%s', got %s", "%5", got)
	}

	// Without pane ID, returns session
	m.State.TargetPane = ""
	if got := m.GetTarget(); got != "my-session" {
		t.Errorf("expected session 'my-session', got %s", got)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ExitKey != "ctrl+\\" {
		t.Errorf("unexpected default ExitKey: %s", cfg.ExitKey)
	}
	if cfg.AttachKey != "ctrl+]" {
		t.Errorf("unexpected default AttachKey: %s", cfg.AttachKey)
	}
	if cfg.CopyKey != "alt+c" {
		t.Errorf("unexpected default CopyKey: %s", cfg.CopyKey)
	}
	if cfg.PasteKey != "alt+v" {
		t.Errorf("unexpected default PasteKey: %s", cfg.PasteKey)
	}
	if cfg.ScrollbackLines != 600 {
		t.Errorf("unexpected default ScrollbackLines: %d", cfg.ScrollbackLines)
	}
}
