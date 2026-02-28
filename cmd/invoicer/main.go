package main

import (
	"github.com/alecthomas/kong"
	"github.com/zon/invoicer/internal/cli"
)

func main() {
	var cmd cli.CLI
	ctx := kong.Parse(&cmd,
		kong.Name("invoicer"),
		kong.Description("Generate invoices for an hourly contractor."),
		kong.UsageOnError(),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
