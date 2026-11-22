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

type Container struct {
	Name    string
	Service string
	State   string
}

type Client struct {
	http *http.Client
}

func NewClient(socket string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
		DisableCompression: true,
	}
	return &Client{http: &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("Docker API redirect refused")
		},
	}}
}

// Snapshot uses only the Docker Engine container-list endpoint.
func (c *Client) Snapshot(ctx context.Context) (map[string][]Container, error) {
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
	result := make(map[string][]Container)
	for _, container := range containers {
		project := container.Labels["com.docker.compose.project"]
		if project == "" {
			continue
		}
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		state := container.State
		if state == "" {
			state = "unknown"
		}
		result[project] = append(result[project], Container{
			Name: name, Service: container.Labels["com.docker.compose.service"], State: state,
		})
	}
	return result, nil
}
