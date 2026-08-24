//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/go-faster/jx"
)

func (s *IntegrationTestSuite) TestWorkshopStatementEndpointSyncsProblemTitle() {
	admin := s.createUser("workshop_statement_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-statement-org", "Workshop Statement Org", admin.Id)

	createResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title:          "Original Workshop Title",
		OrganizationID: corev1.NewOptUUID(org.ID),
		TemplateID:     "builtin:a-plus-b",
	})
	s.Require().NoError(err)
	s.Require().NotNil(createResp)

	problemID := createResp.ID

	newTitle := "Synced From Statement Endpoint"
	updateResp, err := s.client.UpdateProblemStatement(withTestUser(s.ctx, admin.Id), &corev1.UpdateProblemStatementRequest{
		Title: corev1.NewOptString(newTitle),
	}, corev1.UpdateProblemStatementParams{
		ProblemId: problemID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(updateResp)
	s.Equal(newTitle, updateResp.Title)

	problemResp, err := s.client.GetProblem(withTestUser(s.ctx, admin.Id), corev1.GetProblemParams{
		ID: problemID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(problemResp)
	s.Equal(newTitle, problemResp.Problem.Title)
}

func (s *IntegrationTestSuite) TestWorkshopCheckerEndpointsCRUD() {
	admin := s.createUser("workshop_checker_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-checker-org", "Workshop Checker Org", admin.Id)

	createResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title:          "Checker CRUD Problem",
		OrganizationID: corev1.NewOptUUID(org.ID),
		TemplateID:     "builtin:a-plus-b",
	})
	s.Require().NoError(err)
	s.Require().NotNil(createResp)

	problemID := createResp.ID

	// Delete initial checker created by the builtin template
	_, err = s.client.DeleteProblemChecker(withTestUser(s.ctx, admin.Id), corev1.DeleteProblemCheckerParams{
		ProblemId: problemID,
		Name:      "checker.cpp",
	})
	s.Require().NoError(err)

	checkerSource := []byte("int main(){return 0;}")
	_, err = s.client.CreateProblemChecker(
		withTestUser(s.ctx, admin.Id),
		corev1.CreateProblemCheckerReq{Data: bytes.NewReader(checkerSource)},
		corev1.CreateProblemCheckerParams{
			ProblemId: problemID,
			Name:      "checker.cpp",
		},
	)
	s.Require().NoError(err)

	listResp, err := s.client.ListProblemCheckers(withTestUser(s.ctx, admin.Id), corev1.ListProblemCheckersParams{
		ProblemId: problemID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(listResp)
	s.Len(listResp.Files, 1)
	s.Equal("checkers/checker.cpp", listResp.Files[0].Path.Value)

	getResp, err := s.client.GetProblemChecker(withTestUser(s.ctx, admin.Id), corev1.GetProblemCheckerParams{
		ProblemId: problemID,
		Name:      "checker.cpp",
	})
	s.Require().NoError(err)
	s.Require().NotNil(getResp)
	data, err := io.ReadAll(getResp.Data)
	s.Require().NoError(err)
	s.Equal(string(checkerSource), string(data))

	updatedCheckerSource := []byte("int main(){return 1;}")
	_, err = s.client.UpdateProblemChecker(
		withTestUser(s.ctx, admin.Id),
		corev1.UpdateProblemCheckerReq{Data: bytes.NewReader(updatedCheckerSource)},
		corev1.UpdateProblemCheckerParams{
			ProblemId: problemID,
			Name:      "checker.cpp",
		},
	)
	s.Require().NoError(err)

	_, err = s.client.DeleteProblemChecker(withTestUser(s.ctx, admin.Id), corev1.DeleteProblemCheckerParams{
		ProblemId: problemID,
		Name:      "checker.cpp",
	})
	s.Require().NoError(err)

	listAfterDeleteResp, err := s.client.ListProblemCheckers(withTestUser(s.ctx, admin.Id), corev1.ListProblemCheckersParams{
		ProblemId: problemID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(listAfterDeleteResp)
	s.Len(listAfterDeleteResp.Files, 0)
}

func (s *IntegrationTestSuite) TestWorkshopTestsConfigEndpoint() {
	admin := s.createUser("workshop_tests_config_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-tests-config-org", "Workshop Tests Config Org", admin.Id)

	createResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title:          "Tests Config Problem",
		OrganizationID: corev1.NewOptUUID(org.ID),
		TemplateID:     "builtin:a-plus-b",
	})
	s.Require().NoError(err)
	s.Require().NotNil(createResp)

	problemID := createResp.ID

	groupsBytes, err := json.Marshal([]map[string]interface{}{
		{
			"ordinal":       0,
			"name":          "Updated Samples",
			"points":        0,
			"points_policy": "complete-group",
			"depends_on":    []int{},
			"tests":         [2]int{1, 1},
		},
	})
	s.Require().NoError(err)

	testsBytes, err := json.Marshal([]map[string]interface{}{
		{
			"ordinal":   1,
			"method":    "manual",
			"generator": nil,
			"is_sample": true,
		},
	})
	s.Require().NoError(err)

	testsConfig := corev1.UpdateProblemTestsConfigRequest{
		"groups": jx.Raw(groupsBytes),
		"tests":  jx.Raw(testsBytes),
	}

	_, err = s.client.UpdateProblemTestsConfig(withTestUser(s.ctx, admin.Id), testsConfig, corev1.UpdateProblemTestsConfigParams{
		ProblemId: problemID,
	})
	s.Require().NoError(err)

	getResp, err := s.client.GetProblemTestFile(withTestUser(s.ctx, admin.Id), corev1.GetProblemTestFileParams{
		ProblemId: problemID,
		Name:      "tests.json",
	})
	s.Require().NoError(err)
	s.Require().NotNil(getResp)

	bodyBytes, err := io.ReadAll(getResp.Data)
	s.Require().NoError(err)

	var parsed map[string]interface{}
	err = json.Unmarshal(bodyBytes, &parsed)
	s.Require().NoError(err)

	groupsRaw, ok := parsed["groups"].([]interface{})
	s.Require().True(ok)
	s.Require().NotEmpty(groupsRaw)

	group0, ok := groupsRaw[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("Updated Samples", group0["name"])

	testsRaw, ok := parsed["tests"].([]interface{})
	s.Require().True(ok)
	s.Require().NotEmpty(testsRaw)

	test0, ok := testsRaw[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal(true, test0["is_sample"])
}

