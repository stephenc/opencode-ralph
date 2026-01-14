package ralph

import (
	"os"
	"strings"
)

const banner = `
   ____  ____  ____  _   _
  / __ \/ __ \/ __ \/ | / /
 / / / / /_/ / /_/ /  |/ /
/ /_/ / ____/ ____/ /|  /
\____/_/   /_/   /_/ |_/

opencode-ralph
`

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

func shouldUseColor(quiet bool) bool {
	if quiet {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func style(text string, codes ...string) string {
	if len(codes) == 0 {
		return text
	}

	var b strings.Builder
	for _, code := range codes {
		b.WriteString(code)
	}
	b.WriteString(text)
	b.WriteString(ansiReset)
	return b.String()
}

func styleIf(enabled bool, text string, codes ...string) string {
	if !enabled {
		return text
	}
	return style(text, codes...)
}

func statusStyle(status string) (string, []string) {
	switch strings.ToLower(status) {
	case "complete":
		return strings.ToUpper(status), []string{ansiGreen, ansiBold}
	case "rate_limited", "max_iterations":
		return strings.ToUpper(status), []string{ansiYellow, ansiBold}
	case "dry_run":
		return strings.ToUpper(status), []string{ansiCyan, ansiBold}
	case "unknown":
		return strings.ToUpper(status), []string{ansiGray}
	default:
		return strings.ToUpper(status), []string{ansiGray}
	}
}
