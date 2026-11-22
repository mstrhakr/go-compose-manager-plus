package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLiveDockerSnapshot(t *testing.T) {
	if os.Getenv("CMP_LIVE_DOCKER") != "1" {
		t.Skip("set CMP_LIVE_DOCKER=1 and CMP_LIVE_ACK to the exact Docker socket path")
	}
	socket := os.Getenv("CMP_LIVE_ACK")
	if socket == "" || !filepath.IsAbs(socket) || filepath.Clean(socket) != socket {
		t.Fatal("CMP_LIVE_ACK must be the exact absolute Docker socket path")
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	project := "cmp-test-" + hex.EncodeToString(nonce[:])
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	docker := func(args ...string) (string, error) {
		command := exec.CommandContext(ctx, "docker", args...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "DOCKER_HOST=unix://" + socket}
		output, err := command.CombinedOutput()
		return strings.TrimSpace(string(output)), err
	}
	list := func() ([]string, error) {
		out, err := docker("ps", "-aq", "--no-trunc", "--filter", "label=com.docker.compose.project="+project)
		if err != nil || out == "" {
			return nil, err
		}
		return strings.Fields(out), nil
	}
	ids, err := list()
	if err != nil || len(ids) != 0 {
		t.Fatalf("refusing test: generated project is not empty or Docker is unavailable: %v", err)
	}
	image, err := docker("image", "inspect", "ghcr.io/mstrhakr/go-compose-manager-plus:dev", "--format", "{{.Id}}")
	if err != nil || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(image) {
		t.Fatalf("test requires the already-local development image; no pull is performed: %v", err)
	}
	contents := fmt.Sprintf("name: %s\nservices:\n  app:\n    image: %s\n    command: [\"-version\"]\n    network_mode: none\n    read_only: true\n    cap_drop: [ALL]\n    mem_limit: 64m\n    cpus: 0.1\n    pids_limit: 32\n    labels:\n      cmp.test.owner: %s\n", project, image, project)
	file := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(file, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		cmd := exec.CommandContext(cleanupCtx, "docker", "ps", "-aq", "--no-trunc", "--filter", "label=com.docker.compose.project="+project)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "DOCKER_HOST=unix://" + socket}
		out, err := cmd.Output()
		if err != nil {
			t.Errorf("fixture cleanup could not enumerate %s: %v", project, err)
			return
		}
		found := strings.Fields(string(out))
		if len(found) > 1 {
			t.Errorf("fixture cleanup refused unexpected multiple containers for %s", project)
			return
		}
		for _, id := range found {
			if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(id) {
				t.Errorf("fixture cleanup refused invalid container ID for %s", project)
				return
			}
			inspect := exec.CommandContext(cleanupCtx, "docker", "inspect", "--format", `{{index .Config.Labels "cmp.test.owner"}}`, id)
			inspect.Env = cmd.Env
			owner, err := inspect.Output()
			if err != nil || strings.TrimSpace(string(owner)) != project {
				t.Errorf("fixture cleanup refused unverified container %s: %v", id, err)
				return
			}
			remove := exec.CommandContext(cleanupCtx, "docker", "rm", "-f", id)
			remove.Env = cmd.Env
			if output, err := remove.CombinedOutput(); err != nil {
				t.Errorf("fixture cleanup failed for %s: %v: %s", id, err, output)
				return
			}
		}
		verify := exec.CommandContext(cleanupCtx, "docker", "ps", "-aq", "--no-trunc", "--filter", "label=com.docker.compose.project="+project)
		verify.Env = cmd.Env
		remaining, err := verify.Output()
		if err != nil || strings.TrimSpace(string(remaining)) != "" {
			t.Errorf("fixture cleanup could not verify removal for %s: %v", project, err)
		}
	})
	output, err := docker("compose", "--project-directory", dir, "-f", file, "create", "--no-build", "--pull", "never")
	if err != nil {
		t.Fatalf("create disposable project: %v: %s", err, output)
	}
	ids, err = list()
	if err != nil || len(ids) != 1 {
		t.Fatalf("expected exactly one fixture container for %s: %v", project, err)
	}
	snapshot, err := NewClient(socket).Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if containers := snapshot[project]; len(containers) != 1 || containers[0].Service != "app" || containers[0].State != "created" {
		t.Fatalf("fixture snapshot = %#v", containers)
	}
}
