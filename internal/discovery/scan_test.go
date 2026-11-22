package discovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFindsDirectProjectsAndReportsResolutionErrors(t *testing.T) {
	root := t.TempDir()
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("valid/compose.yaml", "services:\n  web:\n    image: 'example:${TAG:-fallback}'\n")
	write("invalid/docker-compose.yml", "services: [\n")
	write("nested/child/compose.yaml", "services:\n  web:\n    image: example\n")
	write("data/file.txt", "not a project")
	write("env-only/.env", "COMPOSE_FILE=custom.yaml\n")
	write("env-only/custom.yaml", "services:\n  web:\n    image: example\n")
	result, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Projects) != 2 {
		t.Fatalf("projects = %+v", result.Projects)
	}
	if got := result.Projects[0]; filepath.Base(got.Path) != "invalid" || got.ResolutionError == "" {
		t.Fatalf("invalid project = %+v", got)
	}
	if got := result.Projects[1]; filepath.Base(got.Path) != "valid" || got.ResolutionError != "" {
		t.Fatalf("valid project = %+v", got)
	}
}

func TestScanUsesLocalEnvAndNotProcessEnvironment(t *testing.T) {
	t.Setenv("DISCOVERY_TEST_SECRET", "must-not-be-used")
	root := t.TempDir()
	dir := filepath.Join(root, "project")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	compose := []byte("services:\n  web:\n    image: 'example:${DISCOVERY_TEST_SECRET}'\n")
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), compose, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(context.Background(), root)
	if err != nil || len(result.Projects) != 1 || !strings.Contains(result.Projects[0].ResolutionError, "unset") {
		t.Fatalf("missing variable result = %+v, error %v", result, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DISCOVERY_TEST_SECRET=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = Scan(context.Background(), root)
	if err != nil || len(result.Projects) != 1 || result.Projects[0].ResolutionError != "" {
		t.Fatalf("local env result = %+v, error %v", result, err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "compose.yaml"))
	if err != nil || string(got) != string(compose) {
		t.Fatalf("compose file changed: %v", err)
	}
}

func TestScanReportsUnsupportedAndMissingInterpolation(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "project")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(dir, "compose.yaml")
	envPath := filepath.Join(dir, ".env")
	for _, test := range []struct {
		name    string
		compose string
		env     string
		want    string
	}{
		{"nested missing", "services:\n  web:\n    image: 'example:${FIRST:-${SECOND}}'\n", "", "unset"},
		{"dotenv missing", "services:\n  web:\n    image: 'example:${TAG}'\n", "TAG=${OUTSIDE}\n", "unset"},
		{"compose file", "services:\n  web:\n    image: example\n", "COMPOSE_FILE=compose.yaml\n", "COMPOSE_FILE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(composePath, []byte(test.compose), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(envPath, []byte(test.env), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := Scan(context.Background(), root)
			if err != nil || len(result.Projects) != 1 || !strings.Contains(result.Projects[0].ResolutionError, test.want) {
				t.Fatalf("scan = %+v, error %v; want %q", result, err, test.want)
			}
		})
	}
}

func TestScanReportsSymlinksAndErrors(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result, err := Scan(context.Background(), root)
	if err != nil || len(result.Projects) != 0 || len(result.SkippedSymlink) != 1 || result.SkippedSymlink[0] != "linked" {
		t.Fatalf("symlink result = %+v, error %v", result, err)
	}
	if _, err := Scan(context.Background(), filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing root unexpectedly scanned")
	}
}
