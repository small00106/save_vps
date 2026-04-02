package public

import "embed"

//go:embed dist/*
var DistFS embed.FS
