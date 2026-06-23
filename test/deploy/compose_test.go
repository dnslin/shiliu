package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	repoRoot      = "../.."
	composePath   = "deploy/docker-compose/docker-compose.yml"
	prodConfig    = "config/prod.yml"
	dockerfile    = "deploy/build/Dockerfile"
	storageVolume = "shiliu_storage"
	storageTarget = "/data/app/storage"
	imageName     = "shiliu-backend:local"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]composeVolume  `yaml:"volumes"`
}

type composeVolume struct {
	Name string `yaml:"name"`
}

type composeService struct {
	Image     string            `yaml:"image"`
	Build     any               `yaml:"build"`
	Command   commandValue      `yaml:"command"`
	Ports     []string          `yaml:"ports"`
	Volumes   []string          `yaml:"volumes"`
	DependsOn dependsOnServices `yaml:"depends_on"`
}

type commandValue []string

func (c *commandValue) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		parts := make([]string, 0, len(value.Content))
		for _, node := range value.Content {
			parts = append(parts, node.Value)
		}
		*c = parts
		return nil
	}

	*c = []string{value.Value}
	return nil
}

type dependsOnServices map[string]dependsOnCondition

type dependsOnCondition struct {
	Condition string `yaml:"condition"`
}

func (d *dependsOnServices) UnmarshalYAML(value *yaml.Node) error {
	services := dependsOnServices{}
	switch value.Kind {
	case yaml.SequenceNode:
		for _, node := range value.Content {
			services[node.Value] = dependsOnCondition{}
		}
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			name := value.Content[i].Value
			var condition dependsOnCondition
			if err := value.Content[i+1].Decode(&condition); err != nil {
				return err
			}
			services[name] = condition
		}
	}
	*d = services
	return nil
}

type prodYAML struct {
	Data struct {
		DB struct {
			User struct {
				DSN string `yaml:"dsn"`
			} `yaml:"user"`
		} `yaml:"db"`
	} `yaml:"data"`
	Task struct {
		FetchIntervalMinutes int `yaml:"fetch_interval_minutes"`
	} `yaml:"task"`
	AI struct {
		APIBaseURL string `yaml:"api_base_url"`
		APIKey     string `yaml:"api_key"`
		Model      string `yaml:"model"`
	} `yaml:"ai"`
}

func TestComposeDefinesShiliuRuntimeServices(t *testing.T) {
	compose := readCompose(t)

	require.Contains(t, compose.Services, "migration")
	require.Contains(t, compose.Services, "server")
	require.Contains(t, compose.Services, "task")
	require.NotContains(t, compose.Services, "user-db")
	require.NotContains(t, compose.Services, "cache-redis")

	for name, service := range compose.Services {
		image := strings.ToLower(service.Image)
		require.NotContains(t, image, "mysql", "service %s must not use scaffold database images", name)
		require.NotContains(t, image, "redis", "service %s must not use scaffold cache images", name)
	}
}

func TestComposeUsesOneImageForAllProcessRoles(t *testing.T) {
	compose := readCompose(t)

	for _, serviceName := range []string{"migration", "server", "task"} {
		service := compose.Services[serviceName]
		require.Equal(t, imageName, service.Image, "%s should use the shared backend image", serviceName)
	}
	for _, serviceName := range []string{"server", "task"} {
		require.Empty(t, compose.Services[serviceName].Build, "%s should reuse the migration-built image instead of declaring a separate build", serviceName)
	}
	require.NotEmpty(t, compose.Services["migration"].Build, "one service should own the shared image build")
}

func TestComposeSharesSQLiteStorageVolume(t *testing.T) {
	compose := readCompose(t)

	require.Contains(t, compose.Volumes, storageVolume)
	require.Equal(t, storageVolume, compose.Volumes[storageVolume].Name)
	for _, serviceName := range []string{"migration", "server", "task"} {
		service := compose.Services[serviceName]
		require.Contains(t, service.Volumes, storageVolume+":"+storageTarget, "%s should mount shared SQLite storage", serviceName)
	}
}

func TestComposeRunsMigrationBeforeLongRunningServices(t *testing.T) {
	compose := readCompose(t)

	for _, serviceName := range []string{"server", "task"} {
		dependency, ok := compose.Services[serviceName].DependsOn["migration"]
		require.True(t, ok, "%s should depend on migration", serviceName)
		require.Equal(t, "service_completed_successfully", dependency.Condition, "%s should wait for successful migration completion", serviceName)
	}
}

func TestComposeCommandsAndPortsMatchProcessRoles(t *testing.T) {
	compose := readCompose(t)

	requireCommandContains(t, compose.Services["migration"].Command, "./bin/migration", "-conf", "config/prod.yml", "-direction", "up", "-path", "migrations")
	requireCommandContains(t, compose.Services["server"].Command, "./bin/server", "-conf", "config/prod.yml")
	requireCommandContains(t, compose.Services["task"].Command, "./bin/task", "-conf", "config/prod.yml")

	require.NotEmpty(t, compose.Services["server"].Ports, "server should publish HTTP")
	require.Empty(t, compose.Services["task"].Ports, "task should not publish HTTP")
	require.Empty(t, compose.Services["migration"].Ports, "migration should not publish HTTP")
}

func TestProdConfigContainsDeploymentKnobs(t *testing.T) {
	config := readProdConfig(t)

	require.Equal(t, 60, config.Task.FetchIntervalMinutes)
	require.Contains(t, []int{0, 30, 60, 360, 1440}, config.Task.FetchIntervalMinutes)
	require.Equal(t, "storage/shiliu.db?_busy_timeout=5000", config.Data.DB.User.DSN)
	require.NotEmpty(t, config.AI.APIBaseURL, "prod config should show the OpenAI-compatible API base URL key")
	require.Empty(t, config.AI.APIKey, "prod config must not hardcode a real AI API key")
	require.NotEmpty(t, config.AI.Model, "prod config should show the model key")
}

func TestDeploymentDocsExplainBackupAndTLSBoundaries(t *testing.T) {
	content := readText(t, "README.md") + "\n" + readOptionalText(t, "deploy/docker-compose/README.md")
	lower := strings.ToLower(content)

	require.Contains(t, lower, "docker compose")
	require.Contains(t, content, "migration")
	require.Contains(t, content, "server")
	require.Contains(t, content, "task")
	require.Contains(t, content, "SQLite")
	require.Contains(t, lower, "volume")
	require.Contains(t, lower, "backup")
	require.Contains(t, lower, "tls")
	require.Contains(t, lower, "reverse proxy")
}

func TestDockerfileBuildsAllRuntimeEntrypoints(t *testing.T) {
	content := readText(t, dockerfile)

	for _, commandPath := range []string{"./cmd/server", "./cmd/task", "./cmd/migration"} {
		require.Contains(t, content, commandPath)
	}
	for _, binaryPath := range []string{"./bin/server", "./bin/task", "./bin/migration"} {
		require.Contains(t, content, binaryPath)
	}
	require.Contains(t, content, "COPY --from=builder /data/app/bin")
	require.Contains(t, content, "COPY --from=builder /data/app/config")
	require.Contains(t, content, "COPY --from=builder /data/app/migrations")
}

func readCompose(t *testing.T) composeFile {
	t.Helper()

	var compose composeFile
	readYAML(t, composePath, &compose)
	return compose
}

func readProdConfig(t *testing.T) prodYAML {
	t.Helper()

	var config prodYAML
	readYAML(t, prodConfig, &config)
	return config
}

func readYAML(t *testing.T, path string, out any) {
	t.Helper()

	content := readText(t, path)
	require.NoError(t, yaml.Unmarshal([]byte(content), out))
}

func readText(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
	require.NoError(t, err)
	return string(content)
}

func readOptionalText(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(content)
}

func requireCommandContains(t *testing.T, command commandValue, want ...string) {
	t.Helper()

	joined := strings.Join(command, " ")
	for _, value := range want {
		require.Contains(t, joined, value)
	}
}
