//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/pkg/formats"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/formats/icpc"
	"github.com/gate149/gate/backend/pkg/formats/polygon"
	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	minioAPIPort       = "9000/tcp"
	minioStartupTimout = 60 * time.Second
)

func TestPolygonParser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "polygon-parser-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = os.MkdirAll(filepath.Join(tempDir, "statements"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "statements", "en.html"), []byte("<h1>A+B</h1>"), 0o644)
	require.NoError(t, err)

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<problem short-name="APlusB" revision="1">
  <judging>
    <testset name="tests">
      <time-limit>2000</time-limit>
      <memory-limit>268435456</memory-limit>
      <test-count>2</test-count>
      <input-path-pattern>tests/%02d.in</input-path-pattern>
      <answer-path-pattern>tests/%02d.ans</answer-path-pattern>
      <tests>
        <test method="manual" sample="true" />
        <test method="generated" cmd="gen 10" sample="false" />
      </tests>
      <groups>
        <group name="samples" points-policy="complete-group" points="0" />
        <group name="main" points-policy="each-test" points="100">
          <dependencies>
            <dependency group="samples" />
          </dependencies>
        </group>
      </groups>
    </testset>
  </judging>
  <files>
    <executables>
      <executable>
        <source path="checkers/checker.cpp" type="checker" />
        <binary path="checkers/checker" type="checker" />
      </executable>
    </executables>
  </files>
  <statements>
    <statement language="en" charset="utf-8" type="application/x-tex" path="statements/en.html" />
  </statements>
</problem>`

	err = os.WriteFile(filepath.Join(tempDir, "problem.xml"), []byte(xmlContent), 0o644)
	require.NoError(t, err)

	plan, err := polygon.NewParser().Parse(tempDir)
	require.NoError(t, err)

	require.Equal(t, "scoring", plan.Problem.Type)
	assert.Equal(t, 2000, plan.Problem.Limits.TimeMs)
	assert.Equal(t, 256, plan.Problem.Limits.MemoryMb)
	assert.Equal(t, "APlusB", plan.Problem.Title)

	require.NotEmpty(t, plan.Mappings)

	parsedFormat, err := formats.DetectFormat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "polygon", parsedFormat)

	parser, err := formats.GetParser(parsedFormat)
	require.NoError(t, err)
	assert.Equal(t, "polygon", parser.GetFormat())
}

func TestICPCParser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "icpc-parser-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = os.WriteFile(filepath.Join(tempDir, "problem.yaml"), []byte(`
name: "A + B"
author: "Gate"
source: "Integration"
limits:
  time: 1.5
  memory: 128
  output: 32
  code: 64
validation:
  type: "default"
`), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "statement", "en"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "statement", "en", "problem.md"), []byte("# A + B"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "data", "sample"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "sample", "sample1.in"), []byte("1 2\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "sample", "sample1.ans"), []byte("3\n"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "data", "secret"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "secret", "test1.in"), []byte("10 20\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "secret", "test1.ans"), []byte("30\n"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "output_validators"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "output_validators", "check.cpp"), []byte("int main(){}"), 0o644)
	require.NoError(t, err)

	plan, err := icpc.NewParser().Parse(tempDir)
	require.NoError(t, err)

	assert.Equal(t, "pass-fail", plan.Problem.Type)
	assert.Equal(t, 1500, plan.Problem.Limits.TimeMs)
	assert.Equal(t, 128, plan.Problem.Limits.MemoryMb)
	assert.Equal(t, "A + B", plan.Problem.Title)

	format, err := formats.DetectFormat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "icpc", format)
}

func TestS3ClientOperations(t *testing.T) {
	ctx, s3Client := newMinioBackedS3Client(t)

	bucket := "test-bucket-" + uuid.New().String()

	err := s3Client.EnsureBucket(ctx, bucket)
	require.NoError(t, err)

	testContent := []byte("Hello, S3!")
	testKey := "test-file.txt"
	err = s3Client.UploadFile(ctx, bucket, testKey, bytes.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	body, _, err := s3Client.DownloadFile(ctx, bucket, testKey, nil)
	require.NoError(t, err)
	defer body.Close()

	downloadedContent, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, testContent, downloadedContent)

	keys, err := s3Client.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.Contains(t, keys, testKey)

	url, err := s3Client.GetPresignedURL(ctx, bucket, testKey, 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, url)

	response, err := http.Get(url)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	err = s3Client.DeleteFile(ctx, bucket, testKey)
	require.NoError(t, err)

	keys, err = s3Client.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.NotContains(t, keys, testKey)
}

func TestLocalStorageOperations(t *testing.T) {
	ctx := context.Background()

	baseDir, err := os.MkdirTemp("", "local-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	localStorage := storage.NewLocalStorage(baseDir)

	bucket := "test-bucket"

	err = localStorage.EnsureBucket(ctx, bucket)
	require.NoError(t, err)

	testContent := []byte("Hello, Local FS!")
	testKey := "test-file.txt"
	err = localStorage.UploadFile(ctx, bucket, testKey, bytes.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	body, etag, err := localStorage.DownloadFile(ctx, bucket, testKey, nil)
	require.NoError(t, err)
	defer body.Close()

	downloadedContent, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, testContent, downloadedContent)
	assert.NotEmpty(t, etag)

	// Test ErrNotModified
	_, _, err = localStorage.DownloadFile(ctx, bucket, testKey, &etag)
	assert.ErrorIs(t, err, storage.ErrNotModified)

	keys, err := localStorage.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.Contains(t, keys, testKey)

	url, err := localStorage.GetPresignedURL(ctx, bucket, testKey, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "/api/files/test-bucket/test-file.txt", url)

	err = localStorage.DeleteFile(ctx, bucket, testKey)
	require.NoError(t, err)

	keys, err = localStorage.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.NotContains(t, keys, testKey)
}

func TestPackageBuilder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "package-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Write standard problem.yaml
	probYaml := `format_version: "1.0"
title: "Test Problem"
type: "pass-fail"
limits:
  time_ms: 1000
  memory_mb: 256
subtasks:
  samples:
    points: 0
    policy: "complete"
    tests:
      - manual: "01.in"
  secret:
    points: 100
    policy: "each"
    tests:
      - manual: "02.in"
`
	err = os.WriteFile(filepath.Join(tempDir, "problem.yaml"), []byte(probYaml), 0644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "tests"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "tests", "01.in"), []byte("1 2\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "tests", "01.out"), []byte("3\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "tests", "02.in"), []byte("5 10\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "tests", "02.out"), []byte("15\n"), 0644)
	require.NoError(t, err)

	// Validate the package layout by loading it
	pkg, err := gfmt.OpenPackage(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.Equal(t, "Test Problem", pkg.Problem.Title)

	// Compress
	var zipBuffer bytes.Buffer
	err = usecase.ZipDirectory(tempDir, &zipBuffer)
	require.NoError(t, err)
	assert.Greater(t, zipBuffer.Len(), 0)
}

func newMinioBackedS3Client(t *testing.T) (context.Context, storage.Storage) {
	t.Helper()

	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{minioAPIPort},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd: []string{"server", "/data", "--address", ":9000"},
			WaitingFor: wait.ForHTTP("/minio/health/ready").
				WithPort(minioAPIPort).
				WithStartupTimeout(minioStartupTimout),
		},
		Started: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, minioAPIPort)
	require.NoError(t, err)

	endpoint := "http://" + host + ":" + port.Port()

	return ctx, storage.NewS3Storage(storage.S3Config{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
	})
}
