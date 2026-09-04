//go:build integration
// +build integration

package integration

import (
	"net/http"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestProblems() {
	admin := s.createUser("admin_problems", models.UserRoleAdmin)

	org := s.createOrganization("admin-org", "Admin Organization", admin.Id)

	var problemID uuid.UUID

	// 1. Create Problem (Admin)
	s.Run("CreateProblem", func() {
		title := "Test Problem"
		resp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
			Title:          title,
			OrganizationID: corev1.NewOptUUID(org.ID),
			TemplateID:     "builtin:a-plus-b",
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		problemID = resp.ID
	})

	// 2. Get Problem (Admin)
	s.Run("GetProblem", func() {
		resp, err := s.client.GetProblem(withTestUser(s.ctx, admin.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(problemID, resp.Problem.ID)
	})

	// 3. Update Problem (Admin)
	s.Run("UpdateProblem", func() {
		newTitle := "Updated Problem"
		visibility := "public"
		err := s.client.UpdateProblem(withTestUser(s.ctx, admin.Id), &corev1.UpdateProblemRequestModel{
			Title:      corev1.NewOptString(newTitle),
			Visibility: corev1.NewOptString(visibility),
		}, corev1.UpdateProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
	})

	// 4. List Problems
	s.Run("ListProblems", func() {
		resp, err := s.client.ListProblems(withTestUser(s.ctx, admin.Id), corev1.ListProblemsParams{
			Page:     corev1.NewOptInt32(1),
			PageSize: corev1.NewOptInt32(10),
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.GreaterOrEqual(len(resp.Problems), 1)
	})

	// 5. Delete Problem (Admin)
	s.Run("DeleteProblem", func() {
		err := s.client.DeleteProblem(withTestUser(s.ctx, admin.Id), corev1.DeleteProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
	})
}

func (s *IntegrationTestSuite) TestProblemTemplates() {
	admin := s.createUser("admin_templates", models.UserRoleAdmin)
	org := s.createOrganization("templates-org", "Templates Organization", admin.Id)

	var problemID uuid.UUID

	// 1. Create Problem A
	s.Run("CreateProblemA", func() {
		title := "Problem A"
		resp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
			Title:          title,
			OrganizationID: corev1.NewOptUUID(org.ID),
			TemplateID:     "builtin:a-plus-b",
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		problemID = resp.ID
	})

	// 2. Try to set as template (should fail because there are no packages)
	s.Run("SetTemplateFailsNoPackages", func() {
		isTemplate := true
		err := s.client.UpdateProblem(withTestUser(s.ctx, admin.Id), &corev1.UpdateProblemRequestModel{
			IsTemplate: corev1.NewOptBool(isTemplate),
		}, corev1.UpdateProblemParams{
			ID: problemID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	})

	// 3. Publish a package for Problem A
	s.Run("PublishPackage", func() {
		_, err := s.client.PublishProblem(withTestUser(s.ctx, admin.Id), corev1.PublishProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
	})

	// 4. Set as template (should succeed now)
	s.Run("SetTemplateSucceeds", func() {
		isTemplate := true
		err := s.client.UpdateProblem(withTestUser(s.ctx, admin.Id), &corev1.UpdateProblemRequestModel{
			IsTemplate: corev1.NewOptBool(isTemplate),
		}, corev1.UpdateProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)

		// Verify metadata in GetProblem
		getResp, err := s.client.GetProblem(withTestUser(s.ctx, admin.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(getResp)
		s.True(getResp.Problem.IsTemplate)
	})

	// 5. List Problem Templates
	s.Run("ListProblemTemplates", func() {
		tmplResp, err := s.client.ListProblemTemplates(withTestUser(s.ctx, admin.Id), corev1.ListProblemTemplatesParams{
			OrganizationID: corev1.NewOptUUID(org.ID),
		})
		s.Require().NoError(err)
		s.Require().NotNil(tmplResp)
		s.GreaterOrEqual(len(tmplResp), 4) // 3 builtin + at least 1 org template
	})

	// 6. Create Problem B using Problem A as a template
	s.Run("CreateProblemFromTemplate", func() {
		title := "Problem B"
		templateID := problemID.String()
		resp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
			Title:          title,
			OrganizationID: corev1.NewOptUUID(org.ID),
			TemplateID:     templateID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		newProblemID := resp.ID

		// Verify Problem B details
		getResp, err := s.client.GetProblem(withTestUser(s.ctx, admin.Id), corev1.GetProblemParams{
			ID: newProblemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(getResp)
		s.Equal("Problem B", getResp.Problem.Title)
		s.False(getResp.Problem.IsTemplate)
	})
}

