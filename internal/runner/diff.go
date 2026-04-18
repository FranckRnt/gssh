package runner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"

	internalssh "gssh/internal/ssh"
)

// outputGroup represents a set of servers that produced identical output for a command.
type outputGroup struct {
	Output  string
	Hosts   []string
	Success bool
	Exit    int
}

// DiffReport holds the diff analysis for one command.
type DiffReport struct {
	Command string
	Groups  []outputGroup
}

// ComputeDiff groups results by command, then by identical output.
// Returns one DiffReport per unique command.
func ComputeDiff(results []internalssh.Result) []DiffReport {
	// Group results by command (preserving command order).
	type cmdResults struct {
		command string
		results []internalssh.Result
	}
	var ordered []cmdResults
	seen := make(map[string]int) // command -> index in ordered
	for _, r := range results {
		idx, ok := seen[r.Command]
		if !ok {
			idx = len(ordered)
			seen[r.Command] = idx
			ordered = append(ordered, cmdResults{command: r.Command})
		}
		ordered[idx].results = append(ordered[idx].results, r)
	}

	var reports []DiffReport
	for _, cr := range ordered {
		report := DiffReport{Command: cr.command}

		// Group by output hash.
		type groupKey struct {
			hash    string
			success bool
			exit    int
		}
		groupMap := make(map[groupKey]*outputGroup)
		var keys []groupKey

		for _, r := range cr.results {
			output := strings.TrimRight(r.Output, "\n")
			if r.Error != "" {
				output = "ERROR: " + r.Error
			}
			h := sha256.Sum256([]byte(output))
			key := groupKey{
				hash:    fmt.Sprintf("%x", h),
				success: r.IsSuccess(),
				exit:    r.ReturnCode,
			}
			g, ok := groupMap[key]
			if !ok {
				g = &outputGroup{
					Output:  output,
					Success: r.IsSuccess(),
					Exit:    r.ReturnCode,
				}
				groupMap[key] = g
				keys = append(keys, key)
			}
			g.Hosts = append(g.Hosts, r.Hostname)
		}

		// Sort groups: largest group first (the "majority").
		sort.Slice(keys, func(i, j int) bool {
			return len(groupMap[keys[i]].Hosts) > len(groupMap[keys[j]].Hosts)
		})

		for _, k := range keys {
			g := groupMap[k]
			sort.Strings(g.Hosts)
			report.Groups = append(report.Groups, *g)
		}

		reports = append(reports, report)
	}

	return reports
}

// PrintDiff prints the diff report.
func (f *Formatter) PrintDiff(reports []DiffReport) {
	for i, report := range reports {
		if i > 0 {
			fmt.Fprintln(f.w)
		}

		fmt.Fprintf(f.w, "%s %s\n", f.bold("── Diff:"), f.cyan(report.Command))

		if len(report.Groups) == 1 {
			g := report.Groups[0]
			fmt.Fprintf(f.w, "  %s All %d server(s) returned identical output\n",
				f.green("✓"), len(g.Hosts))
			printDiffOutput(f.w, g.Output, "    ")
			continue
		}

		fmt.Fprintf(f.w, "  %s %d different outputs across %d server(s)\n",
			f.yellow("⚠"), len(report.Groups), countHosts(report.Groups))

		for gi, g := range report.Groups {
			label := "GROUP"
			if gi == 0 {
				label = "MAJORITY"
			}

			status := f.green("OK")
			if !g.Success {
				status = f.red(fmt.Sprintf("FAIL (exit=%d)", g.Exit))
			}

			fmt.Fprintf(f.w, "\n  %s %s (%d server(s)) %s\n",
				f.bold(label), status, len(g.Hosts), formatHostList(g.Hosts))
			printDiffOutput(f.w, g.Output, "    ")
		}
	}
}

func countHosts(groups []outputGroup) int {
	n := 0
	for _, g := range groups {
		n += len(g.Hosts)
	}
	return n
}

func formatHostList(hosts []string) string {
	if len(hosts) <= 5 {
		return "[" + strings.Join(hosts, ", ") + "]"
	}
	return fmt.Sprintf("[%s, ... +%d more]",
		strings.Join(hosts[:5], ", "), len(hosts)-5)
}

// PrintGrouped prints results grouped by identical output.
// Unlike PrintDiff which is analytical, this replaces the normal per-server output.
func (f *Formatter) PrintGrouped(reports []DiffReport) {
	for i, report := range reports {
		if i > 0 {
			fmt.Fprintln(f.w)
		}

		if len(reports) > 1 {
			fmt.Fprintf(f.w, "%s %s\n", f.bold("── Command:"), f.cyan(report.Command))
		}

		for _, g := range report.Groups {
			status := f.green("OK")
			if !g.Success {
				status = f.red(fmt.Sprintf("FAIL (exit=%d)", g.Exit))
			}

			fmt.Fprintf(f.w, "\n%s %s  %d server(s): %s\n",
				f.bold("▸"), status, len(g.Hosts), formatHostList(g.Hosts))
			printDiffOutput(f.w, g.Output, "  ")
		}
	}
}

func printDiffOutput(w io.Writer, output string, indent string) {
	if output == "" {
		fmt.Fprintf(w, "%s%s\n", indent, "(empty output)")
		return
	}
	lines := strings.Split(output, "\n")
	maxLines := 20
	for i, line := range lines {
		if i >= maxLines {
			fmt.Fprintf(w, "%s... (%d more lines)\n", indent, len(lines)-maxLines)
			break
		}
		fmt.Fprintf(w, "%s%s\n", indent, line)
	}
}
