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

	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/packagegen"
	"github.com/gate149/gate/backend/pkg/parsers"
	"github.com/gate149/gate/backend/pkg/problemformat"
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

	manifest, testsMetadata, err := parsers.NewPolygonParser().Parse(tempDir)
	require.NoError(t, err)

	require.Equal(t, "scoring", manifest.ProblemType)
	require.NotNil(t, manifest.MaxScore)
	assert.Equal(t, 100, *manifest.MaxScore)
	assert.Equal(t, 2000, manifest.TimeLimitMs)
	assert.Equal(t, 256, manifest.MemoryLimitMb)
	assert.Equal(t, "APlusB", manifest.Statement.Title)

	require.Len(t, manifest.FilesMetadata, 1)
	assert.Equal(t, "checker", manifest.FilesMetadata[0].Type)
	assert.Equal(t, "cpp17", manifest.FilesMetadata[0].Compiler)

	require.Len(t, testsMetadata.Tests, 2)
	assert.True(t, testsMetadata.Tests[0].IsSample)
	assert.Equal(t, "generated", testsMetadata.Tests[1].Method)
	require.NotNil(t, testsMetadata.Tests[1].Generator)
	assert.Equal(t, "gen 10", *testsMetadata.Tests[1].Generator)

	require.Len(t, testsMetadata.Groups, 2)
	assert.Equal(t, "samples", testsMetadata.Groups[0].Name)
	assert.Equal(t, "main", testsMetadata.Groups[1].Name)
	assert.Equal(t, []int{0}, testsMetadata.Groups[1].DependsOn)
	assert.Equal(t, [2]int{2, 2}, testsMetadata.Groups[1].Tests)

	parsedFormat, err := parsers.DetectFormat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "polygon", parsedFormat)

	parser, err := parsers.GetParser(parsedFormat)
	require.NoError(t, err)
	assert.Equal(t, "polygon", parser.GetFormat())

	manifestFromAuto, testsFromAuto, autoFormat, err := parsers.ParsePackage(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "polygon", autoFormat)
	assert.Equal(t, manifest.ProblemType, manifestFromAuto.ProblemType)
	assert.Len(t, testsFromAuto.Tests, len(testsMetadata.Tests))

	unknownDir, err := os.MkdirTemp("", "unknown-format-*")
	require.NoError(t, err)
	defer os.RemoveAll(unknownDir)

	_, err = parsers.DetectFormat(unknownDir)
	assert.Error(t, err)

	_, err = parsers.GetParser("unsupported")
	assert.Error(t, err)

	_, _, _, err = parsers.ParsePackage(unknownDir)
	assert.Error(t, err)

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

	manifest, testsMetadata, err := parsers.NewICPCParser().Parse(tempDir)
	require.NoError(t, err)

	assert.Equal(t, "pass-fail", manifest.ProblemType)
	assert.Equal(t, 1500, manifest.TimeLimitMs)
	assert.Equal(t, 128, manifest.MemoryLimitMb)
	assert.Equal(t, 32, manifest.StdoutLimitMb)
	assert.Equal(t, 64, manifest.CodeSizeLimitKb)
	assert.Contains(t, manifest.Statement.Legend, "A + B")
	require.NotEmpty(t, manifest.FilesMetadata)
	assert.Equal(t, "checker", manifest.FilesMetadata[0].Type)

	require.Len(t, testsMetadata.Groups, 1)
	assert.Equal(t, "all", testsMetadata.Groups[0].Name)
	require.Len(t, testsMetadata.Tests, 2)

	sampleCount := 0
	for _, tc := range testsMetadata.Tests {
		if tc.IsSample {
			sampleCount++
		}
	}
	assert.Equal(t, 1, sampleCount)

	format, err := parsers.DetectFormat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "icpc", format)

	manifestAuto, testsAuto, detectedFormat, err := parsers.ParsePackage(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "icpc", detectedFormat)
	assert.Equal(t, manifest.TimeLimitMs, manifestAuto.TimeLimitMs)
	assert.Equal(t, len(testsMetadata.Tests), len(testsAuto.Tests))

}

func TestS3ClientOperations(t *testing.T) {
	ctx, s3Client := newMinioBackedS3Client(t)

	bucket := "test-bucket-" + uuid.New().String()

	err := s3Client.CreateBucket(ctx, bucket)
	require.NoError(t, err)

	testContent := []byte("Hello, S3!")
	testKey := "test-file.txt"
	err = s3Client.UploadFile(ctx, bucket, testKey, bytes.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	file, err := s3Client.DownloadFile(ctx, bucket, testKey, nil)
	require.NoError(t, err)
	defer file.Body.Close()

	downloadedContent, err := io.ReadAll(file.Body)
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

func TestPackageBuilder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "package-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	manifest := &problemformat.ProblemManifest{
		LastUpdated:     time.Now(),
		ProblemType:     "pass-fail",
		MaxScore:        nil,
		FilesMetadata:   []problemformat.FileMetadata{},
		TimeLimitMs:     1000,
		MemoryLimitMb:   256,
		StdoutLimitMb:   64,
		CodeSizeLimitKb: 256,
		Statement: problemformat.Statement{
			Title:        "Test Problem",
			Legend:       "This is a test problem",
			InputFormat:  "Input format",
			OutputFormat: "Output format",
		},
	}

	err = problemformat.SaveManifest(tempDir, manifest)
	require.NoError(t, err)

	testsMetadata := &problemformat.TestsMetadata{
		Groups: []problemformat.TestGroup{
			{
				Ordinal:      0,
				Name:         "all",
				Points:       0,
				PointsPolicy: "complete-group",
				DependsOn:    []int{},
				Tests:        [2]int{1, 2},
			},
		},
		Tests: []problemformat.TestCase{
			{Ordinal: 1, Method: "manual", Generator: nil, IsSample: true},
			{Ordinal: 2, Method: "manual", Generator: nil, IsSample: false},
		},
	}

	err = problemformat.SaveTestsMetadata(tempDir, testsMetadata)
	require.NoError(t, err)

	err = problemformat.SaveTestData(tempDir, 1, []byte("1 2\n"), []byte("3\n"))
	require.NoError(t, err)
	err = problemformat.SaveTestData(tempDir, 2, []byte("5 10\n"), []byte("15\n"))
	require.NoError(t, err)

	pkg, err := packagegen.BuildPackage(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, pkg)

	err = packagegen.ValidatePackage(pkg)
	require.NoError(t, err)

	var zipBuffer bytes.Buffer
	err = packagegen.WritePackageToZip(pkg, &zipBuffer)
	require.NoError(t, err)
	assert.Greater(t, zipBuffer.Len(), 0)
}

func newMinioBackedS3Client(t *testing.T) (context.Context, *pkg.S3Client) {
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

	return ctx, pkg.NewS3Client(pkg.S3Config{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
	})
}
