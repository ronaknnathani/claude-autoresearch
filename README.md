# claude-autoresearch

Autonomous experiment loop for **Claude Code** — try an idea, measure it, keep what works, discard what doesn't, repeat forever.

A port of [pi-autoresearch](https://github.com/davebcn87/pi-autoresearch) for Claude Code. Inspired by [karpathy/autoresearch](https://github.com/karpathy/autoresearch).

You tell Claude what to optimize and how to measure it. Claude does the rest — modifying code, running benchmarks, keeping improvements, reverting regressions, and repeating autonomously until you stop it.

---

## Install

```bash
# Clone
git clone https://github.com/youruser/claude-autoresearch.git
cd claude-autoresearch

# Build the CLI
go build -o bin/autoresearch .

# Add skills to Claude Code (symlink into your project's .claude/ or global skills)
```

## Usage

Start an autoresearch session from Claude Code:

```
/autoresearch optimize pod scheduling latency, keep all existing tests passing
```

That's it. Claude will:

1. Ask clarifying questions (or infer from context) about the goal, benchmark command, metric, and files in scope.
2. Create a feature branch, write `autoresearch.md` (session doc) and `autoresearch.sh` (benchmark script).
3. Run the baseline, then loop forever: modify code → benchmark → keep or revert → repeat.

You can walk away. Come back hours later to find dozens of experiments logged, with the best changes committed.

### Resume a session

If Claude hits a context limit or you restart, it picks up where it left off:

```
/autoresearch continue optimizing, check the ideas backlog
```

Claude reads `autoresearch.md` and `autoresearch.jsonl` to reconstruct full context.

### View progress

```
/autoresearch-export
```

Opens a live dashboard in your browser with a chart, metrics table, and confidence scores. Updates in real time as experiments run.

### Finalize into PRs

When you're happy with the results:

```
/autoresearch-finalize
```

Claude groups the kept experiments into clean, independent branches — each from the merge-base, each reviewable and mergeable on its own.

---

## How it works

```
┌──────────────────────────┐     ┌──────────────────────────────┐
│  CLI (infrastructure)     │     │  Skill (domain knowledge)     │
│                           │     │                               │
│  autoresearch init        │◄────│  goal: reduce scheduling time │
│  autoresearch run         │     │  metric: p99_latency (lower)  │
│  autoresearch log         │     │  scope: scheduler/*.go        │
│  autoresearch serve       │     │  constraint: tests must pass  │
└──────────────────────────┘     └──────────────────────────────┘
```

The **skill** tells Claude how to behave (what to optimize, when to keep/discard, never stop). The **CLI** handles infrastructure (timing, metric parsing, git operations, confidence scoring). This separation means one CLI serves any optimization domain.

Two files survive across context resets and session restarts:

- **`autoresearch.jsonl`** — append-only log of every experiment (metric, status, commit, description, ASI)
- **`autoresearch.md`** — living document with the objective, what's been tried, dead ends, and key wins

A fresh Claude session with no memory can read these two files and continue exactly where the last session left off.

---

## CLI reference

### `autoresearch init`

```
autoresearch init <name> <metric_name> [unit] [direction]
```

Initialize an experiment session. Writes a config header to `autoresearch.jsonl`.

- `direction` is `lower` (default) or `higher`
- Call again to start a new segment (resets baseline, keeps history)

### `autoresearch run`

```
autoresearch run <command> [timeout_seconds] [checks_timeout_seconds]
```

Run a benchmark. Times wall-clock duration, captures stdout/stderr, parses `METRIC name=value` lines from output. If `autoresearch.checks.sh` exists and the benchmark passes, runs correctness checks automatically.

Defaults: 600s timeout, 300s checks timeout.

### `autoresearch log`

```
autoresearch log <status> <metric> <description> [commit] [metrics_json] [asi_json]
```

Record an experiment result.

- `status`: `keep` | `discard` | `crash` | `checks_failed`
- `keep` auto-commits all changes. Everything else auto-reverts (preserving session files).
- Computes MAD-based confidence score after 3+ runs.
- `metrics_json`: secondary metrics as JSON, e.g. `'{"heap_mb":128}'`
- `asi_json`: actionable side information, e.g. `'{"hypothesis":"reduced allocs","rollback_reason":"broke integration test"}'`

### `autoresearch serve`

```
autoresearch serve [--port 8787]
```

Launch a live dashboard in the browser. Embeds the HTML — no external files needed. Auto-updates via Server-Sent Events as experiments complete.

---

## Session files

| File | Purpose |
|------|---------|
| `autoresearch.md` | Living doc — objective, metrics, files in scope, what's been tried. The heart of the session. |
| `autoresearch.sh` | Benchmark script — outputs `METRIC name=value` lines. Keep it fast. |
| `autoresearch.checks.sh` | *(optional)* Correctness checks (tests, types, lint). Failures block `keep`. |
| `autoresearch.jsonl` | Append-only experiment log. Survives everything. |
| `autoresearch.ideas.md` | *(optional)* Backlog of promising but deferred ideas. |
| `autoresearch.config.json` | *(optional)* `{"maxIterations": 50}` to cap runs. |

---

## Confidence scoring

After 3+ experiments, each `autoresearch log` reports a confidence score — how the best improvement compares to the session's measurement noise, using [Median Absolute Deviation](https://en.wikipedia.org/wiki/Median_absolute_deviation).

| Score | Meaning |
|-------|---------|
| ≥ 2.0× | Improvement likely real — signal well above noise |
| 1.0–2.0× | Above noise but marginal — consider re-running |
| < 1.0× | Within noise — probably benchmark jitter, not a real gain |

The score is advisory. It never auto-discards. Claude is instructed to use it as a signal for when to re-run experiments to confirm.

---

## Practical use cases

### Kubernetes scheduler performance

You're tuning a custom scheduler plugin and want to reduce pod scheduling latency.

```
/autoresearch optimize p99 scheduling latency for batch workloads
```

**Benchmark script** (`autoresearch.sh`):
```bash
#!/bin/bash
set -euo pipefail
# Run scheduler simulator with 1000 pods, capture p99 and throughput
go test -bench=BenchmarkSchedule -benchtime=10s -count=3 ./pkg/scheduler/... 2>&1 | tee /dev/stderr | \
  awk '/BenchmarkSchedule/ { sum+=$3; n++ } END { printf "METRIC p99_ns=%d\n", sum/n }'
go test -bench=BenchmarkThroughput -benchtime=5s ./pkg/scheduler/... 2>&1 | \
  awk '/BenchmarkThroughput/ { print "METRIC pods_per_sec="$3 }'
```

Claude will iterate on the scoring plugin, queue sorting, preemption logic — keeping changes that reduce p99 without tanking throughput.

### Container image build time

Your team's base images take 8 minutes to build. You want that under 3.

```
/autoresearch optimize container build time for the platform base image
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
START=$(date +%s%N)
docker build --no-cache -t base-image:test -f Dockerfile.base . 2>&1 | tail -5
END=$(date +%s%N)
DURATION_MS=$(( (END - START) / 1000000 ))
docker image inspect base-image:test --format='{{.Size}}' | awk '{printf "METRIC image_bytes=%d\n", $1}'
echo "METRIC build_ms=$DURATION_MS"
```

Claude will try multi-stage build reordering, layer caching strategies, dependency pruning, and switching base images — keeping only changes that actually cut build time.

### Helm chart render speed

Complex Helm charts are slow to render locally, blocking the dev loop.

```
/autoresearch optimize helm template render time for the platform charts
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
# Render 5 times, report median
TIMES=()
for i in $(seq 1 5); do
  START=$(date +%s%N)
  helm template my-release ./charts/platform -f values.yaml > /dev/null 2>&1
  END=$(date +%s%N)
  TIMES+=( $(( (END - START) / 1000000 )) )
done
# Sort and take median
IFS=$'\n' SORTED=($(sort -n <<< "${TIMES[*]}")); unset IFS
echo "METRIC render_ms=${SORTED[2]}"
```

### Go test suite speed

Your infra repo's tests take 4 minutes. Most of that is integration tests starting containers.

```
/autoresearch optimize test suite runtime, tests must still pass
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
START=$(date +%s%N)
go test -count=1 ./... 2>&1 | tail -20
EXIT=${PIPESTATUS[0]}
END=$(date +%s%N)
echo "METRIC test_seconds=$(echo "scale=2; ($END - $START) / 1000000000" | bc)"
exit $EXIT
```

**Checks script** (`autoresearch.checks.sh`):
```bash
#!/bin/bash
set -euo pipefail
go vet ./...
```

Claude will try test parallelization, shared fixtures, removing redundant setup, reordering expensive operations — and the checks script ensures `go vet` still passes.

### Kubernetes API server memory usage

You've got a custom aggregated API server that's using too much memory under load.

```
/autoresearch reduce peak heap usage of the aggregated API server under load test, keep all API responses correct
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
# Build and start server, run load test, capture peak RSS
go build -o /tmp/apiserver ./cmd/apiserver
/tmp/apiserver &
PID=$!
sleep 2
# Run load test
hey -n 10000 -c 50 http://localhost:8080/apis/custom.io/v1/workloads > /tmp/hey-output.txt 2>&1
PEAK_RSS=$(ps -o rss= -p $PID | tr -d ' ')
kill $PID 2>/dev/null; wait $PID 2>/dev/null || true
LATENCY_P99=$(grep '99%' /tmp/hey-output.txt | awk '{print $2}')
echo "METRIC peak_rss_kb=$PEAK_RSS"
echo "METRIC p99_latency_ms=$LATENCY_P99"
```

### Flink job checkpoint duration

Your Kafka→Flink streaming pipeline's checkpoints are too slow, causing backpressure.

```
/autoresearch reduce flink checkpoint duration while maintaining exactly-once semantics
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
# Deploy to local Flink cluster, run for 60s, capture checkpoint metrics
./bin/flink run -d ./target/streaming-job.jar
sleep 60
METRICS=$(curl -s http://localhost:8081/jobs/${JOB_ID}/checkpoints | jq '.latest.completed')
DURATION=$(echo "$METRICS" | jq '.duration')
SIZE=$(echo "$METRICS" | jq '.state_size')
echo "METRIC checkpoint_ms=$DURATION"
echo "METRIC state_bytes=$SIZE"
./bin/flink cancel $JOB_ID
```

### gRPC service startup time

Your platform service takes 30 seconds to start because of heavy initialization.

```
/autoresearch optimize service startup time, service must pass health checks
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
go build -o /tmp/svc ./cmd/server
START=$(date +%s%N)
/tmp/svc &
PID=$!
# Wait for health check
for i in $(seq 1 60); do
  if grpcurl -plaintext localhost:9090 grpc.health.v1.Health/Check > /dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
END=$(date +%s%N)
kill $PID 2>/dev/null; wait $PID 2>/dev/null || true
echo "METRIC startup_ms=$(( (END - START) / 1000000 ))"
```

Claude will try lazy initialization, parallel init, removing unnecessary pre-warming, deferring non-critical setup — keeping only changes that actually reduce time-to-healthy.

### Terraform plan speed

Large infra repos with hundreds of resources take forever to plan.

```
/autoresearch optimize terraform plan time for the k8s-clusters module
```

**Benchmark script**:
```bash
#!/bin/bash
set -euo pipefail
cd modules/k8s-clusters
START=$(date +%s%N)
terraform plan -no-color -input=false 2>&1 | tail -10
END=$(date +%s%N)
RESOURCES=$(terraform plan -no-color -input=false 2>&1 | grep -c 'will be')
echo "METRIC plan_seconds=$(echo "scale=2; ($END - $START) / 1000000000" | bc)"
echo "METRIC resource_count=$RESOURCES"
```

---

## Architecture

```
claude-autoresearch/
├── main.go              # Subcommand dispatch
├── init.go              # autoresearch init
├── run.go               # autoresearch run (timing, metric parsing, checks)
├── log.go               # autoresearch log (git ops, confidence, JSONL)
├── serve.go             # autoresearch serve (HTTP + SSE, embedded HTML)
├── jsonl.go             # Shared types and JSONL read/write
├── confidence.go        # MAD-based confidence scoring
├── assets/
│   └── template.html    # Dashboard (embedded via go:embed)
├── skills/
│   ├── autoresearch/         # /autoresearch — setup + autonomous loop
│   ├── autoresearch-finalize/ # /autoresearch-finalize — clean branches
│   └── autoresearch-export/   # /autoresearch-export — live dashboard
├── tests/
│   └── finalize_test.sh  # 18 tests for branch finalization
├── go.mod
└── package.json
```

## Prerequisites

- **Claude Code** — CLI, desktop app, or IDE extension
- **Go 1.21+** — to build the binary
- **Git** — for auto-commit/revert and branch management

## License

MIT
