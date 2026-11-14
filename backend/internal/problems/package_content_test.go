package problems

import (
	"context"
	"io"
	"strings"
	"testing"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type packageAssets struct {
	assets []model.PackageAsset
}

func (packageAssets) GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error) {
	return model.PackageDescriptor{}, pkg.ErrNotFound
}
func (packageAssets) GetPackageStatement(context.Context, uuid.UUID) (model.PackageStatement, error) {
	return model.PackageStatement{}, pkg.ErrNotFound
}
func (r packageAssets) FindPackageAssets(context.Context, uuid.UUID, string) ([]model.PackageAsset, error) {
	return r.assets, nil
}

type assetStore struct{ keys []string }

func (s *assetStore) UploadFile(context.Context, string, string, io.Reader, string) error {
	return nil
}
func (s *assetStore) DownloadFile(_ context.Context, _, key string, _ *string) (io.ReadCloser, string, error) {
	s.keys = append(s.keys, key)
	return io.NopCloser(strings.NewReader("image")), "", nil
}

func TestPackageContentServesOnlyImages(t *testing.T) {
	for _, path := range []string{"assets/notes.txt", "assets/page.html", "assets/noext"} {
		store := &assetStore{}
		content := &PackageContent{Repo: packageAssets{assets: []model.PackageAsset{{Path: path, Key: "k"}}}, Store: store}
		_, _, err := content.OpenImage(context.Background(), uuid.New(), "hash")
		require.ErrorIs(t, err, pkg.ErrNotFound, path)
		require.Empty(t, store.keys, path)
	}
	store := &assetStore{}
	content := &PackageContent{Repo: packageAssets{assets: []model.PackageAsset{{Path: "assets/a.txt", Key: "text"}, {Path: "assets/Diagram.SVG", Key: "svg"}}}, Store: store}
	file, contentType, err := content.OpenImage(context.Background(), uuid.New(), "hash")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.Equal(t, "image/svg+xml", contentType)
	require.Equal(t, []string{"svg"}, store.keys)
}
