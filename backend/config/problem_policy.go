package config

import (
	"fmt"
	"os"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"gopkg.in/yaml.v3"
)

type ProblemPolicy struct {
	Draft            model.DraftQuotas `yaml:"draft"`
	Limits           model.BuildLimits `yaml:"limits"`
	Parallelism      int               `yaml:"parallelism"`
	Attempts         int               `yaml:"attempts"`
	Lease            time.Duration     `yaml:"lease"`
	Poll             time.Duration     `yaml:"poll"`
	Duration         time.Duration     `yaml:"duration"`
	OperationLease   time.Duration     `yaml:"operation_lease"`
	UploadTTL        time.Duration     `yaml:"upload_ttl"`
	GCInterval       time.Duration     `yaml:"gc_interval"`
	ObjectRetention  time.Duration     `yaml:"object_retention"`
	BuildRetention   time.Duration     `yaml:"build_retention"`
	PackageRetention time.Duration     `yaml:"package_retention"`
	RequestBytes     bytesize.Size     `yaml:"request_bytes"`
}

func LoadProblemPolicy(path string) (ProblemPolicy, error) {
	f, err := os.Open(path)
	if err != nil {
		return ProblemPolicy{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	var p ProblemPolicy
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&p); err != nil {
		return p, fmt.Errorf("decode problem policy: %w", err)
	}
	// FIXME: вынести в функцию что ли??
	if p.Parallelism < 1 || p.Attempts < 1 || p.Lease < 3*time.Second || p.Poll <= 0 || p.Duration <= p.Lease || p.OperationLease <= p.UploadTTL || p.UploadTTL <= 0 || p.GCInterval <= 0 || p.ObjectRetention <= 0 || p.BuildRetention <= 0 || p.PackageRetention <= 0 || p.RequestBytes < p.Draft.TextBytes || p.Limits.DiskBytes <= 0 || p.Limits.ObjectBytes <= 0 || p.Limits.ObjectBytes > 5*bytesize.GiB || p.Limits.FileCount <= 0 || p.Draft.FileBytes <= 0 || p.Draft.TextBytes <= 0 || p.Draft.Files <= 0 || p.Draft.Tests <= 0 || p.Draft.Components <= 0 || p.Draft.Subtasks <= 0 {
		return p, fmt.Errorf("invalid problem policy budgets or retention")
	}
	for _, l := range []struct {
		cpu, wall                  time.Duration
		memory, output, diagnostic bytesize.Size
		processes                  uint64
	}{
		{p.Limits.Compile.CPU, p.Limits.Compile.Wall, p.Limits.Compile.Memory, p.Limits.Compile.Output, p.Limits.Compile.Diagnostic, p.Limits.Compile.Processes},
		{p.Limits.Auxiliary.CPU, p.Limits.Auxiliary.Wall, p.Limits.Auxiliary.Memory, p.Limits.Auxiliary.Output, p.Limits.Auxiliary.Diagnostic, p.Limits.Auxiliary.Processes},
	} {
		if l.cpu <= 0 || l.wall <= 0 || l.memory <= 0 || l.processes == 0 || l.output <= 0 || l.diagnostic <= 0 || l.diagnostic > 64*bytesize.KiB {
			return p, fmt.Errorf("invalid problem execution limits")
		}
	}
	return p, nil
}
