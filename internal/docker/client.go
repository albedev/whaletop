// Package docker wraps the official Docker Engine Go SDK (github.com/moby/moby/client)
// and adds what the SDK lacks: docker CLI context resolution.
package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/client"
)

// Client embeds the SDK client and remembers how it was reached.
type Client struct {
	*client.Client
	Endpoint string // e.g. unix:///Users/me/.docker/run/docker.sock
	Local    bool   // reachable through a local unix socket: host-side disk probing is meaningful
}

// New connects to endpoint, or resolves one like the docker CLI does when empty.
func New(endpoint string) (*Client, error) {
	if endpoint == "" {
		endpoint = ResolveEndpoint()
	}
	c, err := client.New(client.WithHost(endpoint), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, Endpoint: endpoint, Local: strings.HasPrefix(endpoint, "unix://")}, nil
}

// ResolveEndpoint mimics the docker CLI resolution order:
// DOCKER_HOST > DOCKER_CONTEXT > currentContext in ~/.docker/config.json > well-known sockets.
// The SDK only honours DOCKER_HOST; importing github.com/docker/cli for contexts
// would drag in a huge dependency tree for ~30 lines of logic.
func ResolveEndpoint() string {
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		return h
	}
	home, _ := os.UserHomeDir()
	cfgDir := os.Getenv("DOCKER_CONFIG")
	if cfgDir == "" {
		cfgDir = filepath.Join(home, ".docker")
	}
	ctxName := os.Getenv("DOCKER_CONTEXT")
	if ctxName == "" {
		var cfg struct {
			CurrentContext string `json:"currentContext"`
		}
		if b, err := os.ReadFile(filepath.Join(cfgDir, "config.json")); err == nil {
			_ = json.Unmarshal(b, &cfg)
			ctxName = cfg.CurrentContext
		}
	}
	if ctxName != "" && ctxName != "default" {
		// Context metadata lives in contexts/meta/<sha256(name)>/meta.json.
		sum := sha256.Sum256([]byte(ctxName))
		var meta struct {
			Endpoints map[string]struct {
				Host string `json:"Host"`
			} `json:"Endpoints"`
		}
		p := filepath.Join(cfgDir, "contexts", "meta", hex.EncodeToString(sum[:]), "meta.json")
		if b, err := os.ReadFile(p); err == nil && json.Unmarshal(b, &meta) == nil {
			if ep := meta.Endpoints["docker"].Host; ep != "" {
				return ep
			}
		}
	}
	for _, c := range []string{
		"/var/run/docker.sock",
		filepath.Join(home, ".docker", "run", "docker.sock"),
		filepath.Join(home, ".orbstack", "run", "docker.sock"),
		filepath.Join(home, ".colima", "default", "docker.sock"),
		filepath.Join(home, ".rd", "docker.sock"),
	} {
		if fi, err := os.Stat(c); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return "unix://" + c
		}
	}
	return client.DefaultDockerHost
}

// ContainerName returns the primary name without the leading slash.
func ContainerName(names []string, id string) string {
	if len(names) == 0 {
		return ShortID(id)
	}
	return strings.TrimPrefix(names[0], "/")
}

// ShortID trims a docker id (optionally "sha256:" prefixed) to 12 chars.
func ShortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
