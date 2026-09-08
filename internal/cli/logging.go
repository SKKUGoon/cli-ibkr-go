package cli

import "strings"

// The port emits warnings only. Preserve the global and worker-module filters used by IBKR_LOG.
func shouldLogWarning(filter string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	levels := map[string]int{"off": 0, "error": 1, "warn": 2, "info": 3, "debug": 4, "trace": 5}
	selected := 1
	specificity := -1
	for _, directive := range strings.Split(filter, ",") {
		target, level, hasTarget := strings.Cut(strings.TrimSpace(directive), "=")
		if !hasTarget {
			level = target
			target = ""
		}
		severity, valid := levels[strings.ToLower(level)]
		if !valid {
			return true
		}
		if target == "" || target == "worker" || strings.HasPrefix("worker::commands::market_data", target) || strings.HasPrefix("worker::commands::run", target) {
			if len(target) >= specificity {
				selected = severity
				specificity = len(target)
			}
		}
	}
	return selected >= 2
}
