//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) readJSON(path string, userID uuid.UUID, status int) map[string]json.RawMessage {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if userID != uuid.Nil {
		req.Header.Set("X-Test-User-ID", userID.String())
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	s.Require().Equal(status, rec.Code, rec.Body.String())
	var result map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &result))
	return result
}

func (s *IntegrationTestSuite) readItems(path string, userID uuid.UUID, key string, page, total int) []map[string]json.RawMessage {
	payload := s.readJSON(path, userID, http.StatusOK)
	s.JSONEq(fmt.Sprintf(`{"page":%d,"total":%d}`, page, total), string(payload["pagination"]))
	var items []map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(payload[key], &items))
	if len(items) == 0 {
		s.JSONEq("[]", string(payload[key]))
	}
	return items
}

func (s *IntegrationTestSuite) TestReadQueries() {
	_, cleanupErr := s.dbPool.Exec(s.ctx, "TRUNCATE TABLE users, posts CASCADE")
	s.Require().NoError(cleanupErr)
	suffix := uuid.NewString()[:8]
	owner := s.createUser("read_owner_"+suffix, accessmodel.UserRoleAdmin)
	participant := s.createUser("read_user_"+suffix, accessmodel.UserRoleUser)
	other := s.createUser("read_other_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("read-org-"+suffix, "Reads", owner.Id)
	contestID := uuid.New()
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID: contestID, OrganizationID: org.ID, OwnerID: &owner.Id, Title: "Read contest", Login: "read",
		Visibility: contestsmodel.ContestVisibilityPublic, Settings: map[string]interface{}{},
	}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: participant.Id, Role: "participant"}))
	base := "/organizations/" + org.Login + "/contests/read"
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.Run("users and claimed accounts", func() {
		_, err := s.dbPool.Exec(s.ctx, "UPDATE users SET email='', avatar_url=$2, claimed_by_user_id=$3, claimed_at=$4, expires_at=$5 WHERE id=$1", participant.Id, uuid.Nil.String(), owner.Id, now, now.AddDate(10, 0, 0))
		s.Require().NoError(err)
		items := s.readItems("/users?search=read_user_"+suffix+"&role=user&pageSize=1", owner.Id, "users", 1, 2)
		s.Require().Len(items, 1)
		s.JSONEq(`""`, string(items[0]["email"]))
		s.JSONEq(`"`+uuid.Nil.String()+`"`, string(items[0]["imgId"]))
		s.NotContains(items[0], "password_hash")
		profile := s.readJSON("/users/@"+participant.Username, owner.Id, 200)
		var profileUser map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(profile["user"], &profileUser))
		s.Equal(items[0], profileUser)
		s.readJSON("/users/missing-user", owner.Id, 404)
		s.NotEmpty(items[0]["expires_at"])
		s.NotEmpty(items[0]["claimed_at"])
		s.NotEmpty(items[0]["is_email_verified"])
		s.Empty(s.readItems("/users?search=read_user_"+suffix+"&page=99&pageSize=1", owner.Id, "users", 99, 3))
		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET email=NULL WHERE id=$1", other.Id)
		s.Require().NoError(err)
		users := s.readItems("/users?pageSize=20&search=read_other_"+suffix, owner.Id, "users", 1, 1)
		for _, item := range users {
			if string(item["id"]) == `"`+other.Id.String()+`"` {
				items = []map[string]json.RawMessage{item}
				break
			}
		}
		s.Require().Len(items, 1)
		for _, field := range []string{"email", "imgId", "expires_at", "claimed_by_user_id", "claimed_at"} {
			s.NotContains(items[0], field)
		}
		payload := s.readJSON("/users/me/claimed-accounts", owner.Id, 200)
		var accounts []map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(payload["accounts"], &accounts))
		s.Require().Len(accounts, 1)
		s.JSONEq(`"`+participant.Id.String()+`"`, string(accounts[0]["id"]))
		s.NotEmpty(accounts[0]["expires_at"])
		s.readJSON("/users/me/claimed-accounts", uuid.Nil, 401)
		s.readJSON("/users?pageSize=1", participant.Id, 200)
		s.readJSON("/users", owner.Id, 200)
	})

	s.Run("posts", func() {
		ids := make([]uuid.UUID, 3)
		for i := range ids {
			ids[i] = uuid.New()
			_, err := s.dbPool.Exec(s.ctx, "INSERT INTO posts(id,title,text,description,author_uuid,author_name,image_key,created_at) VALUES($1,$2,'text','description',$3,$4,'preview',$5)", ids[i], fmt.Sprintf("Post %d", i), owner.Id, owner.Username, now.Add(time.Duration(i)*time.Hour))
			s.Require().NoError(err)
		}
		for _, tc := range []struct {
			query string
			index int
		}{{"?pageSize=1&sort_order=asc", 0}, {"?pageSize=1&sort_order=desc", 2}, {"?page=2&pageSize=1&sort_order=asc", 1}} {
			page := 1
			if tc.index == 1 {
				page = 2
			}
			items := s.readItems("/posts"+tc.query, uuid.Nil, "posts", page, 3)
			s.Require().Len(items, 1)
			s.JSONEq(`"`+ids[tc.index].String()+`"`, string(items[0]["id"]))
			s.JSONEq(`"preview"`, string(items[0]["preview_image_id"]))
			s.JSONEq(`"text"`, string(items[0]["text"]))
		}
		s.Empty(s.readItems("/posts?page=4&pageSize=1", uuid.Nil, "posts", 4, 3))
		post := s.readJSON("/posts/"+ids[0].String(), uuid.Nil, 200)
		s.JSONEq(`"Post 0"`, string(post["title"]))
		s.JSONEq(`"`+owner.Id.String()+`"`, string(post["author_id"]))
		s.readJSON("/posts/"+uuid.NewString(), uuid.Nil, 404)
	})

	problemID := uuid.New()
	_, err := s.dbPool.Exec(s.ctx, "INSERT INTO problems(id,organization_id,owner_id,title,short_name,visibility,is_template,time_limit_ms,memory_limit_mb) VALUES($1,$2,$3,'Read problem','read','private',true,1234,256)", problemID, org.ID, owner.Id)
	s.Require().NoError(err)
	s.Run("problem catalog and dashboard", func() {
		query := "/problems?organization_id=" + org.ID.String() + "&search=Read&is_template=true"
		items := s.readItems(query, owner.Id, "problems", 1, 1)
		s.Require().Len(items, 1)
		s.JSONEq(`"`+problemID.String()+`"`, string(items[0]["id"]))
		s.JSONEq(`1234`, string(items[0]["time_limit"]))
		s.Empty(s.readItems(query, other.Id, "problems", 1, 0))
		s.Empty(s.readItems(query+"&owner=true", other.Id, "problems", 1, 0))
		dashboard := s.readJSON("/users/me/dashboard", owner.Id, 200)
		var problems []map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(dashboard["my_problems"], &problems))
		s.Require().Len(problems, 1)
		s.JSONEq(`"`+problemID.String()+`"`, string(problems[0]["id"]))
		s.JSONEq(`256`, string(problems[0]["memory_limit"]))
		dashboard = s.readJSON("/users/me/dashboard", participant.Id, 200)
		var contests []map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(dashboard["recent_contests"], &contests))
		s.Require().Len(contests, 1)
		for _, field := range []string{"start_time", "end_time", "last_submission_time"} {
			s.NotContains(contests[0], field)
		}
	})

	s.Run("drafts and contest members", func() {
		repo := pgstore.NewDraftsRepo(s.dbPool)
		for i := 0; i < 3; i++ {
			id, err := repo.CreateDraft(s.ctx, &contestsmodel.ContestDraftCreation{ContestID: contestID, UserID: participant.Id, Code: fmt.Sprintf("code%d", i)})
			s.Require().NoError(err)
			_, err = s.dbPool.Exec(s.ctx, "UPDATE contest_drafts SET created_at=$2 WHERE id=$1", id, now.Add(time.Duration(i)*time.Hour))
			s.Require().NoError(err)
		}
		_, err := repo.CreateDraft(s.ctx, &contestsmodel.ContestDraftCreation{ContestID: contestID, UserID: other.Id, Code: "hidden"})
		s.Require().NoError(err)
		items := s.readItems(base+"/drafts?page=2&pageSize=1", participant.Id, "drafts", 2, 3) // total is row count here
		s.Require().Len(items, 1)
		s.JSONEq(`"code1"`, string(items[0]["code"]))
		s.JSONEq(`"`+participant.Username+`"`, string(items[0]["username"]))
		s.Empty(s.readItems(base+"/drafts?page=4&pageSize=1", participant.Id, "drafts", 4, 3))
		members := s.readItems(base+"/members?page=2&pageSize=1", owner.Id, "members", 2, 1)
		s.Require().Len(members, 1) // query intentionally returns all members, regardless of page
		s.JSONEq(`"participant"`, string(members[0]["contest_role"]))
	})

	s.Run("clarifications", func() {
		repo := pgstore.NewClarificationsRepo(s.dbPool)
		first, err := repo.CreateClarification(s.ctx, &contestsmodel.CreateContestClarificationInput{ContestID: contestID, UserID: participant.Id, Question: "First?"})
		s.Require().NoError(err)
		second, err := repo.CreateClarification(s.ctx, &contestsmodel.CreateContestClarificationInput{ContestID: contestID, ProblemID: &problemID, UserID: other.Id, Question: "Other?"})
		s.Require().NoError(err)
		_, err = repo.AnswerClarification(s.ctx, second.ID, contestID, owner.Id, "Answer")
		s.Require().NoError(err)
		items := s.readItems(base+"/clarifications?status=answered&problem_id="+problemID.String(), participant.Id, "clarifications", 1, 1)
		s.Require().Len(items, 1) // participant filters are intentionally ignored
		s.JSONEq(`"`+first.ID.String()+`"`, string(items[0]["id"]))
		for _, field := range []string{"problem_id", "problem_title", "problem_letter", "answer", "answered_by", "answered_at", "answered_by_username"} {
			s.NotContains(items[0], field)
		}
		items = s.readItems(base+"/clarifications?status=answered&problem_id="+problemID.String(), owner.Id, "clarifications", 1, 1)
		s.Require().Len(items, 1)
		s.JSONEq(`"Answer"`, string(items[0]["answer"]))
		s.JSONEq(`"Read problem"`, string(items[0]["problem_title"]))
		s.NotContains(items[0], "problem_letter")
		s.NotEmpty(items[0]["answered_at"])
		s.Empty(s.readItems(base+"/clarifications?status=pending&page=2&pageSize=1", owner.Id, "clarifications", 2, 1))
		s.readJSON(base+"/clarifications", uuid.Nil, 401)
	})

	s.Run("notifications", func() {
		repo := pgstore.NewNotificationsRepo(s.dbPool)
		for i := 0; i < 3; i++ {
			var link *string
			var data map[string]interface{}
			if i == 1 {
				empty := ""
				link = &empty
				data = map[string]interface{}{"number": 1, "null": nil, "nested": map[string]interface{}{"ok": true}}
			}
			n, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: participant.Id, Type: notificationsmodel.NotificationTypeSystem, Title: fmt.Sprintf("Notification %d", i), Link: link, Data: data})
			s.Require().NoError(err)
			_, err = s.dbPool.Exec(s.ctx, "UPDATE notifications SET created_at=$2 WHERE id=$1", n.ID, now.Add(time.Duration(i)*time.Hour))
			s.Require().NoError(err)
			if i == 2 {
				s.Require().NoError(repo.MarkAsRead(s.ctx, n.ID, participant.Id))
			}
		}
		_, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: other.Id, Type: notificationsmodel.NotificationTypeSystem, Title: "Hidden"})
		s.Require().NoError(err)
		items := s.readItems("/notifications?unread_only=true&pageSize=1", participant.Id, "notifications", 1, 2)
		s.Require().Len(items, 1)
		s.JSONEq(`"Notification 1"`, string(items[0]["title"]))
		s.JSONEq(`""`, string(items[0]["link"]))
		s.JSONEq(`{"number":1,"null":null,"nested":{"ok":true}}`, string(items[0]["data"]))
		items = s.readItems("/notifications?unread_only=true&page=2&pageSize=1", participant.Id, "notifications", 2, 2)
		s.Require().Len(items, 1)
		s.NotContains(items[0], "link")
		s.NotContains(items[0], "data")
		s.Empty(s.readItems("/notifications?page=4&pageSize=1", participant.Id, "notifications", 4, 3))
		s.JSONEq(`2`, string(s.readJSON("/notifications/unread-count", participant.Id, 200)["count"]))
		s.JSONEq(`1`, string(s.readJSON("/notifications/unread-count", other.Id, 200)["count"]))
		s.readJSON("/notifications", uuid.Nil, 401)
		s.readJSON("/notifications/unread-count", uuid.Nil, 401)
	})
}

func (s *IntegrationTestSuite) TestContestReadQueries() {
	suffix := uuid.NewString()[:8]
	admin := s.createUser("catalog_admin_"+suffix, accessmodel.UserRoleAdmin)
	member := s.createUser("catalog_user_"+suffix, accessmodel.UserRoleUser)
	outsider := s.createUser("catalog_other_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("catalog-"+suffix, "Catalog", admin.Id)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		visibility := contestsmodel.ContestVisibilityPublic
		if i == 1 {
			visibility = contestsmodel.ContestVisibilityPrivate
		}
		s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{ID: id, OrganizationID: org.ID, OwnerID: &admin.Id, Title: fmt.Sprintf("Catalog %d", i), Login: fmt.Sprintf("catalog-%d", i), Visibility: visibility, Settings: map[string]interface{}{"hide_statements": true, "freeze_duration_minutes": int32(15)}}))
		s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: id, UserId: member.Id, Role: "participant"}))
		_, err := s.dbPool.Exec(s.ctx, "UPDATE contests SET created_at=$2 WHERE id=$1", id, time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC))
		s.Require().NoError(err)
	}
	for _, tc := range []struct {
		path  string
		user  uuid.UUID
		total int
	}{
		{"/public/contests?pageSize=1", uuid.Nil, 2},
		{"/admin/contests?pageSize=1", admin.Id, 3},
		{"/organizations/" + org.Login + "/contests?pageSize=1", admin.Id, 3},
		{"/users/" + member.Username + "/contests?pageSize=1", member.Id, 1},
		{"/workshop/contests?pageSize=1", member.Id, 1},
		{"/workshop/contests?pageSize=1&organization_id=" + org.ID.String(), member.Id, 1},
	} {
		s.Run(tc.path, func() {
			items := s.readItems(tc.path, tc.user, "contests", 1, tc.total)
			s.Require().Len(items, 1)
			s.JSONEq(`"`+ids[2].String()+`"`, string(items[0]["id"]))
			s.JSONEq(`true`, string(items[0]["hide_statements"]))
			for _, field := range []string{"start_time", "end_time", "owner"} {
				s.NotContains(items[0], field)
			}
		})
	}
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Contest readers", Slug: "contest-readers", Privacy: organizationsmodel.TeamPrivacyClosed})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.CreateContestTeam(s.ctx, ids[0], team.ID, contestsmodel.ContestRoleParticipant))
	teams := s.readJSON("/organizations/"+org.Login+"/contests/catalog-0/teams", admin.Id, 200)
	var contestTeams []map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(teams["teams"], &contestTeams))
	s.Require().Len(contestTeams, 1)
	s.JSONEq(`"Contest readers"`, string(contestTeams[0]["team_name"]))
	s.NotContains(contestTeams[0], "permissions_mask")

	s.Empty(s.readItems("/public/contests?search=NoSuchContest", uuid.Nil, "contests", 1, 0))
	s.Empty(s.readItems("/public/contests?page=3&pageSize=1", uuid.Nil, "contests", 3, 2))
	s.Empty(s.readItems("/users/"+member.Username+"/contests?page=4&pageSize=1", member.Id, "contests", 4, 0))
	s.readJSON("/admin/contests", outsider.Id, 403)
	s.readJSON("/organizations/missing/contests", admin.Id, 404)
	s.readJSON("/users/missing-user/contests", admin.Id, 404)
}

func (s *IntegrationTestSuite) TestProblemAccessReadQueries() {
	suffix := uuid.NewString()[:8]
	admin := s.createUser("access_admin_"+suffix, accessmodel.UserRoleAdmin)
	other := s.createUser("access_other_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("access-"+suffix, "Access", admin.Id)
	problemID := uuid.New()
	s.Require().NoError(s.problemsRepo.CreateProblem(s.ctx, &problemsmodel.CreateProblemParams{ID: problemID, OrganizationID: org.ID, OwnerID: &admin.Id, Title: "Private", ShortName: "access", Visibility: problemsmodel.ProblemVisibilityPrivate}))
	s.Require().NoError(s.problemsRepo.CreateProblemMember(s.ctx, &problemsmodel.CreateProblemMemberParams{ProblemID: problemID, UserID: admin.Id, Role: problemsmodel.ProblemRoleOwner}))
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Readers", Slug: "readers", Privacy: organizationsmodel.TeamPrivacyClosed})
	s.Require().NoError(err)
	s.Require().NoError(s.problemsRepo.AddProblemTeam(s.ctx, problemID, team.ID, problemsmodel.ProblemPermission("read")))
	base := "/problems/" + problemID.String()
	members := s.readItems(base+"/members?page=2&pageSize=1", admin.Id, "members", 2, 1)
	s.Require().Len(members, 1)
	s.JSONEq(`"owner"`, string(members[0]["role"]))
	teams := s.readJSON(base+"/teams", admin.Id, 200)
	var items []map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(teams["teams"], &items))
	s.Require().Len(items, 1)
	s.JSONEq(`"Readers"`, string(items[0]["team_name"]))
	s.JSONEq(`"read"`, string(items[0]["permission"]))
	packagesRepo := pgstore.NewPackagesRepo(s.dbPool)
	first, err := packagesRepo.CreatePackage(s.ctx, &problemsmodel.CreatePackageParams{ID: uuid.New(), ProblemID: problemID, OrganizationID: org.ID, PackageHash: fmt.Sprintf("%064d", 1), Status: "pending"})
	s.Require().NoError(err)
	second, err := packagesRepo.CreatePackage(s.ctx, &problemsmodel.CreatePackageParams{ID: uuid.New(), ProblemID: problemID, OrganizationID: org.ID, PackageHash: fmt.Sprintf("%064d", 2), Status: "ready"})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE problem_packages SET compiled_at=$2 WHERE id=$1", second.ID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s.Require().NoError(err)
	packages := s.readJSON(base+"/packages", admin.Id, 200)
	var versions []map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(packages["packages"], &versions))
	s.Require().Len(versions, 2)
	s.JSONEq(`"`+second.ID.String()+`"`, string(versions[0]["id"]))
	s.NotEmpty(versions[0]["compiled_at"])
	s.JSONEq(`"`+first.ID.String()+`"`, string(versions[1]["id"]))
	s.NotContains(versions[1], "compiled_at")
	for _, suffix := range []string{"/members", "/teams", "/packages"} {
		s.readJSON(base+suffix, uuid.Nil, 401)
		s.readJSON(base+suffix, other.Id, 403)
		s.readJSON("/problems/"+uuid.NewString()+suffix, admin.Id, 404)
	}
}

func (s *IntegrationTestSuite) TestClarificationReadOptionalFields() {
	suffix := uuid.NewString()[:8]
	admin := s.createUser("optional_admin_"+suffix, accessmodel.UserRoleAdmin)
	org := s.createOrganization("optional-"+suffix, "Optional fields", admin.Id)
	contestID := uuid.New()
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, OwnerID: &admin.Id, Title: "Optional", Login: "optional", Visibility: contestsmodel.ContestVisibilityPublic, Settings: map[string]interface{}{}}))
	for i, ordinal := range []int32{0, 25, 26} {
		problemID, packageID := uuid.New(), uuid.New()
		_, err := s.dbPool.Exec(s.ctx, "INSERT INTO problems(id,organization_id,owner_id,title,short_name,visibility) VALUES($1,$2,$3,'',$4,'private')", problemID, org.ID, admin.Id, fmt.Sprintf("optional-%d", i))
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "INSERT INTO problem_packages(id,problem_id,organization_id,package_hash,status,version) VALUES($1,$2,$3,$4,'ready',1)", packageID, problemID, org.ID, fmt.Sprintf("%064d", i+1))
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "INSERT INTO contest_problems(contest_id,problem_id,package_id,ordinal) VALUES($1,$2,$3,$4)", contestID, problemID, packageID, ordinal)
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "INSERT INTO contest_clarifications(id,contest_id,problem_id,user_id,question,answer,status) VALUES($1,$2,$3,$4,'Question','','answered')", uuid.New(), contestID, problemID, admin.Id)
		s.Require().NoError(err)
		items := s.readItems("/organizations/"+org.Login+"/contests/optional/clarifications?problem_id="+problemID.String(), admin.Id, "clarifications", 1, 1)
		s.Require().Len(items, 1)
		s.JSONEq(`""`, string(items[0]["problem_title"]))
		s.JSONEq(`""`, string(items[0]["answer"]))
		s.JSONEq(`"`+[]string{"A", "Z", "AA"}[i]+`"`, string(items[0]["problem_letter"]))
		for _, field := range []string{"answered_by", "answered_by_username", "answered_at"} {
			s.NotContains(items[0], field)
		}
	}
}
