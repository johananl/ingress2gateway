# Refactor Plan: Replace Notifications with slog

## Problem Statement

The codebase has a custom "notification" system that is functionally equivalent to structured
logging but implemented as global mutable state. This creates:

- A global `NotificationAggr` variable with mutex-protected state
- Duplicate logging: many call sites do both `notify()` and `klog.Something()` for the same event
- A separate code path just to format output as ASCII tables
- Multiple logging libraries in use (`klog`, `log`, and the notification system)

## Proposed Solution

Treat notifications as what they are: logs. Use Go's standard `log/slog` package as the single
logging mechanism throughout the codebase.

## Key Design Decisions

### 1. Slog as the Single Logging Interface

All logging—whether currently done via `klog`, `log.Printf`, or `notify()`—goes through `slog`.
This gives us structured logging with consistent semantics.

### 2. Two Output Formats

- **Pretty text with colors** (default): Human-readable, suitable for terminal use
- **JSON**: Machine-parseable, suitable for log aggregation pipelines

Users select via an `--log-format` flag.

### 3. No Aggregation, No Tables

Logs are emitted as they happen. No collection into a global structure, no post-hoc table
rendering. Each log line stands alone with its full context (provider, object reference, etc.).

### 4. Structured Fields for Context

Instead of the `CallingObjects` slice in notifications, we use slog attributes:

- `provider`: which provider emitted the log
- `object`: the Kubernetes object reference (e.g., `Gateway:default/my-gw`)

### 5. CLI Controls Log Level

Add `--log-level` flag to control verbosity. For example, current `klog.Infof` calls for "ignoring
field X" become `slog.Info` and can be silenced with `--log-level=warn`.

## What Changes

### Removed

- The entire `pkg/i2gw/notifications/` package
- Per-provider `notification.go` wrapper files
- The `tablewriter` dependency
- The notification map return value from `ToGatewayAPIResources()`

### Added

- A small `pkg/i2gw/logging/` package with:
  - Setup function to configure slog
  - A custom handler for colored terminal output
  - Helper functions for common attributes (provider, object keys)
- CLI flags: `--log-format`, `--log-level`

### Modified

- All `notify()` calls become `slog.Info/Warn/Error` calls
- All `klog` calls become `slog` calls
- `ToGatewayAPIResources()` returns `([]GatewayResources, error)` instead of `([]GatewayResources, map[string]string, error)`

## Migration Approach

The refactor can be done incrementally:

1. **Add the new logging package** alongside existing code
2. **Migrate providers one at a time** (each is independent)
3. **Remove the old notification infrastructure** once all providers are migrated
4. **Clean up** unused dependencies

## Backwards Compatibility

This is a breaking change for:

- Anyone calling `ToGatewayAPIResources()` directly (signature changes)
- Anyone parsing the ASCII table output (no longer exists)

The JSON log format provides a better machine-parseable alternative to the tables.
