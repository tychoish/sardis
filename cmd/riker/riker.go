package main

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func resolveConfiguration(ctx context.Context, cc *cli.Context) (*sardis.Configuration, error) {
	conf, err := sardis.LoadConfiguration(cc.GlobalString("conf"))
	if err != nil {
		return nil, err
	}
	conf.Settings.Logging.EnableJSONFormating = cc.Bool("jsonLog")
	conf.Settings.Logging.DisableStandardOutput = cc.Bool("quietStdOut")
	conf.Settings.Logging.Priority = level.FromString(cc.String("level"))

	return conf, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	TopLevel()

	cmdr.Main(ctx, cmd)
}
