package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/urfave/cli/v3"
)

func AddCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Aliases:   []string{"a"},
		Usage:     "Create a new change entry",
		ArgsUsage: "[path]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: changeentry.TypeFlagUsage()},
			&cli.BoolFlag{Name: "breaking", Usage: "Mark this entry as breaking"},
			&cli.StringFlag{Name: "component", Usage: "Component(s), comma separated"},
			&cli.StringFlag{Name: "audience", Usage: "Audience(s), comma separated"},
			&cli.IntFlag{Name: "priority", Usage: "Priority order in changelog"},
			&cli.StringFlag{Name: "issue", Usage: "Issue reference(s), comma separated"},
			&cli.StringFlag{Name: "release", Usage: "Pin entry to release version"},
			&cli.StringFlag{Name: "body", Usage: "Markdown body content"},
			&cli.BoolFlag{Name: "no-prompt", Usage: "Disable interactive prompts and use defaults for omitted fields"},
		},
		Action: addAction,
	}
}

func addAction(_ context.Context, cmd *cli.Command) error {
	params := changeentry.Params{
		Type:         cmd.String("type"),
		TypeSet:      cmd.IsSet("type"),
		Breaking:     cmd.Bool("breaking"),
		BreakingSet:  cmd.IsSet("breaking"),
		Component:    cmd.String("component"),
		ComponentSet: cmd.IsSet("component"),
		Audience:     cmd.String("audience"),
		AudienceSet:  cmd.IsSet("audience"),
		Priority:     cmd.Int("priority"),
		PrioritySet:  cmd.IsSet("priority"),
		Issue:        cmd.String("issue"),
		IssueSet:     cmd.IsSet("issue"),
		Release:      cmd.String("release"),
		ReleaseSet:   cmd.IsSet("release"),
		Body:         cmd.String("body"),
		BodySet:      cmd.IsSet("body"),
	}

	interactive := isInteractiveStdin() && !cmd.Bool("no-prompt")
	path, err := changeentry.CreateChange(".", cmd.Args().Get(0), params, os.Stdin, os.Stdout, interactive)
	if err != nil {
		return err
	}

	fmt.Printf("Created %s\n", path)
	return nil
}

func isInteractiveStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}
