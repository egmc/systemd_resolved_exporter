# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

systemd_resolved_exporter is a Prometheus exporter for systemd-resolved metrics. It collects DNS resolver statistics and exposes them for monitoring.

## Essential Commands

### Build
```bash
go build
# For release builds with goreleaser:
goreleaser release --snapshot --rm-dist
```

### Run
```bash
# Basic execution
./systemd_resolved_exporter

# With options
./systemd_resolved_exporter --listen-address=":9924" --gather-dnssec --collect-mode="dbus" --debug
```

### Dependencies
```bash
go mod tidy
```

## Architecture

The exporter implements two collection modes:

1. **DBus mode (default)**: Communicates directly with systemd-resolved via D-Bus interface `org.freedesktop.resolve1`
2. **CLI mode**: Parses output from `systemd-resolve --statistics` command

Key implementation details:
- Main logic in `systemd_resolved_exporter.go` implements Prometheus `Collector` interface
- Metrics exposed at `/metrics` endpoint on port 9924 (configurable)
- Supports both Gauge and Counter metric types
- Uses zap for structured logging

## Metrics Collected

- Cache statistics: hits, misses, current size
- Transaction statistics: current active, total transactions  
- DNSSEC statistics (optional): secure, insecure, bogus, indeterminate verdicts

## Important Notes

- No test files currently exist in the project
- Packaged as systemd service running as nobody:nogroup
- Requires Go 1.17+ and Linux with systemd-resolved
- Uses goreleaser for release management