package test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	controllerPotentialIssue = "Potential issues"
	controllerTransient      = "Transient candidates"
	controllerInformational  = "Informational"
)

var (
	controllerSummaryFields    = regexp.MustCompile(`\b(?:msg|message|error|err)=("(?:\\.|[^"\\])*"|[^\s]+)`)
	controllerSummaryLevel     = regexp.MustCompile(`(?i)\blevel=("?(?:trace|debug|info|warn|warning|error|fatal|panic)"?)(?:\s|$)`)
	controllerSummaryTextLevel = regexp.MustCompile(`(?i)^(?:\d{4}-\d{2}-\d{2}T\S+\s+)?(trace|debug|info|warn|warning|error|fatal|panic)(?:\s|:|$)`)
	controllerSummaryWarning   = regexp.MustCompile(`(?i)(?:^|\s)(?:warn|warning)(?:\s|:|$)`)
	controllerSummaryPrefix    = regexp.MustCompile(`(?i)^(?:\d{4}-\d{2}-\d{2}T\S+\s+)?(?:(?:error|fatal|panic|warn|warning|info)(?:\s*:\s*|\s+))?`)
)

// ControllerLogGroup is one normalized message and its diagnostic annotation.
// Categories describe provisioning heuristics, never known-error exceptions.
type ControllerLogGroup struct {
	Message, Category, Explanation string
	Count                          int
}

// ParseControllerLogSummary counts every fetched log line, keeping bounded samples
// and at most MaxSampleMessages distinct groups per category. Counts describe the
// supplied log window, not the controller's complete lifetime.
func ParseControllerLogSummary(logs string) ControllerLogSummary {
	summary := ControllerLogSummary{CategoryCounts: make(map[string]int)}
	groups := make(map[string]int)
	retained := make(map[string]int)
	for _, line := range strings.Split(logs, "\n") {
		message, level := controllerSummaryMessage(line)
		if level == "" {
			continue
		}
		switch level {
		case "error":
			summary.ErrorCount++
			if len(summary.Errors) < MaxSampleMessages {
				summary.Errors = append(summary.Errors, line)
			}
		case "warn":
			summary.WarnCount++
			if len(summary.Warnings) < MaxSampleMessages {
				summary.Warnings = append(summary.Warnings, line)
			}
		case "info":
			summary.InfoCount++
		}
		category, explanation := classifyControllerSummary(message)
		summary.CategoryCounts[category]++
		key := category + "\x00" + message
		if index, ok := groups[key]; ok {
			summary.Groups[index].Count++
		} else if retained[category] < MaxSampleMessages {
			groups[key] = len(summary.Groups)
			retained[category]++
			summary.Groups = append(summary.Groups, ControllerLogGroup{Message: message, Category: category, Explanation: explanation, Count: 1})
		}
	}
	sort.Slice(summary.Groups, func(i, j int) bool {
		a, b := summary.Groups[i], summary.Groups[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Message < b.Message
	})
	return summary
}

func controllerSummaryMessage(line string) (string, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	var record map[string]any
	structured := json.Unmarshal([]byte(line), &record) == nil && record != nil
	level := ""
	if structured {
		for _, key := range []string{"level", "severity"} {
			if value, ok := record[key].(string); ok {
				level = strings.ToLower(value)
				break
			}
		}
	} else if match := controllerSummaryLevel.FindStringSubmatch(line); len(match) > 0 {
		level = strings.Trim(match[1], `"`)
		level = strings.ToLower(level)
	} else if match := controllerSummaryTextLevel.FindStringSubmatch(line); len(match) > 0 {
		level = strings.ToLower(match[1])
	} else if controllerKlogPrefix.MatchString(line) {
		switch line[0] {
		case 'I':
			level = "info"
		case 'W':
			level = "warn"
		}
	}
	// Reuse the analyzer's severity handling. Explicit non-error levels take
	// precedence over words such as "error:" inside an informational message.
	message := line
	if level == "" || level == "error" || level == "fatal" || level == "panic" {
		if normalized, isError := controllerErrorMessage(line); isError {
			message, level = normalized, "error"
		}
	}
	if structured {
		var parts []string
		for _, field := range []string{"msg", "message", "error", "err"} {
			if value, ok := record[field]; ok && value != nil {
				parts = append(parts, fmt.Sprint(value))
			}
		}
		if len(parts) > 0 {
			message = strings.Join(parts, ": ")
		}
	} else {
		var parts []string
		for _, match := range controllerSummaryFields.FindAllStringSubmatch(message, -1) {
			value := match[1]
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
			parts = append(parts, value)
		}
		if len(parts) > 0 {
			message = strings.Join(parts, ": ")
		} else {
			message = controllerKlogPrefix.ReplaceAllString(message, "")
			message = controllerSummaryPrefix.ReplaceAllString(message, "")
		}
	}
	if level == "warn" || level == "warning" {
		return message, "warn"
	}
	if level == "error" {
		return message, level
	}
	if level == "" && controllerSummaryWarning.MatchString(line) {
		return message, "warn"
	}
	if strings.Contains(strings.ToLower(message), "waitingforowner") {
		return message, "info"
	}
	return "", ""
}

func classifyControllerSummary(message string) (string, string) {
	lower := strings.ToLower(message)
	for _, problem := range []string{"permission denied", "forbidden", "unauthorized", "certificate has expired", "panic", "fatal"} {
		if strings.Contains(lower, problem) {
			return controllerPotentialIssue, "Inspect controller logs, resource conditions and configuration"
		}
	}
	switch {
	case strings.Contains(lower, "waitingforowner"):
		return controllerInformational, "Usually dependency ordering; investigate if the owner never becomes ready"
	case strings.Contains(lower, "connection to the workload cluster is down"):
		return controllerTransient, "May occur during provisioning; investigate if cluster connectivity does not recover"
	case strings.Contains(lower, "tls handshake error"):
		return controllerTransient, "May occur during startup; investigate persistent TLS or certificate failures"
	case strings.Contains(lower, "retryclassification: retryfast"):
		return controllerTransient, "Controller intends to retry; investigate repeated failures without progress"
	default:
		return controllerPotentialIssue, "Inspect controller logs and resource conditions"
	}
}

func formatControllerLogGroups(out *strings.Builder, summary ControllerLogSummary) {
	for _, category := range []string{controllerPotentialIssue, controllerTransient, controllerInformational} {
		count := summary.CategoryCounts[category]
		if count == 0 {
			continue
		}
		fmt.Fprintf(out, "   %s (%d occurrences):\n", category, count)
		displayed, occurrences := 0, 0
		for _, group := range summary.Groups {
			if group.Category != category || displayed >= 3 {
				continue
			}
			message := group.Message
			if len(message) > 200 {
				message = message[:200] + "..."
			}
			fmt.Fprintf(out, "     - [%dx] %s\n       %s\n", group.Count, message, group.Explanation)
			displayed++
			occurrences += group.Count
		}
		if occurrences < count {
			fmt.Fprintf(out, "     ... %d additional occurrences in unlisted groups; inspect the saved log\n", count-occurrences)
		}
	}
}
