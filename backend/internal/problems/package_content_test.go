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

func (s *assetStore) UploadFileWithSHA256(context.Context, string, string, io.Reader, string, string) error {
	return nil
}
func (s *assetStore) DownloadFile(_ context.Context, _, key string, _ *string) (io.ReadCloser, string, error) {
	s.keys = append(s.keys, key)
	return io.NopCloser(strings.NewReader("image")), "", nil
}

func TestPackageContentServesOnlyImages(t *testing.T) {
	for _, path := range []string{"notes.txt", "page.html", "noext"} {
		store := &assetStore{}
		content := &PackageContent{Repo: packageAssets{assets: []model.PackageAsset{{Name: path, Key: "k"}}}, Store: store}
		_, _, err := content.OpenImage(context.Background(), uuid.New(), "hash")
		require.ErrorIs(t, err, pkg.ErrNotFound, path)
		require.Empty(t, store.keys, path)
	}
	store := &assetStore{}
	content := &PackageContent{Repo: packageAssets{assets: []model.PackageAsset{{Name: "a.txt", Key: "text"}, {Name: "Diagram.SVG", Key: "svg"}}}, Store: store}
	file, contentType, err := content.OpenImage(context.Background(), uuid.New(), "hash")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.Equal(t, "image/svg+xml", contentType)
	require.Equal(t, []string{"svg"}, store.keys)
}

type storedStatement struct{ p model.PackageStatement }

func (s storedStatement) GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error) {
	return model.PackageDescriptor{}, pkg.ErrNotFound
}
func (s storedStatement) GetPackageStatement(context.Context, uuid.UUID) (model.PackageStatement, error) {
	return s.p, nil
}
func (storedStatement) FindPackageAssets(context.Context, uuid.UUID, string) ([]model.PackageAsset, error) {
	return nil, nil
}

func TestReadStatementShowsItsTitle(t *testing.T) {
	problemID := uuid.New()
	statements := []model.DraftStatement{{Language: "en"}, {Language: "ru", Title: "Сумма"}}
	content := &PackageContent{Repo: storedStatement{p: model.PackageStatement{ProblemID: problemID, Title: "Sum", Statements: statements}}}
	title := func(language string) string {
		return content.ReadStatement(context.Background(), problemID, uuid.New(), language).Statement.Title
	}
	require.Equal(t, "Сумма", title("ru"))
	require.Equal(t, "Sum", title("en"))
}

func TestReadStatementFallsBackToEnglishThenFirstLanguage(t *testing.T) {
	problemID := uuid.New()
	read := func(language string, statements ...model.DraftStatement) string {
		content := &PackageContent{Repo: storedStatement{p: model.PackageStatement{ProblemID: problemID, Title: "Sum", Statements: statements}}}
		s := content.ReadStatement(context.Background(), problemID, uuid.New(), language).Statement
		require.Equal(t, "Sum", s.Title)
		return s.Legend
	}
	ru := model.DraftStatement{Language: "ru", Legend: "ru"}
	en := model.DraftStatement{Language: "en", Legend: "en"}
	de := model.DraftStatement{Language: "de", Legend: "de"}
	require.Equal(t, "ru", read("ru", en, ru))
	require.Equal(t, "en", read("fr", ru, en, de))
	require.Equal(t, "de", read("fr", ru, de))
	require.Empty(t, read("ru"))
}
