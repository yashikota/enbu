package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	demoSecretsByEnv = map[string][]secretEntry{
		"development": {
			{key: "API_KEY", value: "sk-demo-abc123"},
			{key: "DATABASE_URL", value: "postgres://localhost:5432/enbu"},
			{key: "SECRET_TOKEN", value: "tok-enbu-xyz789"},
		},
		"production": {
			{key: "API_KEY", value: "sk-prod-abc123"},
			{key: "DATABASE_URL", value: "postgres://prod.example.com:5432/enbu"},
			{key: "SECRET_TOKEN", value: "tok-prod-xyz789"},
		},
		"staging": {
			{key: "API_KEY", value: "sk-stg-abc123"},
			{key: "DATABASE_URL", value: "postgres://staging.example.com:5432/enbu"},
		},
	}
	demoEnvs = []envItem{
		{name: "development", isCurrent: true},
		{name: "production", isCurrent: false},
		{name: "staging", isCurrent: false},
	}
	demoCurrent = "development"
)

func RunDemo() error {
	m := newDemoModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui demo: %w", err)
	}
	return nil
}

func newDemoModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	ki := textinput.New()
	ki.Placeholder = "KEY"
	ki.CharLimit = 256

	vi := textinput.New()
	vi.Placeholder = "VALUE"
	vi.CharLimit = 4096

	return model{
		app:        nil,
		view:       viewSecrets,
		spinner:    sp,
		keyInput:   ki,
		valueInput: vi,
		focusKey:   true,
		loading:    false,
		current:    demoCurrent,
		secrets:    demoSecretsByEnv[demoCurrent],
		envs:       demoEnvs,
	}
}
