/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
)

// PrettyHandler is a slog.Handler that outputs human-readable colored text.
type PrettyHandler struct {
	opts   *slog.HandlerOptions
	output io.Writer
	mu     *sync.Mutex
	attrs  []slog.Attr
	groups []string
}

// NewPrettyHandler creates a new PrettyHandler that writes to the given output.
func NewPrettyHandler(output io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &PrettyHandler{
		opts:   opts,
		output: output,
		mu:     &sync.Mutex{},
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle handles the Record by formatting it as colored text.
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Format level with color
	levelStr, levelColor := formatLevel(r.Level)

	// Build the log line
	line := fmt.Sprintf("%s%s%s %s", levelColor, levelStr, colorReset, r.Message)

	// Collect all attributes (from handler and record)
	allAttrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	allAttrs = append(allAttrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		allAttrs = append(allAttrs, a)
		return true
	})

	// Format attributes
	if len(allAttrs) > 0 {
		line += colorGray
		for _, attr := range allAttrs {
			if attr.Value.String() == "" {
				continue
			}
			line += fmt.Sprintf(" %s=%s", attr.Key, formatAttrValue(attr.Value))
		}
		line += colorReset
	}

	line += "\n"

	_, err := h.output.Write([]byte(line))
	return err
}

// WithAttrs returns a new handler with the given attributes.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &PrettyHandler{
		opts:   h.opts,
		output: h.output,
		mu:     h.mu,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new handler with the given group.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &PrettyHandler{
		opts:   h.opts,
		output: h.output,
		mu:     h.mu,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// formatLevel returns the level string and its color.
func formatLevel(level slog.Level) (string, string) {
	switch {
	case level >= slog.LevelError:
		return "ERROR", colorRed
	case level >= slog.LevelWarn:
		return "WARN ", colorYellow
	case level >= slog.LevelInfo:
		return "INFO ", colorCyan
	default:
		return "DEBUG", colorBlue
	}
}

// formatAttrValue formats an attribute value for display.
func formatAttrValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		// Quote strings with spaces
		if containsSpace(s) {
			return fmt.Sprintf("%q", s)
		}
		return s
	default:
		return v.String()
	}
}

// containsSpace checks if a string contains any whitespace.
func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			return true
		}
	}
	return false
}
