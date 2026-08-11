package appcatalog

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var unresolvedCredentialPattern = regexp.MustCompile(`(?i)(YOUR_[A-Z0-9_]+|CHANGE[_ -]?ME|<[^>]*(TOKEN|PASSWORD|SECRET|KEY)[^>]*>)`)

// HasUnresolvedCredentialPlaceholder prevents a catalog's example token from
// being mistaken for a usable credential by API clients that skip the UI.
func HasUnresolvedCredentialPlaceholder(line string) bool {
	return unresolvedCredentialPattern.MatchString(line)
}

// ParseArguments turns a catalog's shell-style argument line into Docker's
// exec-form Cmd array. It intentionally performs no expansion or execution:
// quotes and backslashes only group literal arguments.
func ParseArguments(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	started := false

	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}

	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			started = true
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
			continue
		}
		switch {
		case r == '\'' || r == '"':
			quote = r
			started = true
		case unicode.IsSpace(r):
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("arguments end with an unfinished escape")
	}
	if quote != 0 {
		return nil, fmt.Errorf("arguments contain an unclosed quote")
	}
	flush()
	return args, nil
}
