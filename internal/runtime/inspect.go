package runtime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ContainerInspection is a discrete Docker observation, not a continuous
// assertion about the container's state between inspections.
type ContainerInspection struct {
	ID        string
	State     string
	Health    string
	StartedAt time.Time
}

// InspectContainer queries the read-only Docker container inspection endpoint.
func (c *Client) InspectContainer(ctx context.Context, id string) (ContainerInspection, error) {
	if len(id) != 64 {
		return ContainerInspection{}, errors.New("Docker container ID must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return ContainerInspection{}, fmt.Errorf("invalid Docker container ID: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/v1.40/containers/"+id+"/json", nil)
	if err != nil {
		return ContainerInspection{}, fmt.Errorf("create Docker inspect request: %w", err)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return ContainerInspection{}, fmt.Errorf("inspect Docker container: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ContainerInspection{}, fmt.Errorf("Docker inspect returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return ContainerInspection{}, fmt.Errorf("read Docker inspect: %w", err)
	}
	if len(body) > maxResponseBytes {
		return ContainerInspection{}, errors.New("Docker inspect exceeds size limit")
	}
	var raw struct {
		ID     string `json:"Id"`
		Config *struct {
			Healthcheck *struct {
				Test []string `json:"Test"`
			} `json:"Healthcheck"`
		} `json:"Config"`
		State *struct {
			Status    string `json:"Status"`
			StartedAt string `json:"StartedAt"`
			Health    *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ContainerInspection{}, fmt.Errorf("decode Docker inspect: %w", err)
	}
	if raw.ID != id || raw.State == nil {
		return ContainerInspection{}, errors.New("Docker inspect returned a different container or missing state")
	}
	state := raw.State.Status
	switch state {
	case "created", "running", "paused", "restarting", "removing", "exited", "dead":
	default:
		state = "unknown"
	}
	health := "unknown"
	if raw.Config != nil && (raw.Config.Healthcheck == nil ||
		(len(raw.Config.Healthcheck.Test) > 0 && raw.Config.Healthcheck.Test[0] == "NONE")) {
		health = "none"
	}
	if raw.State.Health != nil {
		switch raw.State.Health.Status {
		case "starting", "healthy", "unhealthy":
			health = raw.State.Health.Status
		default:
			health = "unknown"
		}
	}
	var started time.Time
	if raw.State.StartedAt != "" {
		started, err = time.Parse(time.RFC3339Nano, raw.State.StartedAt)
		if err != nil {
			return ContainerInspection{}, fmt.Errorf("invalid Docker start time: %w", err)
		}
	}
	return ContainerInspection{ID: id, State: state, Health: health, StartedAt: started}, nil
}
