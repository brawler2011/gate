//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/require"
)

// Probe the actual server independently of our upload implementation: accepting
// the header without enforcing it would make publication's integrity check void.
func TestSeaweedUploadChecksumEnforcement(t *testing.T) {
	ctx := context.Background()
	store, bucket := newSeaweedBackedS3Storage(t)
	client := s3.NewFromConfig(aws.Config{
		Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("any", "any", ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(shared.s3Endpoint)
		o.UsePathStyle = true
	})
	const original = "validated archive bytes"
	digest := sha256.Sum256([]byte(original))
	for _, body := range []string{original, "different archive bytes"} {
		t.Run(body, func(t *testing.T) {
			_, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket), Key: aws.String(body), Body: strings.NewReader(body),
				ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(digest[:])),
			})
			if body != original {
				var apiErr smithy.APIError
				require.ErrorAs(t, err, &apiErr)
				require.Equal(t, "BadDigest", apiErr.ErrorCode())
				t.Logf("SeaweedFS rejected mismatched SHA-256: %s: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
				return
			}
			require.NoError(t, err)
			r, _, err := store.DownloadFile(ctx, bucket, body, nil)
			require.NoError(t, err)
			got, err := io.ReadAll(r)
			require.NoError(t, err)
			require.NoError(t, r.Close())
			require.Equal(t, original, string(got))
		})
	}
}
