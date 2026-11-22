package discovery

import (
	"strings"

	"github.com/compose-spec/compose-go/v2/template"
)

// missingInterpolation checks only branches Compose will interpolate; compose-go
// remains responsible for substitution and validation.
func missingInterpolation(input string, lookup template.Mapping) bool {
	for i := 0; i < len(input); i++ {
		if input[i] != '$' || i+1 == len(input) {
			continue
		}
		if input[i+1] == '$' {
			i++
			continue
		}
		if input[i+1] != '{' {
			start := i + 1
			end := variableEnd(input, start)
			if end > start {
				if _, ok := lookup(input[start:end]); !ok {
					return true
				}
				i = end - 1
			}
			continue
		}
		start := i + 2
		end := variableEnd(input, start)
		if end == start {
			continue
		}
		depth, close := 1, end
		for ; close < len(input) && depth > 0; close++ {
			if input[close] == '{' {
				depth++
			} else if input[close] == '}' {
				depth--
			}
		}
		if depth != 0 {
			return false
		}
		expression := input[end : close-1]
		value, ok := lookup(input[start:end])
		operator, rest := "", ""
		for _, candidate := range []string{":-", ":+", ":?", "-", "+", "?"} {
			if strings.HasPrefix(expression, candidate) {
				operator, rest = candidate, expression[len(candidate):]
				break
			}
		}
		switch operator {
		case "":
			if !ok {
				return true
			}
		case ":-":
			if (!ok || value == "") && missingInterpolation(rest, lookup) {
				return true
			}
		case "-":
			if !ok && missingInterpolation(rest, lookup) {
				return true
			}
		case ":+":
			if ok && value != "" && missingInterpolation(rest, lookup) {
				return true
			}
		case "+":
			if ok && missingInterpolation(rest, lookup) {
				return true
			}
		}
		i = close - 1
	}
	return false
}

func variableEnd(input string, start int) int {
	end := start
	for end < len(input) {
		c := input[end]
		if c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || end > start && c >= '0' && c <= '9' {
			end++
			continue
		}
		break
	}
	return end
}
