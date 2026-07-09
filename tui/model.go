package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yashikota/enbu/app"
)

type viewState int

const (
	viewSecrets viewState = iota
	viewAdd
	viewEdit
	viewConfirmDelete
	viewEnvSwitch
	viewRecipients
	viewConfig
)

type model struct {
	app     *app.App
	view    viewState
	secrets []secretEntry
	cursor  int
	envs    []envItem
	current string

	recipients []app.RecipientInfo

	configContent string
	configDraft   string
	configEditing bool
	configInput   textinput.Model

	keyInput   textinput.Model
	valueInput textinput.Model
	focusKey   bool

	spinner   spinner.Model
	loading   bool
	notInited bool
	err       error
	status    string

	width  int
	height int
}

type secretEntry struct {
	key   string
	value string
}

func newModel(a *app.App) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	ki := textinput.New()
	ki.Placeholder = "KEY"
	ki.CharLimit = 256

	vi := textinput.New()
	vi.Placeholder = "VALUE"
	vi.CharLimit = 4096

	ci := textinput.New()
	ci.Placeholder = "toml content"
	ci.CharLimit = 65536

	return model{
		app:         a,
		view:        viewSecrets,
		spinner:     sp,
		keyInput:    ki,
		valueInput:  vi,
		configInput: ci,
		focusKey:    true,
		loading:     true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadSecrets())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case secretsLoadedMsg:
		m.loading = false
		m.secrets = mapToEntries(msg.secrets)
		m.current = msg.current
		m.err = nil
		if m.cursor >= len(m.secrets) {
			m.cursor = max(0, len(m.secrets)-1)
		}
		return m, nil

	case envsLoadedMsg:
		m.loading = false
		m.envs = msg.envs
		m.current = msg.current
		m.cursor = 0
		m.err = nil
		return m, nil

	case recipientsLoadedMsg:
		m.loading = false
		m.recipients = msg.recipients
		m.err = nil
		return m, nil

	case configLoadedMsg:
		m.loading = false
		m.configContent = msg.content
		m.configDraft = msg.content
		m.configEditing = false
		m.err = nil
		return m, nil

	case configSavedMsg:
		m.loading = false
		m.configContent = m.configDraft
		m.configEditing = false
		m.status = "Config saved."
		m.view = viewSecrets
		return m, nil

	case operationDoneMsg:
		m.loading = true
		m.status = msg.message
		m.view = viewSecrets
		return m, m.loadSecrets()

	case errMsg:
		m.loading = false
		if app.IsNotInitializedError(msg.err) {
			m.notInited = true
			m.err = nil
		} else {
			m.notInited = false
			m.err = msg.err
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			if key.Matches(msg, keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}
		return m.handleKey(msg)
	}

	if m.view == viewAdd || m.view == viewEdit {
		return m.updateInputs(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewSecrets:
		return m.handleSecretsKey(msg)
	case viewAdd:
		return m.handleAddKey(msg)
	case viewEdit:
		return m.handleEditKey(msg)
	case viewConfirmDelete:
		return m.handleDeleteKey(msg)
	case viewEnvSwitch:
		return m.handleEnvSwitchKey(msg)
	case viewRecipients:
		return m.handleRecipientsKey(msg)
	case viewConfig:
		return m.handleConfigKey(msg)
	}
	return m, nil
}

func (m model) handleSecretsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.secrets)-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Add):
		m.view = viewAdd
		m.keyInput.Reset()
		m.valueInput.Reset()
		m.keyInput.Focus()
		m.focusKey = true
		m.err = nil
		m.status = ""
	case key.Matches(msg, keys.Edit):
		if len(m.secrets) > 0 {
			m.view = viewEdit
			m.valueInput.Reset()
			m.valueInput.SetValue(m.secrets[m.cursor].value)
			m.valueInput.Focus()
			m.err = nil
			m.status = ""
		}
	case key.Matches(msg, keys.Delete):
		if len(m.secrets) > 0 {
			m.view = viewConfirmDelete
			m.err = nil
			m.status = ""
		}
	case key.Matches(msg, keys.Switch):
		m.view = viewEnvSwitch
		m.loading = true
		m.err = nil
		m.status = ""
		m.cursor = 0
		return m, m.loadEnvs()
	case key.Matches(msg, keys.Refresh):
		m.loading = true
		return m, m.loadSecrets()
	case key.Matches(msg, keys.Recipients):
		m.view = viewRecipients
		m.loading = true
		m.err = nil
		m.status = ""
		return m, m.loadRecipients()
	case key.Matches(msg, keys.Config):
		m.view = viewConfig
		m.loading = true
		m.err = nil
		m.status = ""
		m.configEditing = false
		return m, m.loadConfig()
	}
	return m, nil
}

func (m model) handleAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.view = viewSecrets
		return m, nil
	case key.Matches(msg, keys.Tab):
		m.focusKey = !m.focusKey
		if m.focusKey {
			m.keyInput.Focus()
			m.valueInput.Blur()
		} else {
			m.keyInput.Blur()
			m.valueInput.Focus()
		}
		return m, nil
	case key.Matches(msg, keys.Enter):
		k := strings.TrimSpace(m.keyInput.Value())
		v := m.valueInput.Value()
		if k == "" {
			m.err = fmt.Errorf("key cannot be empty")
			return m, nil
		}
		m.loading = true
		m.view = viewSecrets
		return m, m.addSecret(k, v)
	}
	return m.updateInputs(msg)
}

func (m model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.view = viewSecrets
		return m, nil
	case key.Matches(msg, keys.Enter):
		v := m.valueInput.Value()
		k := m.secrets[m.cursor].key
		m.loading = true
		m.view = viewSecrets
		return m, m.editSecret(k, v)
	}
	return m.updateInputs(msg)
}

func (m model) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.view = viewSecrets
		return m, nil
	case key.Matches(msg, keys.Enter):
		k := m.secrets[m.cursor].key
		m.loading = true
		m.view = viewSecrets
		return m, m.deleteSecret(k)
	}
	return m, nil
}

func (m model) handleEnvSwitchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.view = viewSecrets
		m.cursor = 0
		return m, nil
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.envs)-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Enter):
		if len(m.envs) > 0 {
			name := m.envs[m.cursor].name
			m.loading = true
			return m, m.switchEnv(name)
		}
	}
	return m, nil
}

func (m model) handleRecipientsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Escape) || key.Matches(msg, keys.Quit) {
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		m.view = viewSecrets
	}
	return m, nil
}

func (m model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Escape):
		if m.configEditing {
			m.configEditing = false
			m.configDraft = m.configContent
			return m, nil
		}
		m.view = viewSecrets
	case key.Matches(msg, keys.Edit):
		if !m.configEditing {
			m.configEditing = true
			m.configInput.SetValue(m.configContent)
			m.configInput.Focus()
			return m, nil
		}
	case key.Matches(msg, keys.Enter):
		if m.configEditing {
			m.configDraft = m.configInput.Value()
			m.loading = true
			return m, m.saveConfig(m.configDraft)
		}
	}
	if m.configEditing {
		var cmd tea.Cmd
		m.configInput, cmd = m.configInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.keyInput, cmd = m.keyInput.Update(msg)
	cmds = append(cmds, cmd)
	m.valueInput, cmd = m.valueInput.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Loading...\n", m.spinner.View())
	}

	switch m.view {
	case viewSecrets:
		return m.viewSecrets()
	case viewAdd:
		return m.viewAdd()
	case viewEdit:
		return m.viewEdit()
	case viewConfirmDelete:
		return m.viewConfirmDelete()
	case viewEnvSwitch:
		return m.viewEnvSwitch()
	case viewRecipients:
		return m.viewRecipientsView()
	case viewConfig:
		return m.viewConfigView()
	}
	return ""
}

func (m model) viewSecrets() string {
	var b strings.Builder

	env := m.current
	if env == "" {
		env = "default"
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("enbu — %s", env)))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render("  Error: "+m.err.Error()) + "\n\n")
	}
	if m.status != "" {
		b.WriteString(successStyle.Render("  ✓ "+m.status) + "\n\n")
	}

	if m.notInited {
		b.WriteString(dimStyle.Render("  Not initialized. Run 'enbu init' to get started."))
		b.WriteString("\n")
	} else if len(m.secrets) == 0 && m.err == nil {
		b.WriteString(dimStyle.Render("  No secrets yet. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		maxKeyLen := 0
		for _, s := range m.secrets {
			if len(s.key) > maxKeyLen {
				maxKeyLen = len(s.key)
			}
		}

		for i, s := range m.secrets {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "▸ "
				style = selectedStyle
			}
			line := fmt.Sprintf("%s%-*s = %s", cursor, maxKeyLen, s.key, s.value)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("  a:add  e:edit  d:delete  s:switch env  r:refresh  R:recipients  C:config  q:quit"))
	return b.String()
}

func (m model) viewAdd() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add Secret"))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render("  "+m.err.Error()) + "\n\n")
	}

	b.WriteString(inputLabelStyle.Render("  Key:   "))
	b.WriteString(m.keyInput.View())
	b.WriteString("\n")
	b.WriteString(inputLabelStyle.Render("  Value: "))
	b.WriteString(m.valueInput.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  tab:switch field  enter:confirm  esc:cancel"))
	return b.String()
}

func (m model) viewEdit() string {
	var b strings.Builder
	k := m.secrets[m.cursor].key
	b.WriteString(titleStyle.Render(fmt.Sprintf("Edit: %s", k)))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render("  "+m.err.Error()) + "\n\n")
	}

	b.WriteString(inputLabelStyle.Render("  Value: "))
	b.WriteString(m.valueInput.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  enter:confirm  esc:cancel"))
	return b.String()
}

func (m model) viewConfirmDelete() string {
	var b strings.Builder
	k := m.secrets[m.cursor].key
	b.WriteString(titleStyle.Render("Delete Secret"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Are you sure you want to delete %s?\n", selectedStyle.Render(k))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  enter:confirm  esc:cancel"))
	return b.String()
}

func (m model) viewEnvSwitch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Switch Environment"))
	b.WriteString("\n")

	for i, env := range m.envs {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		marker := ""
		if env.isCurrent {
			marker = " (current)"
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s%s", cursor, env.name, marker)))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  enter:switch  esc:back"))
	return b.String()
}

func (m model) viewRecipientsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Recipients"))
	b.WriteString("\n")

	if len(m.recipients) == 0 {
		b.WriteString(dimStyle.Render("  No recipients found."))
		b.WriteString("\n")
	} else {
		for _, r := range m.recipients {
			fmt.Fprintf(&b, "  %s\n", selectedStyle.Render(r.Username))
			fmt.Fprintf(&b, "    %s\n", dimStyle.Render(r.Fingerprint))
		}
	}

	b.WriteString(helpStyle.Render("  esc:back"))
	return b.String()
}

func (m model) viewConfigView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("enbu.toml"))
	b.WriteString("\n")

	if m.configEditing {
		b.WriteString(inputLabelStyle.Render("  Content (enter:save  esc:cancel):"))
		b.WriteString("\n")
		b.WriteString(m.configInput.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  enter:save  esc:cancel"))
	} else {
		if m.configContent == "" {
			b.WriteString(dimStyle.Render("  (empty)"))
		} else {
			for _, line := range strings.Split(m.configContent, "\n") {
				b.WriteString("  " + line + "\n")
			}
		}
		b.WriteString(helpStyle.Render("  e:edit  esc:back"))
	}
	return b.String()
}

func (m model) loadSecrets() tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			secrets := make(map[string]string)
			for _, e := range demoSecretsByEnv[demoCurrent] {
				secrets[e.key] = e.value
			}
			return secretsLoadedMsg{secrets: secrets, current: demoCurrent}
		}
		secrets, err := m.app.ListSecrets(context.Background(), "")
		if err != nil {
			return errMsg{err}
		}
		cur, _ := m.app.CurrentEnvironment()
		return secretsLoadedMsg{secrets: secrets, current: cur}
	}
}

func (m model) loadEnvs() tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			return envsLoadedMsg{envs: demoEnvs, current: demoCurrent}
		}
		envs, err := m.app.ListEnvironments()
		if err != nil {
			return errMsg{err}
		}
		var items []envItem
		var current string
		for _, e := range envs {
			items = append(items, envItem{name: e.Name, isCurrent: e.IsCurrent})
			if e.IsCurrent {
				current = e.Name
			}
		}
		return envsLoadedMsg{envs: items, current: current}
	}
}

func (m model) addSecret(key, value string) tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			secrets := demoSecretsByEnv[demoCurrent]
			found := false
			for i, s := range secrets {
				if s.key == key {
					secrets[i].value = value
					found = true
					break
				}
			}
			if !found {
				secrets = append(secrets, secretEntry{key: key, value: value})
			}
			demoSecretsByEnv[demoCurrent] = secrets
			return operationDoneMsg{message: fmt.Sprintf("Added %s", key)}
		}
		if err := m.app.AddSecret(context.Background(), "", key, value); err != nil {
			return errMsg{err}
		}
		return operationDoneMsg{message: fmt.Sprintf("Added %s", key)}
	}
}

func (m model) editSecret(key, value string) tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			secrets := demoSecretsByEnv[demoCurrent]
			for i, s := range secrets {
				if s.key == key {
					secrets[i].value = value
					break
				}
			}
			demoSecretsByEnv[demoCurrent] = secrets
			return operationDoneMsg{message: fmt.Sprintf("Updated %s", key)}
		}
		if err := m.app.EditSecret(context.Background(), "", key, value); err != nil {
			return errMsg{err}
		}
		return operationDoneMsg{message: fmt.Sprintf("Updated %s", key)}
	}
}

func (m model) deleteSecret(key string) tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			var updated []secretEntry
			for _, s := range demoSecretsByEnv[demoCurrent] {
				if s.key != key {
					updated = append(updated, s)
				}
			}
			demoSecretsByEnv[demoCurrent] = updated
			return operationDoneMsg{message: fmt.Sprintf("Deleted %s", key)}
		}
		if err := m.app.DeleteSecret(context.Background(), "", key); err != nil {
			return errMsg{err}
		}
		return operationDoneMsg{message: fmt.Sprintf("Deleted %s", key)}
	}
}

func (m model) loadRecipients() tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			return recipientsLoadedMsg{recipients: demoRecipients}
		}
		list, err := m.app.ListRecipients(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return recipientsLoadedMsg{recipients: list}
	}
}

func (m model) loadConfig() tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			return configLoadedMsg{content: demoConfigContent}
		}
		content, err := m.app.ReadConfig()
		if err != nil {
			return errMsg{err}
		}
		return configLoadedMsg{content: content}
	}
}

func (m model) saveConfig(content string) tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			demoConfigContent = content
			return configSavedMsg{}
		}
		if err := m.app.WriteConfig(content); err != nil {
			return errMsg{err}
		}
		return configSavedMsg{}
	}
}

func (m model) switchEnv(name string) tea.Cmd {
	return func() tea.Msg {
		if m.app == nil {
			demoCurrent = name
			for i, env := range demoEnvs {
				demoEnvs[i].isCurrent = (env.name == name)
			}
			return operationDoneMsg{message: fmt.Sprintf("Switched to %s", name)}
		}
		if err := m.app.SwitchEnvironment(name); err != nil {
			return errMsg{err}
		}
		return operationDoneMsg{message: fmt.Sprintf("Switched to %s", name)}
	}
}

func mapToEntries(secrets map[string]string) []secretEntry {
	entries := make([]secretEntry, 0, len(secrets))
	for k, v := range secrets {
		entries = append(entries, secretEntry{key: k, value: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})
	return entries
}
