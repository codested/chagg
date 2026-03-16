package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/commands"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "chagg",
		Usage: "A modern release-note workflow tool",
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
