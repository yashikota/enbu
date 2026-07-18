package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testModel() *model {
	m := newModel(nil)
	m.loading = false
	m.width = 90
	m.height = 24
	m.current = "development"
	m.repository = "owner/repo"
	m.envs = []envItem{{name: "development", isCurrent: true}, {name: "production"}}
	m.secrets = []secretEntry{{key: "API_KEY", value: "super-secret"}, {key: "DATABASE_URL", value: "postgres://db"}}
	m.resizeInputs()
	return m
}

func keyMsg(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func findHit(t *testing.T, m *model, kind hitKind, index int) hitRegion {
	t.Helper()
	for _, hit := range m.hits {
		if hit.kind == kind && hit.index == index {
			return hit
		}
	}
	t.Fatalf("hit region kind=%d index=%d not found", kind, index)
	return hitRegion{}
}

func click(hit hitRegion) tea.MouseMsg {
	return tea.MouseMsg{
		X:      hit.x,
		Y:      hit.y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
}

func TestSecretsAreMaskedAndCanBeRevealed(t *testing.T) {
	m := testModel()
	view := m.View()
	if strings.Contains(view, "super-secret") {
		t.Fatal("secret is visible before reveal")
	}

	_, _ = m.Update(keyMsg(" "))
	if view = m.View(); !strings.Contains(view, "super-secret") {
		t.Fatal("secret is not visible after reveal")
	}

	reveal := findHit(t, m, hitReveal, 0)
	_, _ = m.Update(click(reveal))
	if view = m.View(); strings.Contains(view, "super-secret") {
		t.Fatal("secret remains visible after mouse toggle")
	}
}

func TestMouseSwitchesTabsAndRefreshesMembers(t *testing.T) {
	m := testModel()
	_ = m.View()
	members := findHit(t, m, hitTab, int(tabMembers))
	_, cmd := m.Update(click(members))
	if m.tab != tabMembers || !m.loading || cmd == nil {
		t.Fatalf("members tab state = tab %d loading %v cmd %v", m.tab, m.loading, cmd != nil)
	}

	msg := cmd()
	_, _ = m.Update(msg)
	if m.loading || len(m.recipients) == 0 {
		t.Fatalf("members did not load: loading=%v recipients=%d", m.loading, len(m.recipients))
	}
}

func TestMouseWheelScrollsLongSecretList(t *testing.T) {
	m := testModel()
	m.height = 16
	m.secrets = make([]secretEntry, 20)
	for i := range m.secrets {
		m.secrets[i] = secretEntry{key: "KEY", value: "VALUE"}
	}
	_ = m.View()
	_, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if m.offset != 3 {
		t.Fatalf("offset = %d, want 3", m.offset)
	}
}

func TestAddOverlaySupportsMouseFocusAndKeepsErrors(t *testing.T) {
	m := testModel()
	_ = m.View()
	add := findHit(t, m, hitAdd, 0)
	_, _ = m.Update(click(add))
	if m.overlay != overlayAdd || !m.keyInput.Focused() {
		t.Fatal("add overlay did not open with key focused")
	}

	_ = m.View()
	value := findHit(t, m, hitInputValue, 0)
	_, _ = m.Update(click(value))
	if !m.valueInput.Focused() || m.keyInput.Focused() {
		t.Fatal("mouse did not focus the value input")
	}

	_, _ = m.Update(errMsg{err: errors.New("network failed")})
	if m.overlay != overlayAdd || m.err == nil {
		t.Fatal("operation error closed the overlay or was lost")
	}
}

func TestCopyUsesInjectedClipboard(t *testing.T) {
	m := testModel()
	var copied string
	m.copyToClipboard = func(value string) error {
		copied = value
		return nil
	}
	_, cmd := m.copySecret(0, false)
	if cmd == nil {
		t.Fatal("copy command is nil")
	}
	_, _ = m.Update(cmd())
	if copied != "super-secret" || m.status != "Copied value" {
		t.Fatalf("copied=%q status=%q", copied, m.status)
	}
}

func TestSettingsEditorRejectsInvalidOutput(t *testing.T) {
	m := testModel()
	m.tab = tabSettings
	m.configContent = "version = \"v1alpha1\"\n"
	_, _ = m.startConfigEdit()
	m.configInput.SetValue("version = \"v1alpha1\"\n[env.default]\noutput = \"../outside\"\n")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil || m.err == nil || !m.configEditing {
		t.Fatalf("invalid config state: cmd=%v err=%v editing=%v", cmd != nil, m.err, m.configEditing)
	}
}

func TestEnvironmentChangeRemasksSecrets(t *testing.T) {
	m := testModel()
	m.revealed["API_KEY"] = true
	_, _ = m.Update(operationDoneMsg{message: "Switched"})
	if m.revealed["API_KEY"] {
		t.Fatal("revealed state survived workspace-changing operation")
	}
}
