package web

import (
	"embed"
	"io/fs"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func FrontendFS() fs.FS {
	sub, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return sub
}
