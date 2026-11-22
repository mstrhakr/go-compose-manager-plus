package discovery

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
)

const maxDotEnvBytes = 1 << 20
const maxDotEnvLines = 4096
const maxDotEnvStatementBytes = 128 << 10
const maxDotEnvStatementLines = 256

// Parse one complete statement at a time so references are checked against
// variables defined before that statement, not later entries in the file.
func readLocalEnv(path string) (map[string]string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxDotEnvBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxDotEnvBytes {
		return nil, false, errors.New("project .env exceeds the supported size")
	}
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) > maxDotEnvLines {
		return nil, false, errors.New("project .env exceeds the supported line count")
	}
	values := make(map[string]string)
	var pending strings.Builder
	var lastErr error
	missing := false
	statementLines := 0
	for _, line := range lines {
		pending.WriteString(line)
		statementLines++
		if pending.Len() > maxDotEnvStatementBytes || statementLines > maxDotEnvStatementLines {
			return nil, false, errors.New("project .env statement exceeds the supported size")
		}
		unresolved := false
		parsed, err := dotenv.ParseWithLookup(strings.NewReader(pending.String()), func(key string) (string, bool) {
			value, ok := values[key]
			if !ok {
				unresolved = true
			}
			return value, ok
		})
		if err != nil {
			lastErr = err
			continue
		}
		missing = missing || unresolved
		for key, value := range parsed {
			values[key] = value
		}
		pending.Reset()
		statementLines = 0
		lastErr = nil
	}
	if lastErr != nil {
		return nil, false, lastErr
	}
	return values, missing, nil
}
