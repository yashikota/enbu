package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yashikota/enbu/app"
)

func Run(a *app.App) error {
	m := newModel(a)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
