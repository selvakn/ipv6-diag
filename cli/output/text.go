package output

import (
	"fmt"
	"strings"
)

// PrintHeader prints the run header to stdout.
func PrintHeader(version, serverURL string) {
	fmt.Printf("ipv6diag %s — targeting %s\n\n", version, serverURL)
}

// PrintSeparator prints a visual separator between IPv4 and IPv6 blocks.
func PrintSeparator() {
	fmt.Println()
}

// PrintBlockHeader prints the protocol stack label (── IPv4 ──…).
func PrintBlockHeader(family string) {
	line := fmt.Sprintf("── %s ", family)
	pad := strings.Repeat("─", 60-len(line))
	fmt.Println(line + pad)
}

// PrintResult prints a single test result row.
func PrintResult(testType, status, target, duration string, extras ...string) {
	extra := ""
	if len(extras) > 0 {
		extra = "  " + strings.Join(extras, "  ")
	}
	icon := statusIcon(status)
	fmt.Printf("  %-6s  %s %-10s  %-8s  %s%s\n", testType, icon, status, duration, target, extra)
}

// PrintSummary prints the overall pass/fail count.
func PrintSummary(pass, total int) {
	fmt.Println()
	line := fmt.Sprintf("── Summary %s", strings.Repeat("─", 51))
	fmt.Println(line)
	if pass == total {
		fmt.Printf("  ✓ %d/%d passed\n", pass, total)
	} else {
		fmt.Printf("  ✗ %d/%d passed  (%d failed)\n", pass, total, total-pass)
	}
}

func statusIcon(status string) string {
	switch status {
	case "passed":
		return "✓"
	case "failed":
		return "✗"
	case "timed_out":
		return "⏱"
	case "skipped":
		return "–"
	default:
		return "?"
	}
}
