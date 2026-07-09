package tui

import "github.com/yashikota/enbu/app"

type secretsLoadedMsg struct {
	secrets map[string]string
	current string
}

type envsLoadedMsg struct {
	envs    []envItem
	current string
}

type recipientsLoadedMsg struct {
	recipients []app.RecipientInfo
}

type configLoadedMsg struct {
	content string
}

type configSavedMsg struct{}

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
