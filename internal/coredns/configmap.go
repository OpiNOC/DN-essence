// Package coredns handles safe patching of the CoreDNS ConfigMap.
// It manages only the section delimited by BEGIN/END markers, leaving
// all other CoreDNS configuration untouched.
package coredns

import (
	"fmt"
	"strings"
)

const (
	beginMarker = "# BEGIN dn-essence"
	endMarker   = "# END dn-essence"
)

// BuildManagedBlock returns the marked block string for the given rewrite lines.
// rewriteLines contains lines like "rewrite name foo.example.com svc.ns.svc.cluster.local".
func BuildManagedBlock(rewriteLines []string) string {
	if len(rewriteLines) == 0 {
		return fmt.Sprintf("    %s\n    %s", beginMarker, endMarker)
	}
	var sb strings.Builder
	sb.WriteString("    " + beginMarker + "\n")
	for _, line := range rewriteLines {
		sb.WriteString("    " + line + "\n")
	}
	sb.WriteString("    " + endMarker)
	return sb.String()
}

// ApplyManagedBlock inserts or replaces the managed block inside the CoreDNS Corefile data.
// It operates on the first server block (.:53 { ... }) found in data.
// If no managed block exists yet, it injects it before the closing brace.
// Returns the updated Corefile string.
func ApplyManagedBlock(data string, rewriteLines []string) (string, error) {
	newBlock := BuildManagedBlock(rewriteLines)

	if hasManagedBlock(data) {
		return replaceManagedBlock(data, newBlock)
	}
	return injectManagedBlock(data, newBlock)
}

// IsUpToDate reports whether the managed block in data already matches rewriteLines.
// Use this to avoid unnecessary ConfigMap patches.
func IsUpToDate(data string, rewriteLines []string) bool {
	if !hasManagedBlock(data) {
		return len(rewriteLines) == 0
	}
	existing := extractManagedBlock(data)
	wanted := BuildManagedBlock(rewriteLines)
	return strings.TrimSpace(existing) == strings.TrimSpace(wanted)
}

// hasManagedBlock returns true when both markers are present.
func hasManagedBlock(data string) bool {
	return strings.Contains(data, beginMarker) && strings.Contains(data, endMarker)
}

// extractManagedBlock returns the full marked section including markers.
func extractManagedBlock(data string) string {
	start := strings.Index(data, beginMarker)
	end := strings.Index(data, endMarker)
	if start == -1 || end == -1 {
		return ""
	}
	return data[start : end+len(endMarker)]
}

// replaceManagedBlock swaps the existing managed block with newBlock.
func replaceManagedBlock(data, newBlock string) (string, error) {
	start := strings.Index(data, beginMarker)
	end := strings.Index(data, endMarker)
	if start == -1 || end == -1 {
		return "", fmt.Errorf("managed block markers not found")
	}
	// Walk back to include leading whitespace on the begin line.
	lineStart := strings.LastIndex(data[:start], "\n") + 1
	return data[:lineStart] + newBlock + data[end+len(endMarker):], nil
}

// injectManagedBlock inserts newBlock before the last closing brace in data.
func injectManagedBlock(data, newBlock string) (string, error) {
	idx := strings.LastIndex(data, "}")
	if idx == -1 {
		return "", fmt.Errorf("no closing brace found in Corefile")
	}
	return data[:idx] + newBlock + "\n" + data[idx:], nil
}
