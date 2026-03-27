//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
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
)

// TestPolygonParser tests the Polygon XML parser
func TestPolygonParser(t *testing.T) {
	t.Skip("Requires sample Polygon package - implement when test data is available")

	// This test would:
	// 1. Create a sample Polygon package with problem.xml
	// 2. Parse it using PolygonParser
	// 3. Validate the resulting manifest and tests metadata
}

// TestICPCParser tests the ICPC YAML parser
func TestICPCParser(t *testing.T) {
	t.Skip("Requires sample ICPC package - implement when test data is available")

	// This test would:
	// 1. Create a sample ICPC package with problem.yaml
	// 2. Parse it using ICPCParser
	// 3. Validate the resulting manifest and tests metadata
}

// TestFormatDetection tests automatic format detection
func TestFormatDetection(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		expectedFormat string
		expectError    bool
	}{
		{
			name: "detect polygon format",
			files: map[string]string{
				"problem.xml": "<problem></problem>",
			},
			expectedFormat: "polygon",
			expectError:    false,
		},
		{
			name: "detect icpc format",
			files: map[string]string{
				"problem.yaml": "name: Test Problem",
			},
			expectedFormat: "icpc",
			expectError:    false,
		},
		{
			name:           "unknown format",
			files:          map[string]string{},
			expectedFormat: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "format-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Create test files
			for filename, content := range tt.files {
				err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
				require.NoError(t, err)
			}

			// Detect format
			format, err := parsers.DetectFormat(tempDir)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedFormat, format)
			}
		})
	}
}

// TestS3ClientOperations tests S3 client basic operations
func TestS3ClientOperations(t *testing.T) {
	t.Skip("Requires SeaweedFS instance - run manually with test environment")

	// This test would require a running SeaweedFS instance
	// To run manually:
	// 1. Start SeaweedFS: docker run -p 8333:8333 chrislusf/seaweedfs server -s3
	// 2. Set environment variables: S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY
	// 3. Remove t.Skip() and run test

	ctx := context.Background()

	s3Client := pkg.NewS3Client(pkg.S3Config{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Region:    "us-east-1",
	})

	bucket := "test-bucket-" + uuid.New().String()

	// Test: Create bucket
	err := s3Client.CreateBucket(ctx, bucket)
	require.NoError(t, err)

	// Test: Upload file
	testContent := []byte("Hello, S3!")
	testKey := "test-file.txt"
	err = s3Client.UploadFile(ctx, bucket, testKey, bytes.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	// Test: Download file
	file, err := s3Client.DownloadFile(ctx, bucket, testKey, nil)
	require.NoError(t, err)
	reader := file.Body
	defer reader.Close()

	downloadedContent := make([]byte, len(testContent))
	_, err = reader.Read(downloadedContent)
	require.NoError(t, err)
	assert.Equal(t, testContent, downloadedContent)

	// Test: List files
	keys, err := s3Client.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.Contains(t, keys, testKey)

	// Test: Get presigned URL
	url, err := s3Client.GetPresignedURL(ctx, bucket, testKey, 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, url)

	// Test: Delete file
	err = s3Client.DeleteFile(ctx, bucket, testKey)
	require.NoError(t, err)

	// Verify deletion
	keys, err = s3Client.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.NotContains(t, keys, testKey)
}

// TestPackageBuilder tests building a package from a problem directory
func TestPackageBuilder(t *testing.T) {
	// Create temporary problem directory
	tempDir, err := os.MkdirTemp("", "package-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create sample manifest
	manifest := &problemformat.ProblemManifest{
		LastUpdated:     time.Now(),
		ProblemType:     "pass-fail",
		MaxScore:        nil,
		FilesMetadata:   []problemformat.FileMetadata{},
		TimeLimitMs:     1000,
		MemoryLimitMb:   256,
		StdoutLimitMb:   64,
		CodeSizeLimitKb: 256,
		Statements: map[string]problemformat.Statement{
			"en": {
				Title:        "Test Problem",
				Legend:       "This is a test problem",
				InputFormat:  "Input format",
				OutputFormat: "Output format",
			},
		},
	}

	// Save manifest
	err = problemformat.SaveManifest(tempDir, manifest)
	require.NoError(t, err)

	// Create sample tests metadata
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

	// Save tests metadata
	err = problemformat.SaveTestsMetadata(tempDir, testsMetadata)
	require.NoError(t, err)

	// Create sample test files
	err = problemformat.SaveTestData(tempDir, 1, []byte("1 2\n"), []byte("3\n"))
	require.NoError(t, err)
	err = problemformat.SaveTestData(tempDir, 2, []byte("5 10\n"), []byte("15\n"))
	require.NoError(t, err)

	// Build package
	pkg, err := packagegen.BuildPackage(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, pkg)

	// Validate package
	err = packagegen.ValidatePackage(pkg)
	require.NoError(t, err)

	// Test ZIP creation
	var zipBuffer bytes.Buffer
	err = packagegen.WritePackageToZip(pkg, &zipBuffer)
	require.NoError(t, err)
	assert.Greater(t, zipBuffer.Len(), 0)
}

// TestEndToEndImportAndPublish tests the complete workflow
func TestEndToEndImportAndPublish(t *testing.T) {
	t.Skip("Requires full integration environment - run manually")

	// This test would:
	// 1. Create a sample problem package (ZIP)
	// 2. Import it using ProblemImportUseCase
	// 3. Publish it using ProblemPublishUseCase
	// 4. Download the published package from S3
	// 5. Verify the package structure
}

// TestAvatarUploadToS3 tests avatar upload workflow
func TestAvatarUploadToS3(t *testing.T) {
	t.Skip("Requires SeaweedFS instance - run manually with test environment")

	// This test would:
	// 1. Upload a sample avatar image to S3
	// 2. Generate a presigned URL
	// 3. Verify the URL is accessible
	// 4. Delete the avatar
	// 5. Verify deletion
}
