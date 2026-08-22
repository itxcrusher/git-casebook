package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/itxcrusher/repo-rehab/internal/app"
	"github.com/itxcrusher/repo-rehab/internal/version"
)

const usageText = `repo-rehab preserves uncertain Git sources and proves their provenance.

Usage:
  repo-rehab investigate <source> [<source> ...] [--case <directory>]
  repo-rehab case init [--case <directory>] [--id <case-id>] [--operator <id>]
  repo-rehab source add [--case <directory>] [--id <source-id>] [--role <role>] [--kind <kind>] <source>
  repo-rehab preserve [--case <directory>]
  repo-rehab inspect [--case <directory>]
  repo-rehab compare [--case <directory>]
  repo-rehab refs plan [--case <directory>]
  repo-rehab verify [--case <directory>]
  repo-rehab report [--case <directory>]

Global options may appear anywhere:
  --case <directory>   Case directory (default .case)
  --json               Emit a machine-readable result
  --json-errors        Emit errors as JSON on stderr
  --help               Show help
  --version            Show version

v0.1 implements safety Levels 0-2 only. It has no execution, install, rewrite,
push, or publication command.`

type globalOptions struct {
	caseRoot   string
	jsonOutput bool
	jsonErrors bool
}

func main() {
	code := run(os.Args[1:])
	os.Exit(code)
}

func run(raw []string) int {
	args, global, err := extractGlobals(raw)
	if err != nil {
		return fail(global, "USAGE", err, 2)
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(usageText)
		return 0
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Println(version.Value)
		return 0
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	application := app.App{CaseRoot: global.caseRoot}

	switch args[0] {
	case "case":
		if len(args) < 2 || args[1] != "init" {
			return fail(global, "USAGE", fmt.Errorf("expected 'case init'"), 2)
		}
		fs := flag.NewFlagSet("case init", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		caseID := fs.String("id", "", "stable case id")
		operator := fs.String("operator", "local-operator", "operator identifier")
		if err := fs.Parse(args[2:]); err != nil || fs.NArg() != 0 {
			if err == nil {
				err = fmt.Errorf("case init accepts no positional arguments")
			}
			return fail(global, "USAGE", err, 2)
		}
		policy, err := application.Init(*caseID, *operator)
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		return success(global, map[string]any{"case_id": policy.CaseID, "case_directory": global.caseRoot, "status": "OPEN"}, fmt.Sprintf("Initialized case %s at %s", policy.CaseID, global.caseRoot))
	case "source":
		if len(args) < 2 || args[1] != "add" {
			return fail(global, "USAGE", fmt.Errorf("expected 'source add'"), 2)
		}
		fs := flag.NewFlagSet("source add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sourceID := fs.String("id", "", "stable source id")
		role := fs.String("role", "UNKNOWN", "ORIGINAL, FORK, MIRROR, ARCHIVE, or UNKNOWN")
		kind := fs.String("kind", "auto", "auto, local, or remote")
		if err := fs.Parse(args[2:]); err != nil || fs.NArg() != 1 {
			if err == nil {
				err = fmt.Errorf("source add requires exactly one source locator")
			}
			return fail(global, "USAGE", err, 2)
		}
		policy, err := application.AddSource(*sourceID, fs.Arg(0), strings.ToUpper(*role), strings.ToLower(*kind))
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		id := policy.Sources[len(policy.Sources)-1].SourceID
		return success(global, map[string]any{"source_id": id, "source_count": len(policy.Sources)}, fmt.Sprintf("Declared source %s", id))
	case "preserve":
		if len(args) != 1 {
			return fail(global, "USAGE", fmt.Errorf("preserve accepts no positional arguments"), 2)
		}
		c, err := application.Preserve(ctx)
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		return success(global, map[string]any{"case_id": c.CaseID, "source_count": c.SourceCount, "status": c.Status}, fmt.Sprintf("Preserved %d source(s)", c.SourceCount))
	case "inspect":
		if len(args) != 1 {
			return fail(global, "USAGE", fmt.Errorf("inspect accepts no positional arguments"), 2)
		}
		c, err := application.Inspect(ctx)
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		return success(global, map[string]any{"case_id": c.CaseID, "source_count": c.SourceCount, "status": c.Status}, fmt.Sprintf("Inspected %d preserved source(s)", c.SourceCount))
	case "compare":
		if len(args) != 1 {
			return fail(global, "USAGE", fmt.Errorf("compare accepts no positional arguments"), 2)
		}
		c, err := application.Compare()
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		return success(global, map[string]any{"case_id": c.CaseID, "relationship_count": len(c.Relationships)}, fmt.Sprintf("Created %d directional relationship record(s)", len(c.Relationships)))
	case "refs":
		if len(args) != 2 || args[1] != "plan" {
			return fail(global, "USAGE", fmt.Errorf("expected 'refs plan'"), 2)
		}
		c, err := application.PlanRefs(ctx)
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		return success(global, map[string]any{"case_id": c.CaseID, "status": c.Status}, "Created collision-safe archival ref plan")
	case "verify":
		if len(args) != 1 {
			return fail(global, "USAGE", fmt.Errorf("verify accepts no positional arguments"), 2)
		}
		verification, err := application.Verify(ctx)
		if err != nil {
			return fail(global, "VERIFICATION_FAILED", err, 1)
		}
		message := "Case verified and ready for human review"
		if !verification.Ready {
			message = "Case verified with fail-closed incomplete evidence"
		}
		return success(global, verification, message)
	case "report":
		if len(args) != 1 {
			return fail(global, "USAGE", fmt.Errorf("report accepts no positional arguments"), 2)
		}
		path, err := application.Report()
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		return success(global, map[string]any{"report": path}, "Generated "+path)
	case "investigate":
		fs := flag.NewFlagSet("investigate", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		operator := fs.String("operator", "local-operator", "operator identifier")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() < 1 {
			if err == nil {
				err = fmt.Errorf("investigate requires at least one source locator")
			}
			return fail(global, "USAGE", err, 2)
		}
		verification, err := application.Investigate(ctx, fs.Args(), *operator)
		if err != nil {
			return fail(global, "OPERATION_FAILED", err, 1)
		}
		message := "Investigation complete; case is ready for human review"
		if !verification.Ready {
			message = "Investigation complete with fail-closed incomplete evidence"
		}
		return success(global, verification, message)
	default:
		return fail(global, "USAGE", fmt.Errorf("unknown command %q", args[0]), 2)
	}
}

func extractGlobals(args []string) ([]string, globalOptions, error) {
	options := globalOptions{caseRoot: ".case"}
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			options.jsonOutput = true
		case args[i] == "--json-errors":
			options.jsonErrors = true
		case args[i] == "--case":
			if i+1 >= len(args) {
				return nil, options, fmt.Errorf("--case requires a directory")
			}
			i++
			options.caseRoot = args[i]
		case strings.HasPrefix(args[i], "--case="):
			options.caseRoot = strings.TrimPrefix(args[i], "--case=")
		default:
			result = append(result, args[i])
		}
	}
	return result, options, nil
}

func success(options globalOptions, value any, message string) int {
	if options.jsonOutput {
		b, err := json.Marshal(value)
		if err != nil {
			return fail(options, "ENCODING_FAILED", err, 1)
		}
		fmt.Println(string(b))
	} else {
		fmt.Println(message)
	}
	return 0
}

func fail(options globalOptions, code string, err error, exitCode int) int {
	if options.jsonErrors {
		b, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": err.Error()}, "exit_code": exitCode})
		fmt.Fprintln(os.Stderr, string(b))
	} else {
		fmt.Fprintf(os.Stderr, "repo-rehab: %s: %v\n", code, err)
	}
	return exitCode
}
