package core

import (
	"context"
	"net/http"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/blogs"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type missingPostImageModule struct {
	BlogsReader
	*blogs.BlogsUseCase
	post blogsmodel.Post
}

func (m missingPostImageModule) GetPostByID(_ context.Context, id uuid.UUID) (*blogsmodel.Post, error) {
	if id != m.post.ID {
		return nil, pkg.ErrNotFound
	}
	return &m.post, nil
}

func TestGetPostImageMissingStorageObject(t *testing.T) {
	post := blogsmodel.Post{ID: uuid.New(), ImageKey: "missing.png"}
	module := missingPostImageModule{
		BlogsUseCase: blogs.NewBlogsUseCase(nil, storage.NewLocalStorage(t.TempDir()), "blogs"),
		post:         post,
	}
	server := &CoreServer{blogs: module}

	response, err := server.GetPostImage(context.Background(), corev1.GetPostImageParams{ID: post.ID})

	require.Nil(t, response)
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.ErrorIs(t, err, pkg.ErrNotFound)
	require.Equal(t, http.StatusNotFound, pkg.ToREST(err))
}
