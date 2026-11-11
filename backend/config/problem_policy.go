package config

import (
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

// ProblemPolicy holds the budgets, limits and retention of problem drafts, builds and packages.
type ProblemPolicy struct {
	// WorkDir is scratch space for builds and judging; it must be on a real disk, not tmpfs.
	WorkDir          string            `yaml:"work_dir"`
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

func (p ProblemPolicy) validate(v *validator) {
	v.required("problems.work_dir", p.WorkDir)

	v.check(p.Draft.FileBytes > 0, "problems.draft.file_bytes", "must be positive")
	v.check(p.Draft.TextBytes > 0, "problems.draft.text_bytes", "must be positive")
	v.check(p.Draft.Files > 0, "problems.draft.files", "must be positive")
	v.check(p.Draft.Tests > 0, "problems.draft.tests", "must be positive")
	v.check(p.Draft.Components > 0, "problems.draft.components", "must be positive")
	v.check(p.Draft.Subtasks > 0, "problems.draft.subtasks", "must be positive")

	v.check(p.Limits.DiskBytes > 0, "problems.limits.disk_bytes", "must be positive")
	v.check(p.Limits.FileCount > 0, "problems.limits.file_count", "must be positive")
	v.check(p.Limits.ObjectBytes > 0 && p.Limits.ObjectBytes <= 5*bytesize.GiB, "problems.limits.object_bytes", "must be in (0, 5GiB]")
	validateRunLimits(v, "problems.limits.compile", p.Limits.Compile)
	validateRunLimits(v, "problems.limits.auxiliary", p.Limits.Auxiliary)

	v.check(p.Parallelism >= 1, "problems.parallelism", "must be at least 1")
	v.check(p.Attempts >= 1, "problems.attempts", "must be at least 1")
	v.check(p.Lease >= 3*time.Second, "problems.lease", "must be at least 3s")
	v.check(p.Poll > 0, "problems.poll", "must be positive")
	v.check(p.Duration > p.Lease, "problems.duration", "must exceed problems.lease")
	v.check(p.UploadTTL > 0, "problems.upload_ttl", "must be positive")
	v.check(p.OperationLease > p.UploadTTL, "problems.operation_lease", "must exceed problems.upload_ttl")
	v.check(p.GCInterval > 0, "problems.gc_interval", "must be positive")
	v.check(p.ObjectRetention > 0, "problems.object_retention", "must be positive")
	v.check(p.BuildRetention > 0, "problems.build_retention", "must be positive")
	v.check(p.PackageRetention > 0, "problems.package_retention", "must be positive")
	v.check(p.RequestBytes >= p.Draft.TextBytes, "problems.request_bytes", "must be at least problems.draft.text_bytes")
}

func validateRunLimits(v *validator, field string, l sandbox.FileRunLimits) {
	v.check(l.CPU > 0, field+".cpu", "must be positive")
	v.check(l.Wall > 0, field+".wall", "must be positive")
	v.check(l.Memory > 0, field+".memory", "must be positive")
	v.check(l.Processes > 0, field+".processes", "must be positive")
	v.check(l.Output > 0, field+".output", "must be positive")
	v.check(l.Diagnostic > 0 && l.Diagnostic <= 64*bytesize.KiB, field+".diagnostic", "must be in (0, 64KiB]")
}
