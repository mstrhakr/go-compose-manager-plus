package metrics

import (
	"context"
	"errors"
	"fmt"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func (c *Collector) eligibleProjects(ctx context.Context, scope CollectionScope) (map[string]bool, error) {
	projectsByName := make(map[string]bool)
	if scope == ScopeAll {
		return projectsByName, nil
	}
	if scope != ScopeActivated {
		return nil, errors.New("invalid collection scope")
	}
	projects, err := c.projects.Projects(ctx)
	if err != nil {
		return nil, fmt.Errorf("read activated projects: %w", err)
	}
	var scopeErr error
	for _, project := range projects {
		if !project.Active || !project.Available || project.ResolutionError != "" {
			continue
		}
		name, err := discovery.ResolveProjectName(ctx, project.Root, project.Path)
		if err != nil {
			scopeErr = errors.Join(scopeErr, fmt.Errorf("resolve activated project %s: %w", project.Path, err))
			continue
		}
		projectsByName[name] = true
	}
	return projectsByName, scopeErr
}

func containerInScope(container runtime.Container, scope CollectionScope, projectsByName map[string]bool) bool {
	if scope == ScopeAll {
		return true
	}
	return scope == ScopeActivated && projectsByName[container.Project]
}
