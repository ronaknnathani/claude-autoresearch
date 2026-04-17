package main

import "fmt"

func cmdInit(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: autoresearch init <name> <metric_name> [metric_unit] [direction]")
	}

	name := args[0]
	metricName := args[1]
	metricUnit := ""
	direction := "lower"

	if len(args) >= 3 {
		metricUnit = args[2]
	}
	if len(args) >= 4 {
		direction = args[3]
	}

	if direction != "lower" && direction != "higher" {
		return fmt.Errorf("direction must be 'lower' or 'higher', got %q", direction)
	}

	entry := ConfigEntry{
		Type:          "config",
		Name:          name,
		MetricName:    metricName,
		MetricUnit:    metricUnit,
		BestDirection: direction,
	}

	if err := AppendJSONL(entry); err != nil {
		return fmt.Errorf("failed to write %s: %w", jsonlFile, err)
	}

	fmt.Printf("✅ Experiment initialized: %q\n", name)
	fmt.Printf("Metric: %s (%s, %s is better)\n", metricName, orDefault(metricUnit, "unitless"), direction)
	fmt.Printf("Config written to %s.\n", jsonlFile)
	return nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
