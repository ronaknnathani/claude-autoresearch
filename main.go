package main

import (
	"fmt"
	"os"
)

const usage = `Usage: autoresearch <command> [args]

Commands:
  init   Initialize experiment session (name, metric, unit, direction)
  run    Run a timed experiment command
  log    Record experiment result (commit/revert, confidence)
  serve  Launch live browser dashboard
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "log":
		err = cmdLog(os.Args[2:])
	case "serve":
		err = cmdServe(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n%s", os.Args[1], usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
