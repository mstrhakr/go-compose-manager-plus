package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const maxResponseBytes = 16 << 20
const dockerResponseHeaderTimeout = 5 * time.Second

type Container struct {
	ID      string
	Name    string
	Project string
	Service string
	State   string
}

type Snapshot struct {
	Containers []Container
	ByProject  map[string][]Container
}

type Client struct {
	http   *http.Client
	stream *http.Client
}

func NewClient(socket string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
		DisableCompression:    true,
		ResponseHeaderTimeout: dockerResponseHeaderTimeout,
	}
	noRedirect := func(*http.Request, []*http.Request) error {
		return errors.New("Docker API redirect refused")
	}
	return &Client{http: &http.Client{
		Transport:     transport,
		Timeout:       5 * time.Second,
		CheckRedirect: noRedirect,
	}, stream: &http.Client{Transport: transport, CheckRedirect: noRedirect}}
}

// Snapshot uses only the Docker Engine container-list endpoint.
func (c *Client) Snapshot(ctx context.Context) (*Snapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/v1.40/containers/json?all=1", nil)
	if err != nil {
		return nil, fmt.Errorf("create Docker request: %w", err)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list Docker containers: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Docker container list returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Docker container list: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("Docker container list exceeds size limit")
	}
	var containers []struct {
		ID     string            `json:"Id"`
		Names  []string          `json:"Names"`
		State  string            `json:"State"`
		Labels map[string]string `json:"Labels"`
	}
	if err := json.Unmarshal(body, &containers); err != nil {
		return nil, fmt.Errorf("decode Docker container list: %w", err)
	}
	if containers == nil {
		return nil, errors.New("Docker container list is not an array")
	}
	result := &Snapshot{ByProject: make(map[string][]Container)}
	for _, container := range containers {
		project := container.Labels["com.docker.compose.project"]
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		state := container.State
		if state == "" {
			state = "unknown"
		}
		entry := Container{ID: container.ID, Name: name, Project: project, Service: container.Labels["com.docker.compose.service"], State: state}
		result.Containers = append(result.Containers, entry)
		if project != "" {
			result.ByProject[project] = append(result.ByProject[project], entry)
		}
	}
	return result, nil
}
