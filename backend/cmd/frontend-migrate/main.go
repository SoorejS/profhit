package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"profhit-backend/internal/frontendmigration"
)

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("frontend-migrate", flag.ContinueOnError)
	flags.SetOutput(output)
	root := flags.String("root", "..", "explicit project directory (default: parent of backend)")
	operation := flags.String("operation", "", "refactor-js, refactor-pages, update-html-css, or update-html-scripts")
	apply := flags.Bool("apply", false, "apply the previewed changes after creating a backup")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	plan, err := frontendmigration.Prepare(*root, *operation)
	if err != nil {
		return err
	}
	for _, change := range plan.Changes {
		action := "write"
		if change.Delete {
			action = "remove legacy source"
		}
		fmt.Fprintf(output, "%s: %s\n", action, change.Path)
	}
	fmt.Fprintf(output, "%d planned file changes\n", len(plan.Changes))
	if !*apply {
		fmt.Fprintln(output, "Preview only. No files changed. Use -apply after reviewing the plan.")
		return nil
	}
	backup, err := frontendmigration.Apply(plan)
	if backup != "" {
		fmt.Fprintf(output, "Originals and manifest backed up at: %s\n", backup)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(output, "Migration applied.")
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Migration stopped:", err)
		os.Exit(1)
	}
}
