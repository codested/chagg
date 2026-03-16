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
				Name:  "debug",
				Usage: "Enable debug logging",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Bool("debug") {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}
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
