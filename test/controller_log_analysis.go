package test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Keep memory per scanned line bounded while allowing large structured log entries.
const maxControllerLogLineBytes = 1024 * 1024

var (
	// Only the standard optimistic-concurrency retry is ignored by default.
	defaultKnownControllerErrors = []string{`Operation cannot be fulfilled on [^\r\n]+: the object has been modified; please apply your changes to the latest version and try again`}
	controllerLogTimestamp       = regexp.MustCompile(`^\d{8}_\d{6}\.log$`)
	controllerKlogPrefix         = regexp.MustCompile(`^[IWEF]\d{4}\s+\d{2}:\d{2}:\d{2}\.\d+\s+\d+\s+[^\]]+\]\s*`)
	controllerTextPrefix         = regexp.MustCompile(`^(?:\d{4}-\d{2}-\d{2}T\S+\s+)?(?:ERROR|FATAL|PANIC)\s+`)
	controllerErrorSeverity      = regexp.MustCompile(`(?i)(?:^|\s)(?:ERROR|FATAL|PANIC)(?:\s|:|$)`)
)

// ControllerErrorGroup counts one known regex or unexpected message across saved files.
// FirstMatch locates an example without retaining every occurrence in memory.
type ControllerErrorGroup struct {
	Pattern    string
	Known      bool
	Count      int
	FirstMatch string
}

// ControllerLogAnalysis counts log-line occurrences, including repeated snapshots.
// FileCount describes coverage; absent controllers are not assumed to be healthy.
type ControllerLogAnalysis struct {
	FileCount       int
	ErrorCount      int
	UnexpectedCount int
	Groups          []ControllerErrorGroup
}

func (a ControllerLogAnalysis) String() string {
	var out strings.Builder
	fmt.Fprintf(&out, "Controller log analysis: %d files, %d errors, %d unexpected\n", a.FileCount, a.ErrorCount, a.UnexpectedCount)
	for _, group := range a.Groups {
		kind := "unexpected"
		if group.Known {
			kind = "known"
		}
		fmt.Fprintf(&out, "%s count=%d pattern=%q first=%s\n", kind, group.Count, group.Pattern, group.FirstMatch)
	}
	return out.String()
}

func loadKnownControllerErrors(path string) ([]*regexp.Regexp, error) {
	patterns := defaultKnownControllerErrors
	if path != "" {
		data, err := os.ReadFile(path) // #nosec G304 -- explicit user-selected regex configuration
		if err != nil {
			return nil, fmt.Errorf("read known error patterns: %w", err)
		}
		patterns = nil
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				patterns = append(patterns, line)
			}
		}
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for i, pattern := range patterns {
		rule, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("known error pattern %d %q: %w", i+1, pattern, err)
		}
		compiled = append(compiled, rule)
	}
	return compiled, nil
}

// controllerErrorMessage reuses summary classification and also handles klog and
// structured severity. Structured records use msg/message + error/err, excluding
// changing timestamps and controller metadata, to keep groups useful.
func controllerErrorMessage(line string) (string, bool) {
	var record map[string]any
	if json.Unmarshal([]byte(line), &record) == nil && record != nil {
		for _, key := range []string{"level", "severity"} {
			if value, ok := record[key].(string); ok {
				switch strings.ToLower(value) {
				case "error", "fatal", "panic":
					var parts []string
					for _, field := range []string{"msg", "message", "error", "err"} {
						if value, ok := record[field]; ok && value != nil {
							parts = append(parts, fmt.Sprint(value))
						}
					}
					if len(parts) > 0 {
						return strings.Join(parts, ": "), true
					}
					return line, true
				default:
					return "", false // An info message mentioning an error is not an error record.
				}
			}
		}
	}
	klog := controllerKlogPrefix.MatchString(line)
	if klog && line[0] != 'E' && line[0] != 'F' {
		return "", false // Respect klog severity even when an info line mentions errors.
	}
	if !klog && !isControllerErrorLine(line) && !controllerErrorSeverity.MatchString(line) {
		return "", false
	}
	message := controllerKlogPrefix.ReplaceAllString(line, "")
	message = controllerTextPrefix.ReplaceAllString(message, "")
	return strings.TrimSpace(message), true
}

// AnalyzeSavedControllerLogs reads top-level files written by SaveControllerLogs
// for configured controllers. It fails on invalid rules, unavailable input or
// incomplete scans. It never shells out or obtains new logs from a cluster.
func AnalyzeSavedControllerLogs(dir string, controllers []ControllerDef, patternsFile string) (ControllerLogAnalysis, error) {
	var report ControllerLogAnalysis
	rules, err := loadKnownControllerErrors(patternsFile)
	if err != nil {
		return report, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return report, fmt.Errorf("read controller log directory %q: %w", dir, err)
	}
	groups := make(map[string]*ControllerErrorGroup)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		matches := false
		for _, controller := range controllers {
			prefix := strings.ToLower(controller.DisplayName) + "-"
			if strings.HasPrefix(entry.Name(), prefix) && controllerLogTimestamp.MatchString(strings.TrimPrefix(entry.Name(), prefix)) {
				matches = true
				break
			}
		}
		if !matches {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := scanControllerLog(path, rules, groups, &report); err != nil {
			return report, err
		}
		report.FileCount++
	}
	if report.FileCount == 0 {
		return report, fmt.Errorf("no saved controller logs in %q (expected <controller>-YYYYMMDD_HHMMSS.log)", dir)
	}
	for _, group := range groups {
		report.Groups = append(report.Groups, *group)
	}
	sort.Slice(report.Groups, func(i, j int) bool {
		if report.Groups[i].Known != report.Groups[j].Known {
			return !report.Groups[i].Known
		}
		return report.Groups[i].Pattern < report.Groups[j].Pattern
	})
	return report, nil
}

func scanControllerLog(path string, rules []*regexp.Regexp, groups map[string]*ControllerErrorGroup, report *ControllerLogAnalysis) error {
	file, err := os.Open(path) // #nosec G304 -- path from explicit results directory and discovered filename
	if err != nil {
		return fmt.Errorf("open controller log %q: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxControllerLogLineBytes)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		message, isError := controllerErrorMessage(line)
		if !isError {
			continue
		}
		report.ErrorCount++
		known := false
		pattern := message
		for _, rule := range rules {
			if rule.MatchString(line) {
				known = true
				pattern = rule.String()
				break
			}
		}
		if !known {
			report.UnexpectedCount++
		}
		key := fmt.Sprintf("%t:%s", known, pattern)
		if groups[key] == nil {
			groups[key] = &ControllerErrorGroup{Pattern: pattern, Known: known, FirstMatch: fmt.Sprintf("%s:%d", path, lineNumber)}
		}
		groups[key].Count++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan controller log %q at line %d (limit %d bytes per line): %w", path, lineNumber+1, maxControllerLogLineBytes, err)
	}
	return nil
}
