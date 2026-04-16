//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *IntegrationTestSuite) TestWorkshopStatementEndpointSyncsProblemTitle() {
	admin := s.createUser("workshop_statement_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-statement-org", "Workshop Statement Org", admin.Id)

	organizationID := openapi_types.UUID(org.ID)
	createResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title:          "Original Workshop Title",
		OrganizationId: &organizationID,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, createResp.StatusCode())
	s.Require().NotNil(createResp.JSON200)

	problemID := createResp.JSON200.Id

	initResp, err := s.client.InitProblemWorkshopWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, initResp.StatusCode())

	newTitle := "Synced From Statement Endpoint"
	updateResp, err := s.client.UpdateProblemStatementWithResponse(s.ctx, problemID, corev1.UpdateProblemStatementJSONRequestBody{
		Title: &newTitle,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateResp.StatusCode())
	s.Require().NotNil(updateResp.JSON200)
	s.Equal(newTitle, updateResp.JSON200.Title)

	problemResp, err := s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, problemResp.StatusCode())
	s.Require().NotNil(problemResp.JSON200)
	s.Equal(newTitle, problemResp.JSON200.Problem.Title)
}

func (s *IntegrationTestSuite) TestWorkshopCheckerEndpointsCRUD() {
	admin := s.createUser("workshop_checker_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-checker-org", "Workshop Checker Org", admin.Id)

	organizationID := openapi_types.UUID(org.ID)
	createResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title:          "Checker CRUD Problem",
		OrganizationId: &organizationID,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, createResp.StatusCode())
	s.Require().NotNil(createResp.JSON200)

	problemID := createResp.JSON200.Id

	initResp, err := s.client.InitProblemWorkshopWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, initResp.StatusCode())

	params := &corev1.CreateProblemCheckerParams{Name: "checker.cpp"}
	checkerSource := []byte("int main(){return 0;}")
	createCheckerResp, err := s.client.CreateProblemCheckerWithBodyWithResponse(
		s.ctx,
		problemID,
		params,
		"application/octet-stream",
		bytes.NewReader(checkerSource),
		func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		},
	)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, createCheckerResp.StatusCode())

	listResp, err := s.client.ListProblemCheckersWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, listResp.StatusCode())
	s.Require().NotNil(listResp.JSON200)
	s.Require().NotNil(listResp.JSON200.Files)
	s.Len(*listResp.JSON200.Files, 1)
	s.Require().NotNil((*listResp.JSON200.Files)[0].Path)
	s.Equal("checkers/checker.cpp", *(*listResp.JSON200.Files)[0].Path)

	getResp, err := s.client.GetProblemCheckerWithResponse(s.ctx, problemID, "checker.cpp", func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, getResp.StatusCode())
	s.Equal(string(checkerSource), string(getResp.Body))

	updatedCheckerSource := []byte("int main(){return 1;}")
	updateCheckerResp, err := s.client.UpdateProblemCheckerWithBodyWithResponse(
		s.ctx,
		problemID,
		"checker.cpp",
		"application/octet-stream",
		bytes.NewReader(updatedCheckerSource),
		func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		},
	)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateCheckerResp.StatusCode())

	setMainResp, err := s.client.SetProblemCheckerMainWithResponse(s.ctx, problemID, corev1.SetProblemCheckerMainJSONRequestBody{
		Name: "checker.cpp",
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, setMainResp.StatusCode())

	deleteResp, err := s.client.DeleteProblemCheckerWithResponse(s.ctx, problemID, "checker.cpp", func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, deleteResp.StatusCode())

	listAfterDeleteResp, err := s.client.ListProblemCheckersWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, listAfterDeleteResp.StatusCode())
	s.Require().NotNil(listAfterDeleteResp.JSON200)
	s.Require().NotNil(listAfterDeleteResp.JSON200.Files)
	s.Len(*listAfterDeleteResp.JSON200.Files, 0)
}

func (s *IntegrationTestSuite) TestWorkshopTestsConfigEndpoint() {
	admin := s.createUser("workshop_tests_config_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-tests-config-org", "Workshop Tests Config Org", admin.Id)

	organizationID := openapi_types.UUID(org.ID)
	createResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title:          "Tests Config Problem",
		OrganizationId: &organizationID,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, createResp.StatusCode())
	s.Require().NotNil(createResp.JSON200)

	problemID := createResp.JSON200.Id

	initResp, err := s.client.InitProblemWorkshopWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, initResp.StatusCode())

	testsConfig := corev1.UpdateProblemTestsConfigJSONRequestBody{
		"groups": []map[string]interface{}{
			{
				"ordinal":       0,
				"name":          "Updated Samples",
				"points":        0,
				"points_policy": "complete-group",
				"depends_on":    []int{},
				"tests":         [2]int{1, 1},
			},
		},
		"tests": []map[string]interface{}{
			{
				"ordinal":   1,
				"method":    "manual",
				"generator": nil,
				"is_sample": true,
			},
		},
	}

	updateResp, err := s.client.UpdateProblemTestsConfigWithResponse(s.ctx, problemID, testsConfig, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateResp.StatusCode())

	getResp, err := s.client.GetProblemTestFileWithResponse(s.ctx, problemID, "tests.json", func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, getResp.StatusCode())

	var parsed map[string]interface{}
	err = json.Unmarshal(getResp.Body, &parsed)
	s.Require().NoError(err)

	groupsRaw, ok := parsed["groups"].([]interface{})
	s.Require().True(ok)
	s.Require().NotEmpty(groupsRaw)

	group0, ok := groupsRaw[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("Updated Samples", group0["name"])
}

func (s *IntegrationTestSuite) TestWorkshopReadmeEndpoint() {
	admin := s.createUser("workshop_readme_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-readme-org", "Workshop Readme Org", admin.Id)

	organizationID := openapi_types.UUID(org.ID)
	createResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title:          "Readme Endpoint Problem",
		OrganizationId: &organizationID,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, createResp.StatusCode())
	s.Require().NotNil(createResp.JSON200)

	problemID := createResp.JSON200.Id

	initResp, err := s.client.InitProblemWorkshopWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, initResp.StatusCode())

	readmeContent := []byte("# Problem\n\nREADME content from endpoint test")
	updateResp, err := s.client.UpdateProblemReadmeWithBodyWithResponse(
		s.ctx,
		problemID,
		"application/octet-stream",
		bytes.NewReader(readmeContent),
		func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		},
	)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateResp.StatusCode())

	getResp, err := s.client.GetProblemReadmeWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, getResp.StatusCode())
	s.Equal(string(readmeContent), string(getResp.Body))
}
