//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"sync"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/blogs"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type blogUploadHook struct {
	storage.Storage
	afterUpload func()
}

func (s blogUploadHook) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	if err := s.Storage.UploadFile(ctx, bucket, key, reader, contentType); err != nil {
		return err
	}
	s.afterUpload()
	return nil
}
func (s *IntegrationTestSuite) TestBlogCommandsAuthorizeAfterUploadAndSerializeImages() {
	admin := s.createUser("blog_command_admin", accessmodel.UserRoleAdmin)
	ctx := accessmodel.WithUser(s.ctx, admin)
	local := storage.NewLocalStorage(s.T().TempDir())
	var data bytes.Buffer
	s.Require().NoError(png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	imageData := data.Bytes()
	hook := blogUploadHook{Storage: local, afterUpload: func() {
		_, err := s.dbPool.Exec(s.ctx, `UPDATE users SET role='user' WHERE id=$1`, admin.Id)
		s.Require().NoError(err)
	}}
	uc := blogs.NewBlogsUseCase(pgstore.NewBlogTransactions(s.dbPool), hook, "blogs")
	_, err := uc.CreatePost(ctx, "Denied", "Text", "Description", bytes.NewReader(imageData), "image.png")
	s.Require().ErrorIs(err, pkg.NoPermission)
	objects, err := local.ListFiles(s.ctx, "blogs", "")
	s.Require().NoError(err)
	s.Empty(objects, "failed authorization cleans up the staged object")
	_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin.Id)
	s.Require().NoError(err)
	uc = blogs.NewBlogsUseCase(pgstore.NewBlogTransactions(s.dbPool), local, "blogs")
	id, err := uc.CreatePost(ctx, "Original", "Text", "Description", bytes.NewReader(imageData), "image.png")
	s.Require().NoError(err)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			results <- uc.UpdatePost(ctx, id, nil, nil, nil, bytes.NewReader(imageData), "image.png")
		})
	}
	wg.Wait()
	close(results)
	for err := range results {
		s.Require().NoError(err)
	}
	post, err := pgstore.NewBlogsRepo(s.dbPool).GetPostByID(s.ctx, id)
	s.Require().NoError(err)
	s.Equal(admin.Id, post.AuthorUUID)
	objects, err = local.ListFiles(s.ctx, "blogs", "")
	s.Require().NoError(err)
	s.Equal([]string{post.ImageKey}, objects, "serialized updates clean up each displaced image")
	s.Require().NoError(uc.DeletePost(ctx, id))
	_, err = pgstore.NewBlogsRepo(s.dbPool).GetPostByID(s.ctx, id)
	s.Require().ErrorIs(err, pkg.ErrNotFound)
	objects, err = local.ListFiles(s.ctx, "blogs", "")
	s.Require().NoError(err)
	s.Empty(objects)
}

func (s *IntegrationTestSuite) TestListPostsPagination() {
	admin := s.createUser("blog_page_admin_"+uuid.NewString()[:8], accessmodel.UserRoleAdmin)
	ids := make([]uuid.UUID, 3)
	for i := range ids {
		ids[i] = uuid.New()
		_, err := s.dbPool.Exec(s.ctx, "INSERT INTO posts(id, title, text, description, author_uuid, author_name, image_key, created_at) VALUES($1, $2, 'text', 'description', $3, $4, 'preview', $5)",
			ids[i], fmt.Sprintf("Paged Post %d", i), admin.Id, admin.Username, time.Now().Add(time.Duration(i)*time.Hour))
		s.Require().NoError(err)
	}

	resAsc, err := s.client.ListPosts(s.ctx, corev1.ListPostsParams{
		Page:      corev1.NewOptInt32(1),
		PageSize:  corev1.NewOptInt32(1),
		SortOrder: corev1.NewOptListPostsSortOrder(corev1.ListPostsSortOrderAsc),
	})
	s.Require().NoError(err)
	s.Require().NotNil(resAsc)
	s.Require().Len(resAsc.Posts, 1)

	resDesc, err := s.client.ListPosts(s.ctx, corev1.ListPostsParams{
		Page:      corev1.NewOptInt32(1),
		PageSize:  corev1.NewOptInt32(1),
		SortOrder: corev1.NewOptListPostsSortOrder(corev1.ListPostsSortOrderDesc),
	})
	s.Require().NoError(err)
	s.Require().NotNil(resDesc)
	s.Require().Len(resDesc.Posts, 1)
	s.NotEqual(resAsc.Posts[0].ID, resDesc.Posts[0].ID)

	got, err := s.client.GetPostById(s.ctx, corev1.GetPostByIdParams{ID: ids[0]})
	s.Require().NoError(err)
	postModel, ok := got.(*corev1.PostModel)
	s.Require().True(ok)
	s.Equal("Paged Post 0", postModel.Title)

	notfound, err := s.client.GetPostById(s.ctx, corev1.GetPostByIdParams{ID: uuid.New()})
	s.Require().NoError(err)
	_, isNotFound := notfound.(*corev1.GetPostByIdNotFound)
	s.True(isNotFound)
}
