package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/commands"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "chagg",
		Usage: "A modern release-note workflow tool",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose logging (key operational steps)",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging (detailed internals, implies --verbose)",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			level := slog.LevelWarn // default: only warnings/errors, no operational noise
			if cmd.Bool("verbose") {
				level = slog.LevelInfo
			}
			if cmd.Bool("debug") {
				level = slog.LevelDebug
			}
			// Always replace the default handler so the Go runtime's default
			// (which outputs INFO and above) is never active.
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
			return ctx, nil
		},
		Commands: []*cli.Command{
			commands.InitCommand(changeentry.NewFileConfigIO()),
			commands.AddCommand(),
			commands.CheckCommand(),
			commands.LogCommand(),
			commands.GenerateCommand(),
			commands.ReleaseCommand(),
			commands.ConfigCommand(changeentry.NewFileConfigIO()),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		exitCode := changeentry.ExitCodeGeneral
		if codedErr, ok := errors.AsType[changeentry.CodedError](err); ok {
			exitCode = codedErr.ExitCode()
		}

		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode)
	}
}
