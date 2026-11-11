package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	pb "github.com/criyle/go-judge/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// manifestDir is where the judge image mounts its manifest inside the sandbox (judge/mount.yaml).
const manifestDir = "/etc/judge"

const manifestFileLimit = 1 << 20

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Identity identifies the sandbox toolchain a problem build is compiled with.
type Identity struct {
	Digest       string `json:"digest"`
	Architecture string `json:"architecture"`
	ABI          string `json:"abi"`
}

// Image describes the running judge image: its toolchain identity and languages.
type Image struct {
	Identity Identity
	Config   *Config
}

// ReadImage reads the manifest the judge image was built with. It waits while
// go-judge is unreachable (still starting) until ctx ends; any other error, such
// as a missing or invalid manifest, fails at once.
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

// unavailable reports whether err means go-judge could not be reached.
func unavailable(err error) bool {
	var s interface{ GRPCStatus() *status.Status }
	return errors.As(err, &s) && s.GRPCStatus().Code() == codes.Unavailable
}

func readImage(ctx context.Context, client *Client) (Image, error) {
	data, err := client.readFile(ctx, manifestDir+"/identity.json")
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
	data, err = client.readFile(ctx, manifestDir+"/langs.toml")
	if err != nil {
		return Image{}, err
	}
	cfg, err := ParseConfig(data)
	if err != nil {
		return Image{}, err
	}
	return Image{Identity: identity, Config: cfg}, nil
}

// readFile returns a small file from the sandbox filesystem.
func (c *Client) readFile(ctx context.Context, path string) ([]byte, error) {
	cmd := pb.Request_CmdType_builder{
		Args: []string{"/bin/cat", path},
		Files: []*pb.Request_File{
			newMemoryFile(nil),
			newPipeCollector("stdout", manifestFileLimit),
			newPipeCollector("stderr", 4096),
		},
		CpuTimeLimit: safeUint64(time.Second.Nanoseconds()),
		MemoryLimit:  64 << 20,
		ProcLimit:    1,
		Env:          []string{"PATH=/usr/bin:/bin"},
	}.Build()
	resp, err := c.Exec(ctx, pb.Request_builder{Cmd: []*pb.Request_CmdType{cmd}}.Build())
	if err != nil {
		return nil, err
	}
	if len(resp.GetResults()) != 1 {
		return nil, fmt.Errorf("read %s: expected one result", path)
	}
	res := resp.GetResults()[0]
	if res.GetStatus() != pb.Response_Result_Accepted {
		return nil, fmt.Errorf("read %s: status=%v exit=%d stderr=%s", path, res.GetStatus(), res.GetExitStatus(), res.GetFiles()["stderr"])
	}
	return res.GetFiles()["stdout"], nil
}
