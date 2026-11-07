package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type failingBlogs struct {
	BlogsModule
	err error
}

func (b failingBlogs) CreatePost(context.Context, string, string, string, io.Reader, string) (uuid.UUID, error) {
	return uuid.Nil, b.err
}
func (b failingBlogs) ListPosts(context.Context, blogsmodel.ListPostsParams) ([]blogsmodel.Post, int, error) {
	return nil, 0, b.err
}

func TestBlogHandlersPreserveErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"conflict", pkg.ErrConflict, http.StatusConflict},
		{"bad input", pkg.ErrBadInput, http.StatusBadRequest},
		{"internal", errors.New("storage failed"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := &CoreServer{blogs: failingBlogs{err: tc.err}}
			ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
			t.Run("create", func(t *testing.T) {
				response, err := server.CreatePost(ctx, &corev1.CreatePostReq{
					Title: corev1.NewOptString("Post"), Description: corev1.NewOptString("Description"), Text: corev1.NewOptString("Text"), PreviewImage: corev1.OptMultipartFile{Set: true},
				})
				require.ErrorIs(t, err, tc.err)
				require.Equal(t, tc.status, pkg.ToREST(err))
				require.Nil(t, response)
			})
			t.Run("list", func(t *testing.T) {
				response, err := server.ListPosts(ctx, corev1.ListPostsParams{})
				require.ErrorIs(t, err, tc.err)
				require.Equal(t, tc.status, pkg.ToREST(err))
				require.Nil(t, response)
			})
		})
	}
}
