package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"time"
)

// manifestDir is where the judge image mounts its manifest inside the sandbox (judge/mount.yaml).
const manifestDir = "/etc/judge"

const manifestFileLimit = 1 << 20

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Identity struct {
	Digest       string `json:"digest"`
	Architecture string `json:"architecture"`
	ABI          string `json:"abi"`
}

type Image struct {
	Identity Identity
	Config   *Config
}

// Compiler includes language configuration because compiler flags can change without a new image digest.
func (i Image) Compiler() string {
	langs, _ := json.Marshal(i.Config.Langs) // string fields only; map keys are sorted
	sum := sha256.Sum256(append([]byte(i.Identity.Digest+"\n"), langs...))
	return hex.EncodeToString(sum[:])
}

// ReadImage retries unreachable go-judge until ctx ends; invalid or missing manifests fail immediately.
func ReadImage(ctx context.Context, client *Client) (Image, error) {
	for {
		image, err := readImage(ctx, client)
		if err == nil {
			return image, nil
		}
		if !unavailable(err) {
			return Image{}, fmt.Errorf("read judge image manifest: %w", err)
		}
		slog.Warn("go-judge is not reachable yet; retrying", "error", err)
		select {
		case <-ctx.Done():
			return Image{}, fmt.Errorf("read judge image manifest: go-judge unreachable: %w", err)
		case <-time.After(time.Second):
		}
	}
}

func readImage(ctx context.Context, client *Client) (Image, error) {
	return parseImage(func(name string) ([]byte, error) { return client.readFile(ctx, manifestDir+"/"+name) })
}

func parseImage(read func(name string) ([]byte, error)) (Image, error) {
	data, err := read("identity.json")
	if err != nil {
		return Image{}, err
	}
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return Image{}, fmt.Errorf("decode identity.json: %w", err)
	}
	if !digestPattern.MatchString(identity.Digest) || identity.Architecture == "" || identity.ABI == "" {
		return Image{}, fmt.Errorf("invalid identity.json: %s", data)
	}
	data, err = read("langs.toml")
	if err != nil {
		return Image{}, err
	}
	cfg, err := ParseConfig(data)
	if err != nil {
		return Image{}, err
	}
	return Image{Identity: identity, Config: cfg}, nil
}

func (c *Client) readFile(ctx context.Context, path string) ([]byte, error) {
	results, err := c.run(ctx, request{Cmd: []cmd{{
		Args:        []string{"/bin/cat", path},
		Files:       []*cmdFile{memoryFile(""), collector("stdout", manifestFileLimit, false), collector("stderr", 4096, false)},
		CPULimit:    safeUint64(time.Second.Nanoseconds()),
		MemoryLimit: 64 << 20,
		ProcLimit:   1,
		Env:         []string{"PATH=/usr/bin:/bin"},
	}}})
	if err != nil {
		return nil, err
	}
	res := results[0]
	if res.Status != resultAccepted {
		return nil, fmt.Errorf("read %s: status=%v exit=%d stderr=%s", path, res.Status, res.ExitStatus, res.Files["stderr"])
	}
	return []byte(res.Files["stdout"]), nil
}
