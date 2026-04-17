---
name: autoresearch-export
description: Open a live dashboard for autoresearch experiments in the browser. Use when asked to "export autoresearch", "show dashboard", or "view experiment results".
---

# Autoresearch Export

Launch a live browser dashboard showing experiment progress, metrics chart, and results table.

## How to Use

Run the dashboard server:

```bash
<SKILL_DIR>/../../bin/autoresearch serve
```

This starts an HTTP server on port 8787 (or next available) that:
- Serves the interactive dashboard HTML
- Streams `autoresearch.jsonl` data
- Auto-updates via Server-Sent Events when new experiments are logged

The server runs in the foreground. Open the URL printed in the output.

To stop the server, press Ctrl+C or kill the process.

## Dashboard Features

- **Statistics cards**: Baseline → Best, improvement %, run count, kept runs
- **Interactive chart**: Smooth curve through metric values with hover tooltips
- **Data table**: All runs with status badges, metrics, deltas, confidence values
- **Export**: Generate a shareable PNG card of results
- **Live updates**: Dashboard refreshes automatically as new experiments complete
