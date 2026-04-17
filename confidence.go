package main

import (
	"math"
	"sort"
)

// ComputeConfidence computes the MAD-based confidence score.
// Returns nil when there's insufficient data or no improvement.
//
// Algorithm:
//  1. Collect all metric values in the current segment (> 0).
//  2. Add the incoming result.
//  3. If fewer than 3 data points, return nil.
//  4. Compute median of all values, then MAD (median of absolute deviations).
//  5. Find the best kept metric vs baseline.
//  6. Return |delta| / MAD.
func ComputeConfidence(results []ExperimentResult, segment int, direction string, curMetric float64, curStatus string) *float64 {
	// Collect current segment results + the new one
	var values []float64
	type entry struct {
		metric float64
		status string
	}
	var entries []entry

	for _, r := range results {
		if r.Segment == segment && r.Metric > 0 {
			values = append(values, r.Metric)
			entries = append(entries, entry{r.Metric, r.Status})
		}
	}
	if curMetric > 0 {
		values = append(values, curMetric)
		entries = append(entries, entry{curMetric, curStatus})
	}

	if len(values) < 3 {
		return nil
	}

	median := sortedMedian(values)

	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}
	mad := sortedMedian(deviations)

	if mad == 0 {
		return nil
	}

	// Baseline = first entry in the segment
	baseline := entries[0].metric

	// Find best kept metric
	var bestKept *float64
	for _, e := range entries {
		if e.status == "keep" && e.metric > 0 {
			if bestKept == nil || isBetter(e.metric, *bestKept, direction) {
				v := e.metric
				bestKept = &v
			}
		}
	}

	if bestKept == nil || *bestKept == baseline {
		return nil
	}

	delta := math.Abs(*bestKept - baseline)
	conf := delta / mad
	// Round to 1 decimal
	conf = math.Round(conf*10) / 10
	return &conf
}

func isBetter(a, b float64, direction string) bool {
	if direction == "higher" {
		return a > b
	}
	return a < b
}

func sortedMedian(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}
