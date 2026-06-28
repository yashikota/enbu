package tui

type secretsLoadedMsg struct {
	secrets map[string]string
}

type envsLoadedMsg struct {
	envs    []envItem
	current string
}

type operationDoneMsg struct {
	message string
}

type errMsg struct {
	err error
}

type envItem struct {
	name      string
	isCurrent bool
}
