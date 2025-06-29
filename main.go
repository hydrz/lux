package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/fatih/color"

	"github.com/hydrz/lux/app"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})))
}

func main() {
	if err := app.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(
			color.Output,
			"Run %s failed: %s\n",
			color.CyanString("%s", app.Name), color.RedString("%v", err),
		)
		os.Exit(1)
	}
}
