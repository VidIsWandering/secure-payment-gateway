package web

import "embed"

//go:embed public/* pages/* css/* js/*
var FS embed.FS
