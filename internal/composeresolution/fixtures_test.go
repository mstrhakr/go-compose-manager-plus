package composeresolution_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

type fixture struct {
	files     map[string]string
	env       []string
	name      string
	image     string
	fileNames []string
}

func TestComposeResolutionFixtures(t *testing.T) {
	tests := map[string]fixture{
		"top-level name and interpolation": {
			files: map[string]string{
				"compose.yaml": "name: example\nservices:\n  web:\n    image: \"example:${TAG:-fallback}\"\n",
				".env":         "TAG=fixture\n",
			},
			name: "example", image: "example:fixture", fileNames: []string{"compose.yaml"},
		},
		"default name and override": {
			files: map[string]string{
				"compose.yaml":          "services:\n  web:\n    image: example:base\n",
				"compose.override.yaml": "services:\n  web:\n    image: example:override\n",
			},
			name: "fixture_project", image: "example:override", fileNames: []string{"compose.yaml", "compose.override.yaml"},
		},
		"default filename precedence": {
			files: map[string]string{
				"compose.yaml":         "services:\n  web:\n    image: example:preferred\n",
				"compose.yml":          "services:\n  web:\n    image: example:ignored\n",
				"docker-compose.yml":   "services:\n  web:\n    image: example:ignored\n",
				"docker-compose.yaml":  "services:\n  web:\n    image: example:ignored\n",
				"compose.override.yml": "services:\n  web:\n    image: example:override\n",
			},
			name: "fixture_project", image: "example:override", fileNames: []string{"compose.yaml", "compose.override.yml"},
		},
		"required interpolation": {
			files: map[string]string{
				"compose.yaml": "name: explicit\nservices:\n  web:\n    image: \"example:${TAG:?TAG is required}\"\n",
			},
			env: []string{"TAG=provided"}, name: "explicit", image: "example:provided", fileNames: []string{"compose.yaml"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dir := writeFixture(t, test.files)
			project, err := loadFixture(dir, nil, test.env, "")
			if err != nil {
				t.Fatal(err)
			}
			checkProject(t, project, test)
			compareCLI(t, dir, test, project)
		})
	}
}

func TestComposeResolutionRejectsInvalidInputs(t *testing.T) {
	tests := map[string]struct {
		files map[string]string
		error string
	}{
		"required interpolation missing": {
			files: map[string]string{"compose.yaml": "services:\n  web:\n    image: \"example:${TAG:?TAG is required}\"\n"},
			error: "TAG is required",
		},
		"invalid yaml": {
			files: map[string]string{"compose.yaml": "services: [\n"},
			error: "yaml",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dir := writeFixture(t, test.files)
			if _, err := loadFixture(dir, nil, nil, ""); err == nil || !strings.Contains(err.Error(), test.error) {
				t.Fatalf("invalid fixture: got error %v, want %q", err, test.error)
			}
		})
	}
	t.Run("missing explicit environment file", func(t *testing.T) {
		dir := writeFixture(t, map[string]string{
			"compose.yaml": "services:\n  web:\n    image: example:base\n",
			".env":         "TAG=must-not-be-substituted\n",
		})
		if _, err := loadFixture(dir, nil, nil, filepath.Join(dir, "missing.env")); err == nil || !strings.Contains(err.Error(), "missing.env") {
			t.Fatalf("missing explicit environment file: got error %v", err)
		}
	})
	t.Run("missing explicit compose file", func(t *testing.T) {
		dir := writeFixture(t, nil)
		if _, err := loadFixture(dir, []string{filepath.Join(dir, "missing.yaml")}, nil, ""); err == nil || !strings.Contains(err.Error(), "missing.yaml") {
			t.Fatalf("missing explicit compose file: got error %v", err)
		}
	})
}

func TestExplicitEnvironmentAndComposeSelection(t *testing.T) {
	dir := writeFixture(t, map[string]string{
		"compose.yaml":  "services:\n  web:\n    image: \"example:${TAG}\"\n",
		"selected.yaml": "name: selected\nservices:\n  web:\n    image: \"example:${TAG}\"\n",
		"other.yaml":    "services:\n  web:\n    image: example:other\n",
		"custom.env":    "TAG=custom\n",
		".env":          "TAG=default\n",
	})
	envFile := filepath.Join(dir, "custom.env")
	project, err := loadFixture(dir, nil, nil, envFile)
	if err != nil {
		t.Fatal(err)
	}
	checkProject(t, project, fixture{name: "fixture_project", image: "example:custom", fileNames: []string{"compose.yaml"}})

	project, err = loadFixture(dir, []string{filepath.Join(dir, "selected.yaml")},
		[]string{"COMPOSE_FILE=" + filepath.Join(dir, "other.yaml")}, envFile)
	if err != nil {
		t.Fatal(err)
	}
	checkProject(t, project, fixture{name: "selected", image: "example:custom", fileNames: []string{"selected.yaml"}})
}

func TestComposeFileEnvironmentSelectsOrderedFiles(t *testing.T) {
	dir := writeFixture(t, map[string]string{
		"compose.yaml": "services:\n  web:\n    image: example:default\n",
		"custom.yaml":  "name: custom\nservices:\n  web:\n    image: example:custom\n",
		"extra.yaml":   "services:\n  web:\n    image: example:extra\n",
	})
	env := []string{"COMPOSE_FILE=" + strings.Join([]string{filepath.Join(dir, "custom.yaml"), filepath.Join(dir, "extra.yaml")}, string(os.PathListSeparator))}
	project, err := loadFixture(dir, nil, env, "")
	if err != nil {
		t.Fatal(err)
	}
	test := fixture{name: "custom", image: "example:extra", fileNames: []string{"custom.yaml", "extra.yaml"}, env: env}
	checkProject(t, project, test)
	compareCLI(t, dir, test, project)
}

func loadFixture(dir string, files, environment []string, envFile string) (*types.Project, error) {
	var envFiles []string
	if envFile != "" {
		envFiles = []string{envFile}
	}
	options, err := cli.NewProjectOptions(files,
		cli.WithWorkingDirectory(dir),
		cli.WithEnv(environment),
		cli.WithEnvFiles(envFiles...),
		cli.WithDotEnv,
		cli.WithConfigFileEnv,
		cli.WithDefaultConfigPath,
		cli.WithEnvFiles(envFiles...),
		cli.WithDotEnv,
		cli.WithDefaultProfiles(),
	)
	if err != nil {
		return nil, err
	}
	return options.LoadProject(context.Background())
}

func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("COMPOSE_DISABLE_ENV_FILE", "false")
	dir := filepath.Join(t.TempDir(), "Fixture_Project")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Errorf("read fixture directory: %v", err)
			return
		}
		if len(entries) != len(files) {
			t.Errorf("fixture directory has %d entries, want %d", len(entries), len(files))
		}
		for path, content := range files {
			got, err := os.ReadFile(filepath.Join(dir, path))
			if err != nil || string(got) != content {
				t.Errorf("fixture %s changed: err=%v", path, err)
			}
		}
	})
	return dir
}

func checkProject(t *testing.T, project *types.Project, test fixture) {
	t.Helper()
	if project.Name != test.name || project.Services["web"].Image != test.image {
		t.Fatalf("name=%q image=%q, want %q and %q", project.Name, project.Services["web"].Image, test.name, test.image)
	}
	var files []string
	for _, path := range project.ComposeFiles {
		files = append(files, filepath.Base(path))
	}
	if !reflect.DeepEqual(files, test.fileNames) {
		t.Fatalf("selected files=%v, want %v", files, test.fileNames)
	}
}

func compareCLI(t *testing.T, dir string, test fixture, project *types.Project) {
	t.Helper()
	if os.Getenv("COMPOSE_FIXTURE_COMPARE_CLI") != "1" {
		return
	}
	cmd := exec.Command("docker", "compose", "config", "--format", "json")
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}, test.env...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("docker compose config: %v: %s", err, exitErr.Stderr)
		}
		t.Fatalf("docker compose config: %v", err)
	}
	var actual struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image string `json:"image"`
		} `json:"services"`
	}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Name != project.Name || actual.Services["web"].Image != project.Services["web"].Image {
		t.Fatalf("CLI name=%q image=%q, loader name=%q image=%q", actual.Name, actual.Services["web"].Image, project.Name, project.Services["web"].Image)
	}
}
