package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/cpus"
	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP     HTTP          `yaml:"http"`
	GoJudge  GoJudge       `yaml:"go_judge"`
	Storage  Storage       `yaml:"storage"`
	Email    Email         `yaml:"email"`
	Judge    Judge         `yaml:"judge"`
	Problems ProblemPolicy `yaml:"problems"`

	Secrets Secrets `yaml:"-"`
}

type HTTP struct {
	Address string `yaml:"address"`
	// Empty trusts only directly connected clients.
	TrustedProxy  string `yaml:"trusted_proxy"`
	SecureCookies bool   `yaml:"secure_cookies"`
}

type GoJudge struct {
	HTTPURL string `yaml:"http_url"`
	// Where go-judge sees problems.work_dir; it reads test files from there by path.
	WorkDir string `yaml:"work_dir"`
	// Programs go-judge runs at once, and one build's concurrent steps; 0 means the physical cores available.
	// Strict mode requires 0: judge.cpus decides.
	Parallelism *int `yaml:"parallelism"`
}

// Parallelism is how many programs go-judge runs at once.
func (c Config) Parallelism() (int, error) {
	if c.Judge.Mode == JudgeStrict {
		cores, err := c.Judge.Cores()
		return len(cores), err
	}
	if *c.GoJudge.Parallelism > 0 {
		return *c.GoJudge.Parallelism, nil
	}
	n, err := cpus.Physical()
	if err != nil {
		return 0, fmt.Errorf("detect go_judge.parallelism: %w", err)
	}
	return n, nil
}

// GoJudgeArgs are the go-judge flags the worker's settings imply.
func (c Config) GoJudgeArgs() ([]string, error) {
	n, err := c.Parallelism()
	if err != nil {
		return nil, err
	}
	args := []string{"-parallelism=" + strconv.Itoa(n)}
	if c.Judge.Mode == JudgeStrict {
		cores, err := c.Judge.Cores()
		if err != nil {
			return nil, err
		}
		// One go-judge run loop per listed cpuset, so list single cores.
		list := make([]string, len(cores))
		for i, core := range cores {
			list[i] = strconv.Itoa(core)
		}
		args = append(args, "-cpuset="+strings.Join(list, ","))
	}
	return args, nil
}

type Storage struct {
	S3Endpoint string `yaml:"s3_endpoint"`
}

const (
	EmailProviderResend = "resend"
	EmailProviderLog    = "log"
)

type Email struct {
	Provider   string `yaml:"provider"`
	From       string `yaml:"from"`
	AppBaseURL string `yaml:"app_base_url"`
}

type JudgeMode string

const (
	// JudgeDirty shares the CPU with everything else and re-runs a TLE before it stands.
	JudgeDirty JudgeMode = "dirty"
	// JudgeStrict judges only on verified dedicated cores (deploy/strict-judging.md).
	JudgeStrict JudgeMode = "strict"
)

type Judge struct {
	Mode JudgeMode `yaml:"mode"`
	// Strict mode: the kernel CPU list go-judge runs programs on, one at a time per core.
	CPUs string `yaml:"cpus"`
	// Strict mode: the calibration run at startup.
	Benchmark calibration.Settings `yaml:"benchmark"`
	Workers   int                  `yaml:"workers"`
	// API instances read judge languages here because they have no sandbox.
	ManifestDir string `yaml:"manifest_dir"`
}

func (j Judge) Cores() ([]int, error) {
	cores, err := cpus.ParseList(j.CPUs)
	if err != nil {
		return nil, fmt.Errorf("judge.cpus: %w", err)
	}
	return cores, nil
}

type Mode string

const (
	ModeAll    Mode = "all"
	ModeAPI    Mode = "api"
	ModeWorker Mode = "worker"
)

func ParseMode(s string) (Mode, error) {
	switch m := Mode(s); m {
	case ModeAll, ModeAPI, ModeWorker:
		return m, nil
	default:
		return "", fmt.Errorf("unknown mode %q: want %q, %q or %q", s, ModeAll, ModeAPI, ModeWorker)
	}
}

func (m Mode) API() bool { return m != ModeWorker }

func (m Mode) Worker() bool { return m != ModeAPI }

// Secrets come only from the environment.
type Secrets struct {
	PostgresDSN  string `env:"POSTGRES_DSN"`
	S3AccessKey  string `env:"S3_ACCESS_KEY"`
	S3SecretKey  string `env:"S3_SECRET_KEY"`
	ResendAPIKey string `env:"RESEND_API_KEY"`
}

// Read decodes a config file without secrets or validation, so tests can share the deploy configs.
func Read(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config %s: %w", path, err)
	}
	cfg.Problems.Limits.DeriveWalls()
	return cfg, nil
}

func Load(path string, mode Mode) (Config, error) {
	cfg, err := Read(path)
	if err != nil {
		return cfg, err
	}
	if err := cleanenv.ReadEnv(&cfg.Secrets); err != nil {
		return cfg, fmt.Errorf("read secrets: %w", err)
	}
	if err := cfg.Validate(mode); err != nil {
		return cfg, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// Settings unused by the selected mode are not validated.
func (c Config) Validate(mode Mode) error {
	var v validator
	v.required("http.address", c.HTTP.Address)
	if mode.Worker() {
		v.url("go_judge.http_url", c.GoJudge.HTTPURL)
		v.check(filepath.IsAbs(c.GoJudge.WorkDir), "go_judge.work_dir", "must be an absolute path")
		v.check(c.Judge.Workers >= 1, "judge.workers", "must be at least 1")
		v.check(c.GoJudge.Parallelism != nil && *c.GoJudge.Parallelism >= 0, "go_judge.parallelism", "must be set, 0 to detect physical cores")
		cores, err := c.Judge.Cores()
		switch c.Judge.Mode {
		case JudgeDirty:
			v.check(c.Judge.CPUs == "", "judge.cpus", "is only for strict mode")
			b := c.Judge.Benchmark
			v.check(b.Repeats == 0 && b.MaxSpread == 0 && b.MaxDisagreement == 0 && b.Reference == nil, "judge.benchmark", "is only for strict mode")
		case JudgeStrict:
			v.check(err == nil && len(cores) > 0, "judge.cpus", "must be a non-empty CPU list")
			v.check(c.GoJudge.Parallelism == nil || *c.GoJudge.Parallelism == 0, "go_judge.parallelism", "must be 0 in strict mode, judge.cpus decides")
			b := c.Judge.Benchmark
			v.check(b.Repeats >= 3, "judge.benchmark.repeats", "must be at least 3")
			v.check(b.MaxSpread > 0 && b.MaxSpread < 1, "judge.benchmark.max_spread", "must be between 0 and 1")
			v.check(b.MaxDisagreement > 0 && b.MaxDisagreement < 1, "judge.benchmark.max_disagreement", "must be between 0 and 1")
			for profile, d := range b.Reference {
				v.check(slices.Contains(calibration.Profiles, profile) && d > 0, "judge.benchmark.reference", fmt.Sprintf("has unknown profile or non-positive time %q", profile))
			}
		default:
			v.fail("judge.mode", "must be %q or %q", JudgeDirty, JudgeStrict)
		}
	}
	if mode == ModeAPI {
		v.required("judge.manifest_dir", c.Judge.ManifestDir)
	}
	v.url("storage.s3_endpoint", c.Storage.S3Endpoint)
	switch c.Email.Provider {
	case EmailProviderResend:
		v.required("email.from", c.Email.From)
		v.required("RESEND_API_KEY", c.Secrets.ResendAPIKey)
	case EmailProviderLog:
	default:
		v.fail("email.provider", "must be %q or %q", EmailProviderResend, EmailProviderLog)
	}
	v.url("email.app_base_url", c.Email.AppBaseURL)
	c.Problems.validate(&v)

	v.required("POSTGRES_DSN", c.Secrets.PostgresDSN)
	v.required("S3_ACCESS_KEY", c.Secrets.S3AccessKey)
	v.required("S3_SECRET_KEY", c.Secrets.S3SecretKey)
	return errors.Join(v.errs...)
}

type validator struct{ errs []error }

func (v *validator) fail(field, format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf("%s: %s", field, fmt.Sprintf(format, args...)))
}

func (v *validator) check(ok bool, field, message string) {
	if !ok {
		v.fail(field, "%s", message)
	}
}

func (v *validator) required(field, value string) {
	v.check(value != "", field, "is required")
}

func (v *validator) url(field, value string) {
	if value == "" {
		v.fail(field, "is required")
		return
	}
	u, err := url.Parse(value)
	v.check(err == nil && u.Scheme != "" && u.Host != "", field, "must be an absolute URL")
}
