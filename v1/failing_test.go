package json

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
)

var skipKnownFailures = flag.Bool("skip-known-failures", true, "skip tests that are known to already be failing")
var updateKnownFailures = flag.Bool("update-known-failures", false, "update the list of known failures")

//go:embed failing.txt
var knownFailuresText string
var knownFailures = sync.OnceValue(func() map[string]bool {
	failures := make(map[string]bool)
	for _, s := range strings.Split(knownFailuresText, "\n") {
		failures[strings.TrimRight(s, "\r")] = true
	}
	return failures
})

// skipKnownFailure skips the current test if it is in the failing.txt list.
func skipKnownFailure(t *testing.T) {
	if *skipKnownFailures && knownFailures()[t.Name()] {
		t.SkipNow()
	}
}

// TestKnownFailures tests whether the failing.old is up-to-date.
func TestKnownFailures(t *testing.T) {
	if !*skipKnownFailures {
		return // avoid infinite recursion calling the same test
	}

	// Produce a sorted list of currently known failures.
	b, _ := exec.Command("go", "test", "-skip-known-failures=false", ".").CombinedOutput()
	var newFailing []string
	for _, line := range strings.Split(string(b), "\n") {
		if _, suffix, ok := strings.Cut(strings.TrimRight(line, "\r"), "--- FAIL: "); ok {
			suffix = strings.TrimSuffix(suffix, ")")
			suffix = strings.TrimRight(suffix, ".0123456789s")
			suffix = strings.TrimSuffix(suffix, " (")
			newFailing = append(newFailing, suffix)
		}
	}
	newFailingSorted := slices.Clone(newFailing)
	slices.Sort(newFailingSorted)

	// Produce a sorted list of previously known failures.
	oldFailing := strings.Split(strings.TrimSuffix(knownFailuresText, "\n"), "\n")
	for i, s := range oldFailing {
		oldFailing[i] = strings.TrimRight(s, "\r")
	}
	oldFailingSorted := slices.Clone(oldFailing)
	slices.Sort(oldFailingSorted)

	// Check whether the two lists match.
	if !slices.Equal(newFailingSorted, oldFailingSorted) {
		var diff []string
		before, after := oldFailingSorted, newFailingSorted
		for len(before)|len(after) > 0 {
			switch {
			case len(before) == 0:
				diff = append(diff, fmt.Sprintf("+ %s\n", after[0]))
				after = after[1:]
			case len(after) == 0:
				diff = append(diff, fmt.Sprintf("- %s\n", before[0]))
				before = before[1:]
			case after[0] < before[0]:
				diff = append(diff, fmt.Sprintf("+ %s\n", after[0]))
				after = after[1:]
			case before[0] < after[0]:
				diff = append(diff, fmt.Sprintf("- %s\n", before[0]))
				before = before[1:]
			default:
				before, after = before[1:], after[1:]
			}
		}
		t.Errorf("known failures mismatch (-old +new):\n%s", strings.Join(diff, ""))
		if *updateKnownFailures {
			if err := os.WriteFile("failing.txt", []byte(strings.Join(newFailing, "\n")+"\n"), 0664); err != nil {
				t.Errorf("os.WriteFile error: %v", err)
			}
		}
	}
}
