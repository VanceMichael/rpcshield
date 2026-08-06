package servicemapping

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/uber-research/rpcshield/go/common/utils"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var _ignoredConfigFiles = map[string]bool{
	"staging.yaml":     true,
	"development.yaml": true,
	"test.yaml":        true,
}

type configAliasResolver struct {
	workspace      string
	configMap      map[string]map[string]string // binaryPkgPath -> alias -> service name.
	logger         *zap.SugaredLogger
	srcCodebaseDir string
}

func newConfigAliasResolver(workspace string, codebasePkgPrefix string, logger *zap.SugaredLogger) (*configAliasResolver, error) {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	logger.Infow("Scanning pinocchio config files in the workspace", "workspace", workspace)
	configAliasMap, err := extractConfigAliasMap(workspace)
	if err != nil {
		return nil, fmt.Errorf("extract config mappings: %w", err)
	}

	return &configAliasResolver{
		workspace:      workspace,
		configMap:      configAliasMap,
		logger:         logger,
		srcCodebaseDir: "src" + string(filepath.Separator) + codebasePkgPrefix,
	}, nil
}

// resolve resolves the actual service name from the given name from the package directory (must
// be relative to the workspace). If no alias is defined, the given name is returned as is.
func (c *configAliasResolver) resolve(pkgDir string, name string) string {
	// Try finding if the package directory is within a (collected) service directory that has a
	// config alias mapping.
	for dir := pkgDir; dir != c.srcCodebaseDir && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		aliasMap, ok := c.configMap[dir]
		if !ok {
			continue
		}
		if actual, ok := aliasMap[name]; ok {
			return actual
		}
		break
	}

	return name
}

func extractConfigAliasMap(workspace string) (map[string]map[string]string, error) {
	pinocchio := filepath.Join(workspace, "udeploy", "pinocchio")
	entries, err := os.ReadDir(pinocchio)
	if err != nil {
		return nil, fmt.Errorf("walk pinocchio directory: %w", err)
	}

	configMapping := make(map[string]map[string]string)
	// Read each production.yaml file and extract "go_standard.bazel_targets" as the inferred
	// service directory, and the "UBER_CONFIG_DIR" as the config directory.
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		production := filepath.Join(pinocchio, entry.Name(), "production.yaml")
		f, err := os.Open(production)
		// The production.yaml may not exist in the pinocchio directory, which is fine.
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("open production.yaml: %w", err)
		}

		var content struct {
			GoStandard struct {
				BazelTargets []string `yaml:"bazel_targets"`
			} `yaml:"go_standard"`
			EnvVars []struct {
				Name  string `yaml:"name"`
				Value string `yaml:"value"`
			} `yaml:"env_vars"`
		}
		if err := yaml.NewDecoder(f).Decode(&content); err != nil {
			return nil, fmt.Errorf("decode %q: %w", production, err)
		}

		var configDir string
		for _, envVar := range content.EnvVars {
			if envVar.Name == "UBER_CONFIG_DIR" {
				configDir = envVar.Value
				break
			}
		}
		// If UBER_CONFIG_DIR is not specified, we do not record it.
		if len(configDir) == 0 {
			continue
		}

		aliasMap, err := extractAliasMap(workspace, configDir)
		if err != nil {
			return nil, fmt.Errorf("extract alias map: %w", err)
		}

		// Construct the (inferred) service directory from the bazel targets.
		for _, target := range content.GoStandard.BazelTargets {
			// "//src/example.internal/foo/bar:rule_name" -> "src/example.internal/foo/bar"
			dir := strings.TrimPrefix(strings.Split(target, ":")[0], "//")

			// Sometimes the service binary directory is only a subdirectory of the service directory.
			// Here we employ a heuristics to extract the service directory (i.e., find the uppermost
			// "generic" directory name like "cmd", "handler", "gateway" etc. and hop to its parent
			// directory).
			parts := strings.Split(dir, "/")
			minIndex := len(parts)
			for hopDir := range utils.GenericServiceSubDirs {
				if i := slices.Index(parts, hopDir); i >= 0 && i < minIndex {
					minIndex = i
				}
			}
			dir = strings.Join(parts[:minIndex], "/")

			configMapping[dir] = aliasMap
		}
	}

	return configMapping, nil
}

func extractAliasMap(workspace string, configDir string) (map[string]string, error) {
	aliasMap := make(map[string]string)

	entries, err := os.ReadDir(filepath.Join(workspace, configDir))
	// We ignore errors since the specified config directory might be stale (i.e., no longer exists
	// within the monorepo) and we would just move on.
	if os.IsNotExist(err) {
		return aliasMap, nil
	} else if err != nil {
		return nil, fmt.Errorf("read config directory: %w", err)
	}

	// For deduplication purposes since files listed in meta.yaml file might duplicate the ones
	// in existing directory.
	configFiles := make(map[string]bool, len(entries))
	metaPaths, err := extractMetaYAMLPaths(workspace, configDir)
	if err != nil {
		return nil, fmt.Errorf("expand meta.yaml (%q): %w", filepath.Join(workspace, configDir, "meta.yaml"), err)
	}
	for _, path := range metaPaths {
		configFiles[filepath.Clean(filepath.Join(workspace, configDir, path))] = true
	}
	for _, entry := range entries {
		configFiles[filepath.Join(workspace, configDir, entry.Name())] = true
	}

	var configFilesSlice []string
	for configFile := range configFiles {
		// We would not consider the staging.yaml, development.yaml, test.yaml etc. files since
		// they usually contain aliases to staging, development, or test environments instead of
		// production environment.
		name := filepath.Base(configFile)
		if !strings.HasSuffix(name, ".yaml") || _ignoredConfigFiles[name] {
			continue
		}

		configFilesSlice = append(configFilesSlice, configFile)
	}

	// We should put "production*.yaml" first and "base.yaml" last since they more specific config
	// files override the more general ones. To have a deterministic order, we sort the other
	// config files alphabetically.
	slices.SortFunc(configFilesSlice, func(i, j string) int {
		switch {
		case strings.HasPrefix(filepath.Base(i), "production") || filepath.Base(j) == "base.yaml":
			return -1
		case strings.HasPrefix(filepath.Base(j), "production") || filepath.Base(i) == "base.yaml":
			return 1
		default:
			return cmp.Compare(i, j)
		}
	})

	for _, configFile := range configFilesSlice {
		f, err := os.Open(configFile)
		if errors.Is(err, os.ErrNotExist) {
			// We expand from meta.yaml files, and some config files might not exist, which is OK.
			continue
		} else if err != nil {
			return nil, fmt.Errorf("open config file: %w", err)
		}

		var content struct {
			YARPC struct {
				Outbounds map[string]map[string]any `yaml:"outbounds"`
			} `yaml:"yarpc"`
		}
		// It is OK for decoding to fail since they may not contain the "yarpc.outbounds" field,
		// which means there is no aliasing configurations - we just move on.
		if err := yaml.NewDecoder(f).Decode(&content); err != nil {
			continue
		}
		for alias, conf := range content.YARPC.Outbounds {
			actual, ok := conf["service"]
			if !ok {
				continue
			}
			if serviceStr, ok := actual.(string); ok {
				aliasMap[alias] = serviceStr
			}
		}
	}

	return aliasMap, nil
}

func extractMetaYAMLPaths(workspace string, configDir string) ([]string, error) {
	// meta.yaml contains meta information to include more config files, e.g.:
	// file:
	//  - base.yaml
	//  - ../base.yaml
	//  - ${UBER_RUNTIME_ENVIRONMENT:development}.yaml
	// If it exists, we should first parse it and put the listed files to the config file list.
	// Note that we also have to expand the environment variables in the file paths.
	metaFile := filepath.Join(workspace, configDir, "meta.yaml")
	buf, err := os.ReadFile(metaFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("open meta.yaml: %w", err)
	}

	var content struct {
		Files []string `yaml:"files"`
	}
	err = yaml.NewDecoder(strings.NewReader(expandEnvVars(string(buf)))).Decode(&content)
	if err != nil && !errors.Is(err, io.EOF) { // EOF is OK since it might be an empty meta.yaml (or just a comment).
		return nil, fmt.Errorf("decode meta.yaml: %w", err)
	}

	return content.Files, nil
}

// expandEnvVars expands environment variables in the given string. It also supports default values
// like "${VAR:default}". Moreover, it assumes "UBER_RUNTIME_ENVIRONMENT" to be "production".
func expandEnvVars(s string) string {
	// Custom expand function to handle default values like ${VAR:default}
	mapper := func(key string) string {
		parts := strings.SplitN(key, ":", 2)
		if parts[0] == "UBER_RUNTIME_ENVIRONMENT" {
			return "production" // Assume production.
		}

		if val, exists := os.LookupEnv(parts[0]); exists {
			return val
		} else if len(parts) > 1 && parts[1] != "\"\"" {
			return parts[1] // Use default value if the env var is not set
		}
		return "" // Return empty if no env var and no default
	}

	return os.Expand(s, mapper)
}
