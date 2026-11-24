package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/logging"
	"github.com/sirupsen/logrus"
)

func TestConfigureNormalizesComposeAndStandardLogs(t *testing.T) {
	previous := slog.Default()
	standard := logrus.StandardLogger()
	output, hooks, level := standard.Out, standard.Hooks, standard.GetLevel()
	stdOutput, stdFlags, stdPrefix := log.Writer(), log.Flags(), log.Prefix()
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(stdOutput)
		log.SetFlags(stdFlags)
		log.SetPrefix(stdPrefix)
		standard.SetOutput(output)
		standard.ReplaceHooks(hooks)
		standard.SetLevel(level)
	})
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logging.Configure(logger)
	logging.Configure(logger)
	root := t.TempDir()
	dir := filepath.Join(root, "fixture")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("version: '3.8'\nservices:\n  web:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := discovery.ResolveProjectName(context.Background(), root, dir); err != nil {
		t.Fatal(err)
	}
	logrus.WithField("component", "fixture").Warn("quoted \"warning\"\nsecond line")
	logrus.Debug("SDK configuration dump must remain disabled")
	log.Print("standard library diagnostic")
	lines := bytes.Split(bytes.TrimSpace(buffer.Bytes()), []byte{'\n'})
	if len(lines) != 3 {
		t.Fatalf("expected exactly three JSON records, without SDK debug or duplicate hooks: %s", buffer.String())
	}
	var records [3]struct {
		Level  string            `json:"level"`
		Msg    string            `json:"msg"`
		Source string            `json:"source"`
		Fields map[string]string `json:"fields"`
	}
	for index, line := range lines {
		if err := json.Unmarshal(line, &records[index]); err != nil {
			t.Fatalf("unstructured diagnostic: %s: %v", line, err)
		}
	}
	if records[0].Level != "WARN" || records[0].Source != "compose" || !strings.Contains(records[0].Msg, "obsolete") ||
		records[1].Msg != "quoted \"warning\"\nsecond line" || records[1].Fields["component"] != "fixture" ||
		records[2].Level != "INFO" || records[2].Msg != "standard library diagnostic" {
		t.Fatalf("diagnostic level/message/fields were not preserved: %#v", records)
	}
}
