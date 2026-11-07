package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type bodyLimitHandler struct {
	corev1.UnimplementedHandler
	calls int
	bytes int64
}

func (h *bodyLimitHandler) Login(context.Context, *corev1.LoginRequestModel) (*corev1.AuthResponseModelHeaders, error) {
	h.calls++
	return nil, pkg.ErrConflict
}

func (h *bodyLimitHandler) AdminSetPassword(context.Context, *corev1.AdminSetPasswordRequestModel, corev1.AdminSetPasswordParams) error {
	h.calls++
	return pkg.ErrConflict
}

func (h *bodyLimitHandler) UploadAvatar(_ context.Context, req *corev1.UploadAvatarReq, _ corev1.UploadAvatarParams) (*corev1.UploadAvatarOK, error) {
	h.calls++
	n, err := io.Copy(io.Discard, req.Avatar.File)
	h.bytes = n
	if err != nil {
		return nil, fmt.Errorf("read avatar: %w", err)
	}
	return nil, pkg.ErrConflict
}

func (h *bodyLimitHandler) UpdateProblemLib(_ context.Context, req corev1.UpdateProblemLibReq, _ corev1.UpdateProblemLibParams) (*corev1.MessageResponse, error) {
	h.calls++
	n, err := io.Copy(io.Discard, req.Data)
	h.bytes = n
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "read upload")
	}
	return nil, pkg.ErrConflict
}

type countedBody struct {
	io.Reader
	read int
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err //nolint:wrapcheck // Preserve io.EOF and reader errors while counting bytes.
}

func (*countedBody) Close() error { return nil }

func limitedHTTPHandler(t *testing.T, handler corev1.Handler, limit func(string) int64) http.Handler {
	t.Helper()
	onError := middleware.ResponseErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
	server, err := corev1.NewServer(handler, middleware.NewSecurityHandler(), corev1.WithErrorHandler(onError), corev1.WithMaxMultipartMemory(1024))
	require.NoError(t, err)
	return middleware.RequestLimits(server, limit, onError)
}

func TestJSONRequestBodyBoundaries(t *testing.T) {
	const jsonBody = `{"identifier":"alice","password":"secret"}`
	for _, unknownLength := range []bool{false, true} {
		for _, size := range []int{DefaultRequestBodyLimit - 1, DefaultRequestBodyLimit, DefaultRequestBodyLimit + 1, 3 << 20} {
			t.Run(fmt.Sprintf("unknown=%t/size=%d", unknownLength, size), func(t *testing.T) {
				stub := &bodyLimitHandler{}
				handler := limitedHTTPHandler(t, stub, RequestBodyLimit)
				body := &countedBody{Reader: strings.NewReader(jsonBody + strings.Repeat(" ", size-len(jsonBody)))}
				req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
				req.Body, req.ContentLength = body, int64(size)
				if unknownLength {
					req.ContentLength = -1
					req.TransferEncoding = []string{"chunked"}
				}
				req.Header.Set("Content-Type", "application/json")
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				if size <= DefaultRequestBodyLimit {
					require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
					require.Equal(t, 1, stub.calls)
				} else {
					require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code, rr.Body.String())
					require.Zero(t, stub.calls)
					if !unknownLength {
						require.Zero(t, body.read)
					}
				}
				require.LessOrEqual(t, body.read, DefaultRequestBodyLimit+1)
			})
		}
	}
}

func TestProtectedRequestsRejectBeforeReading(t *testing.T) {
	for _, tc := range []struct {
		path string
		user accessmodel.User
		want int
	}{
		{path: "/admin/users/alice/set-password", user: accessmodel.Guest, want: http.StatusUnauthorized},
		{path: "/admin/users/alice/set-password", user: accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}, want: http.StatusForbidden},
		{path: "/users/alice/avatar", user: accessmodel.Guest, want: http.StatusUnauthorized},
		{path: "/problems/" + uuid.NewString() + "/import", user: accessmodel.Guest, want: http.StatusUnauthorized},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.path, tc.want), func(t *testing.T) {
			stub := &bodyLimitHandler{}
			body := &countedBody{Reader: strings.NewReader("malformed body")}
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req = req.WithContext(accessmodel.WithUser(req.Context(), tc.user))
			req.Body, req.ContentLength = body, -1
			rr := httptest.NewRecorder()
			limitedHTTPHandler(t, stub, RequestBodyLimit).ServeHTTP(rr, req)
			require.Equal(t, tc.want, rr.Code, rr.Body.String())
			require.Zero(t, body.read)
			require.Zero(t, stub.calls)
		})
	}
}

func TestAdminRequestBodyLimit(t *testing.T) {
	for _, unknownLength := range []bool{false, true} {
		for _, user := range []accessmodel.User{accessmodel.Guest, {Id: uuid.New(), Role: accessmodel.UserRoleAdmin}} {
			stub := &bodyLimitHandler{}
			req := httptest.NewRequest(http.MethodPost, "/admin/users/alice/set-password", strings.NewReader(`{"password":"`+strings.Repeat("x", 3<<20)+`"}`))
			req = req.WithContext(accessmodel.WithUser(req.Context(), user))
			body := &countedBody{Reader: req.Body}
			req.Body = body
			if unknownLength {
				req.ContentLength = -1
			}
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			limitedHTTPHandler(t, stub, RequestBodyLimit).ServeHTTP(rr, req)
			want := http.StatusRequestEntityTooLarge
			if unknownLength && user.IsGuest() {
				want = http.StatusUnauthorized
			}
			require.Equal(t, want, rr.Code, rr.Body.String())
			require.Zero(t, stub.calls)
			if user.IsGuest() || !unknownLength {
				require.Zero(t, body.read)
			}
			require.LessOrEqual(t, body.read, DefaultRequestBodyLimit+1)
		}
	}
}

func TestMultipartTotalBodyLimit(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	require.NoError(t, err)
	_, err = io.WriteString(part, strings.Repeat("x", DefaultRequestBodyLimit+1))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	for _, unknownLength := range []bool{false, true} {
		for _, overLimit := range []bool{false, true} {
			t.Run(fmt.Sprintf("unknown=%t/over=%t", unknownLength, overLimit), func(t *testing.T) {
				stub := &bodyLimitHandler{}
				limit := int64(buf.Len())
				if overLimit {
					limit--
				}
				body := &countedBody{Reader: bytes.NewReader(buf.Bytes())}
				req := httptest.NewRequest(http.MethodPost, "/users/alice/avatar", nil)
				req = req.WithContext(accessmodel.WithUser(req.Context(), accessmodel.User{Id: uuid.New(), Username: "alice", Role: accessmodel.UserRoleUser}))
				req.Body, req.ContentLength = body, int64(buf.Len())
				if unknownLength {
					req.ContentLength = -1
				}
				req.Header.Set("Content-Type", writer.FormDataContentType())
				rr := httptest.NewRecorder()
				limitedHTTPHandler(t, stub, func(string) int64 { return limit }).ServeHTTP(rr, req)
				if overLimit {
					require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code, rr.Body.String())
					require.Zero(t, stub.calls)
				} else {
					require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
					require.EqualValues(t, DefaultRequestBodyLimit+1, stub.bytes)
				}
				require.LessOrEqual(t, int64(body.read), limit+1)
			})
		}
	}
}

func TestRawUploadRetainsSourceLimit(t *testing.T) {
	for _, size := range []int{DefaultRequestBodyLimit + 1, MaxSourceFileSize, MaxSourceFileSize + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			stub := &bodyLimitHandler{}
			req := httptest.NewRequest(http.MethodPut, "/problems/"+uuid.NewString()+"/lib/main.cpp", strings.NewReader(strings.Repeat("x", size)))
			req = req.WithContext(accessmodel.WithUser(req.Context(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}))
			req.ContentLength = -1
			req.Header.Set("Content-Type", "text/plain")
			rr := httptest.NewRecorder()
			limitedHTTPHandler(t, stub, RequestBodyLimit).ServeHTTP(rr, req)
			want := http.StatusConflict
			if size > MaxSourceFileSize {
				want = http.StatusRequestEntityTooLarge
			}
			require.Equal(t, want, rr.Code, rr.Body.String())
		})
	}
}

func TestUploadLimits(t *testing.T) {
	for operation, want := range map[string]int64{
		"Login":                           DefaultRequestBodyLimit,
		"CreateSubmission":                DefaultRequestBodyLimit,
		"ImportProblem":                   MaxPackageZipSize + multipartOverheadLimit,
		"UploadAvatar":                    MaxMediaFileSize + multipartOverheadLimit,
		"CreatePost":                      MaxMediaFileSize + multipartOverheadLimit,
		"PatchPostById":                   MaxMediaFileSize + multipartOverheadLimit,
		"CreateProblemMediaFile":          MaxMediaFileSize,
		"UpdateProblemMediaFile":          MaxMediaFileSize,
		"CreateProblemTestFile":           MaxTestFileSize,
		"UpdateProblemTestFile":           MaxTestFileSize,
		"CreateProblemChecker":            MaxSourceFileSize,
		"UpdateProblemWorkshopSubmission": MaxSourceFileSize,
	} {
		require.Equal(t, want, RequestBodyLimit(operation), operation)
	}
}

func TestWorkshopReadsAreBounded(t *testing.T) {
	for _, size := range []int{7, 8, 9, 1000} {
		body := &countedBody{Reader: strings.NewReader(strings.Repeat("x", size))}
		data, err := readWorkshopBody(body, 8)
		if size > 8 {
			require.ErrorIs(t, err, pkg.ErrPayloadTooLarge)
			require.Equal(t, http.StatusRequestEntityTooLarge, pkg.ToREST(err))
		} else {
			require.NoError(t, err)
			require.Len(t, data, size)
		}
		require.LessOrEqual(t, body.read, 9)
	}
}

type failedBody struct{}

func (failedBody) Read([]byte) (int, error) { return 0, errors.New("upload interrupted") }

func TestSVGReadFailureIsReturned(t *testing.T) {
	h := &CoreServer{}
	_, err := h.CreateProblemMediaFile(context.Background(), corev1.CreateProblemMediaFileReq{Data: failedBody{}}, corev1.CreateProblemMediaFileParams{Name: "image.svg"})
	require.ErrorContains(t, err, "upload interrupted")
	_, err = h.UpdateProblemMediaFile(context.Background(), corev1.UpdateProblemMediaFileReq{Data: failedBody{}}, corev1.UpdateProblemMediaFileParams{Name: "image.svg"})
	require.ErrorContains(t, err, "upload interrupted")
}

func TestWorkshopSourceRejectsBeforeProcessing(t *testing.T) {
	h := &CoreServer{}
	body := &countedBody{Reader: strings.NewReader(strings.Repeat("x", MaxSourceFileSize+100))}
	_, err := h.CreateProblemChecker(context.Background(), corev1.CreateProblemCheckerReq{Data: body}, corev1.CreateProblemCheckerParams{Name: "checker.cpp"})
	require.ErrorIs(t, err, pkg.ErrPayloadTooLarge)
	require.Equal(t, http.StatusRequestEntityTooLarge, pkg.ToREST(err))
	require.Equal(t, MaxSourceFileSize+1, body.read)
}
