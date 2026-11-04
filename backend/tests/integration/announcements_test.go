//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListContestAnnouncements() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("ann_owner_"+suffix, accessmodel.UserRoleUser)
	outsider := s.createUser("ann_outsider_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("ann-org-"+suffix, "Announcements", owner.Id)
	contestID, otherContestID := uuid.New(), uuid.New()
	for login, id := range map[string]uuid.UUID{"ann-public": contestID, "ann-private": otherContestID} {
		visibility := contestsmodel.ContestVisibilityPublic
		if id == otherContestID {
			visibility = contestsmodel.ContestVisibilityPrivate
		}
		s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
			ID: id, OrganizationID: org.ID, OwnerID: &owner.Id, Title: login, Login: login,
			Visibility: visibility, Settings: map[string]interface{}{},
		}))
	}
	path := "/organizations/" + org.Login + "/contests/ann-public/announcements"
	get := func(path string, userID uuid.UUID, status int) map[string]json.RawMessage {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if userID != uuid.Nil {
			req.Header.Set("X-Test-User-ID", userID.String())
		}
		recorder := httptest.NewRecorder()
		s.handler.ServeHTTP(recorder, req)
		s.Require().Equal(status, recorder.Code, recorder.Body.String())
		var payload map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload
	}
	assertPage := func(query string, page, total int32, ids []uuid.UUID) {
		payload := get(path+query, uuid.Nil, http.StatusOK)
		var pagination corev1.PaginationModel
		s.Require().NoError(json.Unmarshal(payload["pagination"], &pagination))
		s.Equal(corev1.PaginationModel{Page: page, Total: total}, pagination)
		var items []corev1.ContestAnnouncementModel
		s.Require().NoError(json.Unmarshal(payload["announcements"], &items))
		s.Require().Len(items, len(ids))
		for i, id := range ids {
			s.Equal(id, items[i].ID)
		}
		if len(ids) == 0 {
			s.JSONEq("[]", string(payload["announcements"]))
		}
	}
	assertPage("", 1, 0, nil)

	// Include nullable joins, an unlinked problem and a valid zero UUID with an empty title.
	problemIDs := []*uuid.UUID{nil}
	for i, ordinal := range []int32{0, 25, 26, 27, 28} {
		id := uuid.New()
		title := fmt.Sprintf("Problem %d", i)
		if i == 4 {
			id, title = uuid.Nil, ""
		}
		_, err := s.dbPool.Exec(s.ctx,
			"INSERT INTO problems (id, organization_id, owner_id, visibility, title, short_name) VALUES ($1,$2,$3,$4,$5,$6)",
			id, org.ID, owner.Id, problemsmodel.ProblemVisibilityPrivate, title, fmt.Sprintf("ann-problem-%d", i))
		s.Require().NoError(err)
		if i < 3 {
			packageID := uuid.New()
			_, err = s.dbPool.Exec(s.ctx,
				"INSERT INTO problem_packages (id, problem_id, organization_id, package_hash, status, version) VALUES ($1,$2,$3,$4,'ready',1)",
				packageID, id, org.ID, fmt.Sprintf("%064d", i+1))
			s.Require().NoError(err)
			_, err = s.dbPool.Exec(s.ctx,
				"INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal) VALUES ($1,$2,$3,$4)",
				contestID, id, packageID, ordinal)
			s.Require().NoError(err)
		}
		problemIDs = append(problemIDs, &id)
	}
	ids := make([]uuid.UUID, len(problemIDs))
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, problemID := range problemIDs {
		ids[i] = uuid.New()
		_, err := s.dbPool.Exec(s.ctx,
			"INSERT INTO contest_announcements (id, contest_id, problem_id, author_id, title, body, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)",
			ids[i], contestID, problemID, owner.Id, fmt.Sprintf("Announcement %d", i), "**Body**", createdAt.Add(time.Duration(i)*time.Minute), createdAt.Add(time.Hour))
		s.Require().NoError(err)
	}
	_, err := s.dbPool.Exec(s.ctx,
		"INSERT INTO contest_announcements (contest_id, author_id, title, body) VALUES ($1,$2,'Other contest','Hidden')", otherContestID, owner.Id)
	s.Require().NoError(err)

	assertPage("", 1, 1, []uuid.UUID{ids[5], ids[4], ids[3], ids[2], ids[1], ids[0]})
	assertPage("?page=1&pageSize=2", 1, 3, []uuid.UUID{ids[5], ids[4]})
	assertPage("?page=2&pageSize=2", 2, 3, []uuid.UUID{ids[3], ids[2]})
	assertPage("?page=3&pageSize=2", 3, 3, []uuid.UUID{ids[1], ids[0]})
	assertPage("?page=4&pageSize=2", 4, 3, nil)
	assertPage("?page=2&pageSize=4", 2, 2, []uuid.UUID{ids[1], ids[0]})
	assertPage("?pageSize=1", 1, 6, []uuid.UUID{ids[5]})
	assertPage("?pageSize=100", 1, 1, []uuid.UUID{ids[5], ids[4], ids[3], ids[2], ids[1], ids[0]})

	payload := get(path, owner.Id, http.StatusOK)
	var items []map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(payload["announcements"], &items))
	for i, item := range items {
		index := len(ids) - 1 - i
		s.JSONEq(fmt.Sprintf("%q", contestID), string(item["contest_id"]))
		s.JSONEq(fmt.Sprintf("%q", owner.Id), string(item["author_id"]))
		s.JSONEq(fmt.Sprintf("%q", owner.Username), string(item["author_username"]))
		s.JSONEq(fmt.Sprintf("%q", fmt.Sprintf("Announcement %d", index)), string(item["title"]))
		s.JSONEq(`"**Body**"`, string(item["body"]))
		var actualCreatedAt, actualUpdatedAt time.Time
		s.Require().NoError(json.Unmarshal(item["created_at"], &actualCreatedAt))
		s.Require().NoError(json.Unmarshal(item["updated_at"], &actualUpdatedAt))
		s.True(actualCreatedAt.Equal(createdAt.Add(time.Duration(index) * time.Minute)))
		s.True(actualUpdatedAt.Equal(createdAt.Add(time.Hour)))
		if index == 0 || index == 5 {
			s.NotContains(item, "problem_id")
		} else {
			s.JSONEq(fmt.Sprintf("%q", *problemIDs[index]), string(item["problem_id"]))
		}
		if index == 0 {
			s.NotContains(item, "problem_title")
		} else {
			title := fmt.Sprintf("Problem %d", index-1)
			if index == 5 {
				title = ""
			}
			s.JSONEq(fmt.Sprintf("%q", title), string(item["problem_title"]))
		}
		if index >= 1 && index <= 3 {
			s.JSONEq(fmt.Sprintf("%q", []string{"A", "Z", "AA"}[index-1]), string(item["problem_letter"]))
		} else {
			s.NotContains(item, "problem_letter")
		}
	}

	for _, query := range []string{"?page=0", "?page=-1", "?pageSize=0", "?pageSize=101"} {
		get(path+query, owner.Id, http.StatusBadRequest)
	}
	privatePath := "/organizations/" + org.Login + "/contests/ann-private/announcements"
	get(privatePath, uuid.Nil, http.StatusUnauthorized)
	get(privatePath, outsider.Id, http.StatusForbidden)
	get(privatePath, owner.Id, http.StatusOK)
	get(path, outsider.Id, http.StatusOK)
	get("/organizations/"+org.Login+"/contests/missing/announcements", owner.Id, http.StatusNotFound)
	get("/organizations/missing/contests/ann-public/announcements", owner.Id, http.StatusNotFound)

	// Cross the default page size to distinguish the default of 50 from other limits.
	_, err = s.dbPool.Exec(s.ctx, `
		INSERT INTO contest_announcements (contest_id, author_id, title, body)
		SELECT $1, $2, 'Extra announcement', 'Body' FROM generate_series(1, 45)`, contestID, owner.Id)
	s.Require().NoError(err)
	payload = get(path, uuid.Nil, http.StatusOK)
	s.Require().NoError(json.Unmarshal(payload["announcements"], &items))
	s.Len(items, 50)
	var pagination corev1.PaginationModel
	s.Require().NoError(json.Unmarshal(payload["pagination"], &pagination))
	s.Equal(corev1.PaginationModel{Page: 1, Total: 2}, pagination)

	reader := pgstore.NewAnnouncementsRepo(s.dbPool)
	protected := contests.NewCommunicationReader(reader, nil, nil)
	for _, params := range [][2]int32{{0, 1}, {-3, 2}, {1, 0}, {1, -1}, {1, 101}, {2, 0}} {
		_, err := protected.ListAnnouncements(s.ctx, contestID, params[0], params[1])
		s.Require().ErrorIs(err, pkg.ErrBadInput)
	}

	for _, tc := range []struct {
		page, pageSize, wantPage, wantTotal int32
		wantLength                          int
	}{
		{1, 50, 1, 2, 50},
		{2, 50, 2, 2, 1},
		{1, 100, 1, 1, 51},
		{3, 50, 3, 2, 0},
	} {
		s.Run(fmt.Sprintf("reader_page_%d_size_%d", tc.page, tc.pageSize), func() {
			response, err := reader.ListAnnouncements(s.ctx, contestID, tc.page, tc.pageSize)
			s.Require().NoError(err)
			s.Equal(paging.Pagination{Page: tc.wantPage, Total: tc.wantTotal}, response.Pagination)
			s.Len(response.Announcements, tc.wantLength)
			s.NotNil(response.Announcements)
		})
	}

}
