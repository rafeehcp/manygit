package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDoubleEscapeFooterHint(t *testing.T) {
	at := time.Now()
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	next, expire := (Model{statusLine: "fetch complete"}).handleKeyAt(esc, at)
	m := next.(Model)
	if got := stripANSI(m.statusOrFilterLine()); got != "Press Esc again to quit" {
		t.Fatalf("first Escape footer = %q", got)
	}
	if expire == nil {
		t.Fatal("first Escape must schedule a redraw when the hint expires")
	}
	msg := expire()
	if _, ok := msg.(escapeExpireMsg); !ok {
		t.Fatalf("expiration command returned %T", msg)
	}
	next, _ = m.Update(msg)
	if got := stripANSI(next.(Model).statusOrFilterLine()); got != "fetch complete" {
		t.Fatalf("expiration should restore the existing status, got %q", got)
	}

	// A stale timer cannot clear a newer press or its hint.
	next, _ = m.handleKeyAt(esc, at.Add(time.Second))
	next, _ = next.(Model).Update(msg)
	if !strings.Contains(next.(Model).statusOrFilterLine(), "Press Esc again to quit") {
		t.Fatal("stale expiration cleared the new hint")
	}
	next, _ = next.(Model).handleKeyAt(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}, at.Add(time.Second+time.Millisecond))
	if strings.Contains(next.(Model).statusOrFilterLine(), "Press Esc again to quit") {
		t.Fatal("another key should clear the hint immediately")
	}
}

func TestDoubleEscapeHintWhileSettingsRemainOpen(t *testing.T) {
	m := Model{showHelp: true, editingOpenCmd: true, width: 120, height: 40}
	next, _ := m.handleKeyAt(tea.KeyMsg{Type: tea.KeyEsc}, time.Now())
	m = next.(Model)
	if !m.showHelp || m.editingOpenCmd {
		t.Fatal("first Escape should close only the editor setting input")
	}
	if !strings.Contains(m.View(), "Press Esc again to quit") {
		t.Fatal("settings must show the quit hint while the overlay remains open")
	}
}

func escapeKeyAt(m Model, key tea.KeyMsg, at time.Time) (Model, bool) {
	next, cmd := m.handleKeyAt(key, at)
	quit := false
	if cmd != nil {
		_, quit = cmd().(tea.QuitMsg)
	}
	return next.(Model), quit
}

func TestDoubleEscapeTiming(t *testing.T) {
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	at := time.Now()
	for _, tc := range []struct {
		name string
		gap  time.Duration
		quit bool
	}{
		{"rapid", 100 * time.Millisecond, true},
		{"boundary", doubleEscapeWindow, true},
		{"expired", doubleEscapeWindow + time.Nanosecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, quit := escapeKeyAt(Model{}, esc, at)
			if quit {
				t.Fatal("first Escape quit")
			}
			m, quit = escapeKeyAt(m, esc, at.Add(tc.gap))
			if quit != tc.quit {
				t.Fatalf("quit = %v, want %v", quit, tc.quit)
			}
			if !quit {
				_, quit = escapeKeyAt(m, esc, at.Add(tc.gap+time.Millisecond))
				if !quit {
					t.Fatal("expired press did not start a fresh pair")
				}
			}
		})
	}
}

func TestDoubleEscapeOtherKeyResets(t *testing.T) {
	at := time.Now()
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	m, _ := escapeKeyAt(Model{}, esc, at)
	m, _ = escapeKeyAt(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}, at.Add(time.Millisecond))
	_, quit := escapeKeyAt(m, esc, at.Add(2*time.Millisecond))
	if quit {
		t.Fatal("Escape after another key quit")
	}
}

func TestDoubleEscapeAcrossLayers(t *testing.T) {
	at := time.Now()
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	for _, tc := range []struct {
		name   string
		model  Model
		closed func(Model) bool
	}{
		{"help", Model{showHelp: true}, func(m Model) bool { return !m.showHelp }},
		{"graph", Model{showGraph: true}, func(m Model) bool { return !m.showGraph }},
		{"news", Model{showNews: true}, func(m Model) bool { return !m.showNews }},
		{"changelog", Model{showChangelog: true}, func(m Model) bool { return !m.showChangelog }},
		{"filter", Model{filtering: true}, func(m Model) bool { return !m.filtering }},
		{"shell", Model{shellPrompting: true}, func(m Model) bool { return !m.shellPrompting }},
		{"ai", Model{aiPrompting: true}, func(m Model) bool { return !m.aiPrompting }},
		{"discard", Model{confirmDiscard: true}, func(m Model) bool { return !m.confirmDiscard }},
		{"plan", Model{confirmPlan: true}, func(m Model) bool { return !m.confirmPlan }},
		{"zoom", Model{zoomed: true}, func(m Model) bool { return !m.zoomed }},
		{"diff", Model{focus: panelBottom, bottomView: bvChanges, changeShowDiff: true}, func(m Model) bool { return !m.changeShowDiff && m.bottomView == bvChanges }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, quit := escapeKeyAt(tc.model, esc, at)
			if quit || !tc.closed(m) {
				t.Fatal("first Escape did not back out normally")
			}
			// Background UI messages must not interrupt a consecutive key sequence.
			next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			_, quit = escapeKeyAt(next.(Model), esc, at.Add(100*time.Millisecond))
			if !quit {
				t.Fatal("second Escape did not quit")
			}
		})
	}
}
