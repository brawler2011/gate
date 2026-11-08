//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
	_ "golang.org/x/image/webp"
)

func (s *IntegrationTestSuite) TestOpenAPIAlignment() {
	pngSig := []byte("\x89PNG\r\n\x1a\n")

	// Helper to generate test images in various formats
	makeImage := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				img.Set(x, y, color.RGBA{R: uint8(x * 15), G: uint8(y * 15), B: 100, A: 255})
			}
		}
		return img
	}

	makeJPEGBytes := func() []byte {
		var buf bytes.Buffer
		s.Require().NoError(jpeg.Encode(&buf, makeImage(), &jpeg.Options{Quality: 90}))
		return buf.Bytes()
	}

	makeGIFBytes := func() []byte {
		var buf bytes.Buffer
		s.Require().NoError(gif.Encode(&buf, makeImage(), nil))
		return buf.Bytes()
	}

	// 1. Avatar Media Normalization & Legacy Stored Object
	s.Run("AvatarUploadNormalization", func() {
		user := s.createUser("align_avatar_usr", accessmodel.UserRoleUser)

		// A. Upload JPEG avatar: must be normalized to PNG
		jpegBytes := makeJPEGBytes()
		resp, err := s.client.UploadAvatar(withTestUser(s.ctx, user.Id), &corev1.UploadAvatarReq{
			Avatar: ht.MultipartFile{
				Name:   "avatar.jpg",
				File:   bytes.NewReader(jpegBytes),
				Size:   int64(len(jpegBytes)),
				Header: textproto.MIMEHeader{"Content-Type": []string{"image/jpeg"}},
			},
		}, corev1.UploadAvatarParams{Username: user.Username})
		s.Require().NoError(err)
		s.Require().NotNil(resp)

		// Download avatar: returned bytes must be valid PNG with PNG magic header
		avatarRes, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{Username: user.Username})
		s.Require().NoError(err)
		okResp, ok := avatarRes.(*corev1.GetUserAvatarOKHeaders)
		s.Require().True(ok)
		s.True(okResp.ETag.IsSet())
		etag := okResp.ETag.Value
		s.NotEmpty(etag)

		data, err := io.ReadAll(okResp.Response.Data)
		s.Require().NoError(err)
		s.True(bytes.HasPrefix(data, pngSig), "stored and served avatar must be valid PNG")

		// Conditional request with If-None-Match returns 304 Not Modified
		notModRes, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username:    user.Username,
			IfNoneMatch: corev1.NewOptString(etag),
		})
		s.Require().NoError(err)
		_, is304 := notModRes.(*corev1.GetUserAvatarNotModified)
		s.True(is304, "matching ETag must return 304 Not Modified")

		// B. Upload GIF avatar: normalized to PNG
		gifBytes := makeGIFBytes()
		_, err = s.client.UploadAvatar(withTestUser(s.ctx, user.Id), &corev1.UploadAvatarReq{
			Avatar: ht.MultipartFile{
				Name:   "avatar.gif",
				File:   bytes.NewReader(gifBytes),
				Size:   int64(len(gifBytes)),
				Header: textproto.MIMEHeader{"Content-Type": []string{"image/gif"}},
			},
		}, corev1.UploadAvatarParams{Username: user.Username})
		s.Require().NoError(err)

		avatarRes2, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{Username: user.Username})
		s.Require().NoError(err)
		okResp2, ok := avatarRes2.(*corev1.GetUserAvatarOKHeaders)
		s.Require().True(ok)
		data2, err := io.ReadAll(okResp2.Response.Data)
		s.Require().NoError(err)
		s.True(bytes.HasPrefix(data2, pngSig), "GIF avatar must normalize to PNG")

		// C. Corrupted bytes rejected with 400 Bad Request
		_, err = s.client.UploadAvatar(withTestUser(s.ctx, user.Id), &corev1.UploadAvatarReq{
			Avatar: ht.MultipartFile{
				Name:   "broken.png",
				File:   bytes.NewReader([]byte("not an image at all")),
				Size:   19,
				Header: textproto.MIMEHeader{"Content-Type": []string{"image/png"}},
			},
		}, corev1.UploadAvatarParams{Username: user.Username})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	})

	s.Run("LegacyNonPNGStoredAvatarConversion", func() {
		// Simulate a user whose avatar was stored as raw JPEG prior to normalization
		user := s.createUser("align_legacy_usr", accessmodel.UserRoleUser)
		legacyKey := uuid.NewString()
		jpegBytes := makeJPEGBytes()

		// Directly write raw JPEG into storage bucket
		err := s.storage.UploadFile(s.ctx, "avatars", legacyKey, bytes.NewReader(jpegBytes), "image/jpeg")
		s.Require().NoError(err)

		// Update user avatar_url in database directly
		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET avatar_url=$1 WHERE id=$2", legacyKey, user.Id)
		s.Require().NoError(err)

		// GetUserAvatar on the legacy object must convert on-the-fly and emit valid PNG
		avatarRes, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{Username: user.Username})
		s.Require().NoError(err)
		okResp, ok := avatarRes.(*corev1.GetUserAvatarOKHeaders)
		s.Require().True(ok)
		s.True(okResp.ETag.IsSet())
		legacyEtag := okResp.ETag.Value
		s.NotEmpty(legacyEtag)

		servedBytes, err := io.ReadAll(okResp.Response.Data)
		s.Require().NoError(err)
		s.True(bytes.HasPrefix(servedBytes, pngSig), "legacy JPEG avatar must be served as PNG")

		// ETag/304 caching still functions for legacy avatars
		cachedRes, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username:    user.Username,
			IfNoneMatch: corev1.NewOptString(legacyEtag),
		})
		s.Require().NoError(err)
		_, is304 := cachedRes.(*corev1.GetUserAvatarNotModified)
		s.True(is304)
	})

	// 2. Missing Multipart Required Fields Validation
	s.Run("MissingMultipartRequiredFields", func() {
		admin := s.createUser("align_blog_adm", accessmodel.UserRoleAdmin)

		// POST /posts without preview_image or required fields triggers 400 Bad Request
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("title", "Post Title")
		// Missing description, text, preview_image
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/posts", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		req.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})

		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, req)
		s.Equal(http.StatusBadRequest, rec.Code, "missing multipart required fields must return 400")

		// POST /problems/{id}/import missing package field returns 400
		org := s.createOrganization("import_prob_org", "Import Prob Org", admin.Id)
		problemResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
			Title:          "Import Problem",
			OrganizationID: corev1.NewOptUUID(org.ID),
			TemplateID:     "builtin:a-plus-b",
		})
		s.Require().NoError(err)

		impBody := &bytes.Buffer{}
		impWriter := multipart.NewWriter(impBody)
		_ = impWriter.WriteField("dummy", "value")
		_ = impWriter.Close()

		impReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/problems/%s/import", problemResp.ID), impBody)
		impReq.Header.Set("Content-Type", impWriter.FormDataContentType())
		impReq.Header.Set("X-Test-User-ID", admin.Id.String())
		impReq.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})

		impRec := httptest.NewRecorder()
		s.handler.ServeHTTP(impRec, impReq)
		s.Equal(http.StatusBadRequest, impRec.Code, "missing 'package' multipart field must return 400")
	})

	// 3. Team Name Length Constraints (3 to 64)
	s.Run("TeamNameConstraints", func() {
		user := s.createUser("align_team_usr", accessmodel.UserRoleUser)
		org := s.createOrganization("team_org", "Team Org Name", user.Id)

		// Name < 3 characters
		_, err := s.client.CreateTeam(withTestUser(s.ctx, user.Id), &corev1.CreateTeamReq{
			Name:           "ab",
			OrganizationID: org.ID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err), "team name < 3 chars must return 400")

		// Name > 64 characters
		_, err = s.client.CreateTeam(withTestUser(s.ctx, user.Id), &corev1.CreateTeamReq{
			Name:           strings.Repeat("x", 65),
			OrganizationID: org.ID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err), "team name > 64 chars must return 400")

		// Valid name (length 3..64)
		resp, err := s.client.CreateTeam(withTestUser(s.ctx, user.Id), &corev1.CreateTeamReq{
			Name:           "Valid Team Name",
			OrganizationID: org.ID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
	})

	// 4. Role Validation Across Domain Vocabularies
	s.Run("DomainRoleValidation", func() {
		owner := s.createUser("align_role_own", accessmodel.UserRoleUser)
		other := s.createUser("align_role_tgt", accessmodel.UserRoleUser)
		org := s.createOrganization("align-role-org", "Role Org", owner.Id)
		s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, other.Id, organizationsmodel.OrgRoleMember))

		// Organization membership: invalid role "viewer" (valid are owner, admin, member)
		badOrgReq := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/organizations/%s/members?user_id=%s&role=viewer", org.Login, other.Id), nil)
		badOrgReq.Header.Set("X-Test-User-ID", owner.Id.String())
		badOrgReq.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})
		badOrgRec := httptest.NewRecorder()
		s.handler.ServeHTTP(badOrgRec, badOrgReq)
		s.Equal(http.StatusBadRequest, badOrgRec.Code, "invalid org role must return 400")

		// Team membership: create team first
		teamResp, err := s.client.CreateTeam(withTestUser(s.ctx, owner.Id), &corev1.CreateTeamReq{
			Name:           "Role Test Team",
			OrganizationID: org.ID,
		})
		s.Require().NoError(err)
		teamID := teamResp.ID

		// Add member as valid "member"
		err = s.client.AddTeamMember(withTestUser(s.ctx, owner.Id), corev1.AddTeamMemberParams{
			ID:     teamID,
			UserID: other.Id,
			Role:   corev1.NewOptAddTeamMemberRole(corev1.AddTeamMemberRoleMember),
		})
		s.Require().NoError(err)

		// Team update: invalid role "admin" (valid are maintainer, member)
		badTeamReq := httptest.NewRequest(http.MethodPatch,
			fmt.Sprintf("/teams/%s/members?user_id=%s&role=admin", teamID, other.Id), nil)
		badTeamReq.Header.Set("X-Test-User-ID", owner.Id.String())
		badTeamReq.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})
		badTeamRec := httptest.NewRecorder()
		s.handler.ServeHTTP(badTeamRec, badTeamReq)
		s.Equal(http.StatusBadRequest, badTeamRec.Code, "invalid team role must return 400")

		// Contest membership: create contest
		contestID := uuid.New()
		err = s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &owner.Id,
			Title:          "Role Contest",
			Login:          "align-contest",
			Visibility:     contestsmodel.ContestVisibilityPublic,
		})
		s.Require().NoError(err)

		// Update contest member: invalid role "viewer" (valid are owner, moderator, participant)
		badContestReq := httptest.NewRequest(http.MethodPatch,
			fmt.Sprintf("/organizations/%s/contests/%s/members?user_id=%s&role=viewer", org.Login, "align-contest", other.Id), nil)
		badContestReq.Header.Set("X-Test-User-ID", owner.Id.String())
		badContestReq.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})
		badContestRec := httptest.NewRecorder()
		s.handler.ServeHTTP(badContestRec, badContestReq)
		s.Equal(http.StatusBadRequest, badContestRec.Code, "invalid contest role must return 400")

		// Contest team update: invalid role "invalid_role" (valid are moderator, participant)
		badContestTeamReq := httptest.NewRequest(http.MethodPatch,
			fmt.Sprintf("/organizations/%s/contests/%s/teams?team_id=%s&role=invalid_role", org.Login, "align-contest", teamID), nil)
		badContestTeamReq.Header.Set("X-Test-User-ID", owner.Id.String())
		badContestTeamReq.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})
		badContestTeamRec := httptest.NewRecorder()
		s.handler.ServeHTTP(badContestTeamRec, badContestTeamReq)
		s.Equal(http.StatusBadRequest, badContestTeamRec.Code, "invalid contest team role must return 400")
	})

	// 5. Error Response Envelope ({error, message, request_id})
	s.Run("ErrorEnvelopeFormat", func() {
		// Request a non-existent post to trigger middleware error handler
		req := httptest.NewRequest(http.MethodGet, "/posts/"+uuid.NewString(), nil)
		req.Header.Set("X-Request-ID", "envelope-test-id")

		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, req)

		s.Equal(http.StatusNotFound, rec.Code)
		s.Equal("envelope-test-id", rec.Header().Get("X-Request-ID"))

		var envelope struct {
			Error     string `json:"error"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}
		err := json.NewDecoder(rec.Body).Decode(&envelope)
		s.Require().NoError(err)
		s.NotEmpty(envelope.Error, "envelope 'error' field must be populated")
		s.NotEmpty(envelope.Message, "envelope 'message' field must be populated")
		s.Equal("envelope-test-id", envelope.RequestID, "envelope 'request_id' must match client/middleware request id")
	})

	// 6. Relative Server Base Path (/api)
	s.Run("ServerMetadataBasePath", func() {
		// Verify openapi.yaml declares servers: [{url: /api, ...}]
		_, b, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(b)
		specPath := filepath.Join(basepath, "../../../contracts/core/v1/openapi.yaml")
		content, err := os.ReadFile(specPath)
		s.Require().NoError(err)
		s.Contains(string(content), "- url: /api")

		// Verify runtime behavior: when mounted under /api/ with http.StripPrefix, endpoints resolve properly
		apiMux := http.NewServeMux()
		apiMux.Handle("/api/", http.StripPrefix("/api", s.handler))

		req := httptest.NewRequest(http.MethodGet, "/api/languages", nil)
		rec := httptest.NewRecorder()
		apiMux.ServeHTTP(rec, req)

		s.Equal(http.StatusOK, rec.Code, "/api/languages must be accessible under /api prefix")
		var langs corev1.SupportedLanguagesResponse
		err = json.NewDecoder(rec.Body).Decode(&langs)
		s.Require().NoError(err)
		s.NotEmpty(langs.Languages)
	})
}
