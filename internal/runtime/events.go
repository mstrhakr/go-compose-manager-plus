package runtime

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxEventBytes = 1 << 20

// ContainerEvent is a Docker event, not an authoritative current state.
type ContainerEvent struct {
	ID      string
	Project string
	Service string
	Action  string
	Health  string
	Time    time.Time
}

// WatchContainerEvents streams read-only container events until cancellation,
// disconnection or a callback error. EOF is an error: reconnects can have gaps.
// The caller owns reconciliation, scope, retry and replay deduplication.
func (c *Client) WatchContainerEvents(ctx context.Context, consume func(ContainerEvent) error) error {
	if consume == nil {
		return errors.New("container event callback is required")
	}
	if c.stream == nil {
		return errors.New("Docker event stream is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		`http://docker/v1.40/events?filters=%7B%22type%22%3A%5B%22container%22%5D%7D`, nil)
	if err != nil {
		return fmt.Errorf("create Docker event request: %w", err)
	}
	response, err := c.stream.Do(request)
	if err != nil {
		return fmt.Errorf("request Docker events: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Docker events returned HTTP %d", response.StatusCode)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), maxEventBytes)
	for scanner.Scan() {
		var raw struct {
			Type   string `json:"Type"`
			Action string `json:"Action"`
			Actor  struct {
				ID         string            `json:"ID"`
				Attributes map[string]string `json:"Attributes"`
			} `json:"Actor"`
			TimeNano int64 `json:"timeNano"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			return fmt.Errorf("decode Docker event: %w", err)
		}
		if raw.Type != "container" {
			continue
		}
		action, health := raw.Action, ""
		if strings.HasPrefix(action, "health_status: ") {
			health = strings.TrimPrefix(action, "health_status: ")
			switch health {
			case "starting", "healthy", "unhealthy":
				action = "health_status"
			default:
				return fmt.Errorf("unsupported Docker health event %q", health)
			}
		}
		switch action {
		case "start", "stop", "die", "restart", "oom", "health_status":
		default:
			continue
		}
		if action == "health_status" && health == "" {
			return errors.New("Docker health event has no status")
		}
		if len(raw.Actor.ID) != 64 {
			return errors.New("Docker event has invalid container ID")
		}
		if _, err := hex.DecodeString(raw.Actor.ID); err != nil {
			return fmt.Errorf("Docker event has invalid container ID: %w", err)
		}
		if raw.TimeNano <= 0 {
			return errors.New("Docker event has no valid nanosecond timestamp")
		}
		event := ContainerEvent{
			ID: raw.Actor.ID, Project: raw.Actor.Attributes["com.docker.compose.project"],
			Service: raw.Actor.Attributes["com.docker.compose.service"], Action: action,
			Health: health, Time: time.Unix(0, raw.TimeNano).UTC(),
		}
		if err := consume(event); err != nil {
			return fmt.Errorf("consume Docker event: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Docker event stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("Docker event stream ended: %w", err)
	}
	return errors.New("Docker event stream ended unexpectedly")
}
