package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalEnvUsesEarlierValuesAndSupportsMultiline(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	text := "BASE=example\nTAG=${BASE}\nMULTI='first\nsecond'\nFORWARD=${LATER}\nLATER=ready\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	values, missing, err := readLocalEnv(path)
	if err != nil || !missing || values["TAG"] != "example" || values["MULTI"] != "first\nsecond" || values["FORWARD"] != "" {
		t.Fatalf("values=%v missing=%v error=%v", values, missing, err)
	}
}

func TestLocalEnvRejectsOversizedStatementAndMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	for _, contents := range []string{"BROKEN='unterminated\n", "LONG='" + strings.Repeat("x", maxDotEnvStatementBytes) + "'\n"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readLocalEnv(path); err == nil {
			t.Fatal("expected invalid environment file to be rejected")
		}
	}
}
