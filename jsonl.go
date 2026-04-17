package main

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

const jsonlFile = "autoresearch.jsonl"

// ConfigEntry is the header line written by `init`.
type ConfigEntry struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	MetricName    string `json:"metricName"`
	MetricUnit    string `json:"metricUnit"`
	BestDirection string `json:"bestDirection"`
}

// ExperimentResult is one run line in the JSONL.
type ExperimentResult struct {
	Run             int                    `json:"run"`
	Commit          string                 `json:"commit"`
	Metric          float64                `json:"metric"`
	Metrics         map[string]float64     `json:"metrics"`
	Status          string                 `json:"status"`
	Description     string                 `json:"description"`
	Timestamp       int64                  `json:"timestamp"`
	Segment         int                    `json:"segment"`
	Confidence      *float64               `json:"confidence"`
	IterationTokens *int                   `json:"iterationTokens"`
	ASI             map[string]any `json:"asi,omitempty"`
}

// jsonlLine is a union type for parsing: we check "type" or "run" fields.
type jsonlLine struct {
	// config fields
	Type          string `json:"type"`
	Name          string `json:"name"`
	MetricName    string `json:"metricName"`
	MetricUnit    string `json:"metricUnit"`
	BestDirection string `json:"bestDirection"`

	// result fields
	Run             *int               `json:"run"`
	Commit          string             `json:"commit"`
	Metric          float64            `json:"metric"`
	Metrics         map[string]float64 `json:"metrics"`
	Status          string             `json:"status"`
	Description     string             `json:"description"`
	Timestamp       int64              `json:"timestamp"`
	Segment         int                `json:"segment"`
	Confidence      *float64           `json:"confidence"`
	IterationTokens *int               `json:"iterationTokens"`
	ASI             map[string]any     `json:"asi,omitempty"`
}

// SessionState holds reconstructed state from the JSONL file.
type SessionState struct {
	Results   []ExperimentResult
	Direction string // "lower" or "higher"
	Segment   int
	RunCount  int
}

// ReadSession parses autoresearch.jsonl and returns the session state.
func ReadSession() (*SessionState, error) {
	f, err := os.Open(jsonlFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &SessionState{Direction: "lower"}, nil
		}
		return nil, err
	}
	defer f.Close()

	state := &SessionState{Direction: "lower"}
	segment := 0
	hasRuns := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry jsonlLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed
		}

		if entry.Type == "config" {
			if hasRuns {
				segment++
			}
			if entry.BestDirection != "" {
				state.Direction = entry.BestDirection
			}
			state.Segment = segment
			continue
		}

		if entry.Run != nil {
			hasRuns = true
			metrics := entry.Metrics
			if metrics == nil {
				metrics = make(map[string]float64)
			}
			state.Results = append(state.Results, ExperimentResult{
				Run:             *entry.Run,
				Commit:          entry.Commit,
				Metric:          entry.Metric,
				Metrics:         metrics,
				Status:          entry.Status,
				Description:     entry.Description,
				Timestamp:       entry.Timestamp,
				Segment:         segment,
				Confidence:      entry.Confidence,
				IterationTokens: entry.IterationTokens,
				ASI:             entry.ASI,
			})
			state.RunCount++
		}
	}
	state.Segment = segment
	return state, scanner.Err()
}

// AppendJSONL appends a single JSON object as a line to autoresearch.jsonl.
func AppendJSONL(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(jsonlFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// NowMillis returns the current time in milliseconds.
func NowMillis() int64 {
	return time.Now().UnixMilli()
}
