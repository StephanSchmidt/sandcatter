package builtin

import (
	"embed"
)

//go:embed plugins/*
var Plugins embed.FS
