package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// cmdLog: autoresearch log <status> <metric> <description> [commit] [metrics_json] [asi_json]
func cmdLog(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: autoresearch log <status> <metric> <description> [commit] [metrics_json] [asi_json]")
	}

	status := args[0]
	metricStr := args[1]
	description := args[2]
	commit := ""
	metricsJSON := "{}"
	asiJSON := ""

	if len(args) >= 4 {
		commit = args[3]
	}
	if len(args) >= 5 {
		metricsJSON = args[4]
	}
	if len(args) >= 6 {
		asiJSON = args[5]
	}

	switch status {
	case "keep", "discard", "crash", "checks_failed":
	default:
		return fmt.Errorf("status must be keep|discard|crash|checks_failed, got %q", status)
	}

	var metric float64
	if _, err := fmt.Sscanf(metricStr, "%f", &metric); err != nil {
		return fmt.Errorf("metric must be a number, got %q", metricStr)
	}

	// Parse secondary metrics
	var metrics map[string]float64
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		metrics = make(map[string]float64)
	}

	// Parse ASI
	var asi map[string]any
	if asiJSON != "" {
		if err := json.Unmarshal([]byte(asiJSON), &asi); err != nil {
			asi = nil
		}
	}

	// --- Git operations ---
	if status == "keep" {
		gitExec("add", "-A")
		if !gitDiffCachedQuiet() {
			resultJSON := fmt.Sprintf(`{"status":"%s","metric":%g}`, status, metric)
			commitMsg := fmt.Sprintf("%s\n\nResult: %s", description, resultJSON)
			gitExec("commit", "-m", commitMsg, "--quiet")
		}
		if commit == "" {
			out, err := exec.Command("git", "rev-parse", "--short=7", "HEAD").Output()
			if err == nil {
				commit = strings.TrimSpace(string(out))
			} else {
				commit = "unknown"
			}
		}
		fmt.Println("📝 Git: committed")
	} else {
		// Revert — preserve autoresearch session files
		for _, f := range []string{"autoresearch.jsonl", "autoresearch.md", "autoresearch.ideas.md", "autoresearch.sh", "autoresearch.checks.sh"} {
			gitExec("add", f)
		}
		gitExec("checkout", "--", ".")
		exec.Command("git", "clean", "-fd").Run()
		fmt.Printf("📝 Git: reverted changes (%s) — autoresearch files preserved\n", status)
		if commit == "" {
			commit = "—"
		}
	}

	// --- Read session state ---
	state, err := ReadSession()
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", jsonlFile, err)
	}

	runNumber := state.RunCount + 1
	segment := state.Segment

	// --- Compute confidence ---
	conf := ComputeConfidence(state.Results, segment, state.Direction, metric, status)

	// --- Build and append JSONL entry ---
	entry := ExperimentResult{
		Run:             runNumber,
		Commit:          commit,
		Metric:          metric,
		Metrics:         metrics,
		Status:          status,
		Description:     description,
		Timestamp:       NowMillis(),
		Segment:         segment,
		Confidence:      conf,
		IterationTokens: nil,
		ASI:             asi,
	}
	if err := AppendJSONL(entry); err != nil {
		return fmt.Errorf("failed to write %s: %w", jsonlFile, err)
	}

	// --- Report ---
	fmt.Println()
	fmt.Printf("Logged #%d: %s — %s\n", runNumber, status, description)

	// Baseline info
	baseline := findBaseline(state.Results, segment)
	if baseline != nil {
		fmt.Printf("Baseline: %g", *baseline)
		if status == "keep" && metric > 0 && *baseline > 0 {
			delta := (metric - *baseline) / *baseline * 100
			sign := ""
			if delta > 0 {
				sign = "+"
			}
			fmt.Printf(" | this: %g (%s%.1f%%)", metric, sign, delta)
		}
		fmt.Println()
	}

	// Confidence
	if conf != nil {
		if *conf >= 2.0 {
			fmt.Printf("📊 Confidence: %.1f× noise floor — improvement is likely real\n", *conf)
		} else if *conf >= 1.0 {
			fmt.Printf("📊 Confidence: %.1f× noise floor — above noise but marginal\n", *conf)
		} else {
			fmt.Printf("⚠️ Confidence: %.1f× noise floor — within noise, consider re-running\n", *conf)
		}
	}

	fmt.Printf("\n(%d experiments in segment %d)\n", runNumber, segment)
	return nil
}

func findBaseline(results []ExperimentResult, segment int) *float64 {
	for _, r := range results {
		if r.Segment == segment {
			return &r.Metric
		}
	}
	return nil
}

func gitExec(args ...string) {
	exec.Command("git", args...).Run()
}

func gitDiffCachedQuiet() bool {
	err := exec.Command("git", "diff", "--cached", "--quiet").Run()
	return err == nil // true means no staged changes
}
