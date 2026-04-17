package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var metricLineRe = regexp.MustCompile(`(?m)^METRIC\s+([\w.µ]+)=(\S+)\s*$`)

var deniedMetricNames = map[string]bool{
	"__proto__":   true,
	"constructor": true,
	"prototype":   true,
}

func parseMetricLines(output string) map[string]float64 {
	metrics := make(map[string]float64)
	for _, match := range metricLineRe.FindAllStringSubmatch(output, -1) {
		name := match[1]
		if deniedMetricNames[name] {
			continue
		}
		val, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}
		metrics[name] = val
	}
	return metrics
}

// cmdRun: autoresearch run <command> [timeout_seconds] [checks_timeout_seconds]
func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: autoresearch run <command> [timeout_seconds] [checks_timeout_seconds]")
	}

	command := args[0]
	timeout := 600
	checksTimeout := 300

	if len(args) >= 2 {
		if v, err := strconv.Atoi(args[1]); err == nil {
			timeout = v
		}
	}
	if len(args) >= 3 {
		if v, err := strconv.Atoi(args[2]); err == nil {
			checksTimeout = v
		}
	}

	// Run the benchmark
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	t0 := time.Now()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output, cmdErr := cmd.CombinedOutput()
	duration := time.Since(t0)
	durationSec := duration.Seconds()

	exitCode := 0
	timedOut := ctx.Err() == context.DeadlineExceeded
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if !timedOut {
			exitCode = -1
		}
	}

	benchmarkPassed := exitCode == 0 && !timedOut
	outputStr := string(output)

	// Parse METRIC lines
	parsedMetrics := parseMetricLines(outputStr)

	// Run checks if benchmark passed and file exists
	checksPass := "null"
	checksTimedOut := false
	checksOutput := ""
	checksDuration := 0.0

	if benchmarkPassed {
		if _, err := os.Stat("autoresearch.checks.sh"); err == nil {
			checksCtx, checksCancel := context.WithTimeout(context.Background(), time.Duration(checksTimeout)*time.Second)
			defer checksCancel()

			ct0 := time.Now()
			checksCmd := exec.CommandContext(checksCtx, "bash", "autoresearch.checks.sh")
			checksCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			checksOut, checksErr := checksCmd.CombinedOutput()
			checksDuration = time.Since(ct0).Seconds()

			checksTimedOut = checksCtx.Err() == context.DeadlineExceeded
			if checksTimedOut {
				checksPass = "false"
			} else if checksErr != nil {
				checksPass = "false"
			} else {
				checksPass = "true"
			}

			// Keep last 80 lines
			lines := strings.Split(string(checksOut), "\n")
			if len(lines) > 80 {
				lines = lines[len(lines)-80:]
			}
			checksOutput = strings.Join(lines, "\n")
		}
	}

	// Output report
	fmt.Println("═══ Experiment Result ═══")
	fmt.Println()

	if timedOut {
		fmt.Printf("⏰ TIMEOUT after %.1fs\n", durationSec)
	} else if !benchmarkPassed {
		fmt.Printf("💥 FAILED (exit code %d) in %.1fs\n", exitCode, durationSec)
	} else {
		fmt.Printf("✅ PASSED in %.1fs\n", durationSec)
	}

	if checksPass == "true" {
		fmt.Printf("✅ Checks passed in %.1fs\n", checksDuration)
	} else if checksPass == "false" && checksTimedOut {
		fmt.Printf("⏰ CHECKS TIMEOUT after %.1fs\n", checksDuration)
		fmt.Println("Log this as 'checks_failed'")
	} else if checksPass == "false" {
		fmt.Printf("💥 CHECKS FAILED in %.1fs\n", checksDuration)
		fmt.Println("Log this as 'checks_failed'")
	}

	if len(parsedMetrics) > 0 {
		fmt.Println()
		fmt.Print("📐 Parsed metrics:")
		for k, v := range parsedMetrics {
			fmt.Printf(" %s=%g", k, v)
		}
		fmt.Println()
	}

	// Show last 20 lines of output
	outputLines := strings.Split(outputStr, "\n")
	totalLines := len(outputLines)
	tail := outputLines
	if len(tail) > 20 {
		tail = tail[len(tail)-20:]
	}
	fmt.Println()
	fmt.Printf("── Output (last %d lines of %d total) ──\n", len(tail), totalLines)
	fmt.Println(strings.Join(tail, "\n"))

	if checksPass == "false" {
		fmt.Println()
		fmt.Println("── Checks output (last 80 lines) ──")
		fmt.Println(checksOutput)
	}

	// Structured metadata
	metricsJSON := "{}"
	if len(parsedMetrics) > 0 {
		var parts []string
		for k, v := range parsedMetrics {
			parts = append(parts, fmt.Sprintf("%q:%g", k, v))
		}
		metricsJSON = "{" + strings.Join(parts, ",") + "}"
	}

	fmt.Println()
	fmt.Println("═══ Run Metadata ═══")
	fmt.Printf("duration_seconds=%.2f\n", durationSec)
	fmt.Printf("exit_code=%d\n", exitCode)
	fmt.Printf("timed_out=%v\n", timedOut)
	fmt.Printf("benchmark_passed=%v\n", benchmarkPassed)
	fmt.Printf("checks_pass=%s\n", checksPass)
	fmt.Printf("checks_timed_out=%v\n", checksTimedOut)
	fmt.Printf("checks_duration=%.2f\n", checksDuration)
	fmt.Printf("metrics_json=%s\n", metricsJSON)
	fmt.Printf("total_output_lines=%d\n", totalLines)

	return nil
}
