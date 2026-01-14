package main

import (
	"embed"

	"github.com/quasilyte/go-ruleguard/ruleguard"
	"github.com/tychoish/grip"
)

//go:embed rules/*.go
var ruleFS embed.FS

func main() {
	eng := ruleguard.NewEngine()
	grip.Info(eng)
}
