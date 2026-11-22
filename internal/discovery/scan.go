package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/template"
)

type ScanResult struct {
	Projects       []Project
	SkippedSymlink []string
}

// ValidateProject checks a single discovered path without registering other projects.
func ValidateProject(ctx context.Context, root, path string) error {
	if root == "" || filepath.Dir(path) != root || filepath.Clean(path) != path {
		return errors.New("project is outside the saved discovery root")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return errors.New("project directory is unavailable or is a symlink; scan again")
	}
	recognized, err := recognizedProject(path)
	if err != nil {
		return errors.New("project directory cannot be inspected")
	}
	if !recognized {
		return errors.New("project Compose file is unavailable; scan again")
	}
	return resolve(ctx, path)
}

var interpolationName = regexp.MustCompile(`^\$\{?([a-zA-Z_][a-zA-Z0-9_]*)`)
var errUnsupportedComposeFile = errors.New("COMPOSE_FILE selection is not yet supported during discovery")

func Scan(ctx context.Context, root string) (ScanResult, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ScanResult{}, fmt.Errorf("read discovery root: %w", err)
	}
	var result ScanResult
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return ScanResult{}, err
		}
		path := filepath.Join(root, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			result.SkippedSymlink = append(result.SkippedSymlink, entry.Name())
			continue
		}
		if !entry.IsDir() {
			continue
		}
		recognized, err := recognizedProject(path)
		if err != nil {
			return ScanResult{}, fmt.Errorf("inspect project directory %q: %w", entry.Name(), err)
		}
		if !recognized {
			continue
		}
		project := Project{Path: path, Root: root}
		if err := resolve(ctx, path); err != nil {
			if ctx.Err() != nil {
				return ScanResult{}, ctx.Err()
			}
			project.ResolutionError = err.Error()
		}
		result.Projects = append(result.Projects, project)
	}
	return result, nil
}

func recognizedProject(path string) (bool, error) {
	for _, name := range cli.DefaultFileNames {
		info, err := os.Lstat(filepath.Join(path, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
}

func resolve(ctx context.Context, path string) error {
	var missingVariables bool
	environment := make(map[string]string)
	envPath := filepath.Join(path, ".env")
	info, err := os.Stat(envPath)
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("project .env is not a regular file")
		}
		var lookedUp []string
		environment, err = dotenv.ReadWithLookup(func(name string) (string, bool) {
			lookedUp = append(lookedUp, name)
			return "", false
		}, envPath)
		if err != nil {
			return errors.New("unable to read project interpolation environment")
		}
		for _, name := range lookedUp {
			if _, found := environment[name]; !found {
				missingVariables = true
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("project .env cannot be accessed")
	}
	options, err := cli.NewProjectOptions(nil,
		cli.WithWorkingDirectory(path),
		func(options *cli.ProjectOptions) error {
			options.Environment.Merge(environment)
			return nil
		},
		func(options *cli.ProjectOptions) error {
			if _, ok := options.Environment["COMPOSE_FILE"]; ok {
				return errUnsupportedComposeFile
			}
			return nil
		},
		cli.WithDefaultConfigPath,
		cli.WithDefaultProfiles(),
		cli.WithLoadOptions(func(options *loader.Options) {
			if options.Interpolate == nil {
				return
			}
			options.Interpolate.Substitute = func(input string, mapping template.Mapping) (string, error) {
				return template.SubstituteWithOptions(input, mapping, template.WithoutLogging,
					template.WithReplacementFunction(func(substring string, mapping template.Mapping, config *template.Config) (string, error) {
						var missing []string
						value, applied, err := template.DefaultReplacementAppliedFunc(substring, func(name string) (string, bool) {
							value, ok := mapping(name)
							if !ok {
								missing = append(missing, name)
							}
							return value, ok
						}, config)
						if err == nil && len(missing) > 0 {
							// A missing outer variable with an applied fallback is expected.
							match := interpolationName.FindStringSubmatch(substring)
							if !applied || len(missing) != 1 || len(match) != 2 || missing[0] != match[1] {
								missingVariables = true
							}
						}
						return value, err
					}))
			}
		}),
	)
	if err != nil {
		if errors.Is(err, errUnsupportedComposeFile) {
			return err
		}
		return errors.New("unable to read project interpolation environment")
	}
	if len(options.ConfigPaths) == 0 || filepath.Dir(options.ConfigPaths[0]) != path {
		return errors.New("project Compose file is unavailable")
	}
	if _, err := options.LoadProject(ctx); err != nil {
		return errors.New("unable to resolve Compose inputs; check the project files, permissions and interpolation")
	}
	if missingVariables {
		return errors.New("project interpolation refers to unset variables; manager environment is not inherited")
	}
	return nil
}
