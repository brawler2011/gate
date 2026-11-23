//go:build integration
// +build integration

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Template database clones let top-level tests run in parallel.
var (
	templateOnce sync.Once
	adminDSN     string
	templateErr  error

	testDBsMu sync.Mutex
	testDBs   = map[string]string{} // top-level test name -> DSN of its database
)

const templateDatabase = "gate_template"

// Subtests share the top-level database; repeated runs with -count reuse it.
func testDSN(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	templateOnce.Do(func() { adminDSN, templateErr = startTemplate(ctx) })
	require.NoError(t, templateErr)

	root, _, _ := strings.Cut(t.Name(), "/")
	testDBsMu.Lock()
	defer testDBsMu.Unlock()
	if dsn, ok := testDBs[root]; ok {
		return dsn
	}
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	conn, err := pgx.Connect(ctx, adminDSN)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateDatabase)
	require.NoError(t, errors.Join(err, conn.Close(ctx)))
	dsn, err := withDatabase(adminDSN, name)
	require.NoError(t, err)
	testDBs[root] = dsn
	return dsn
}

func startTemplate(ctx context.Context) (string, error) {
	container, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase(templateDatabase),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		// Every parallel test holds its own pools; durability is irrelevant for throwaway data.
		testcontainers.WithCmd("postgres", "-c", "max_connections=1000", "-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return "", fmt.Errorf("start postgres container: %w", err)
	}
	template, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("get container dsn: %w", err)
	}

	_, b, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(b), "../../../migrations")
	db, err := sql.Open("pgx", template)
	if err != nil {
		return "", fmt.Errorf("open sql db for migrations: %w", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return "", errors.Join(fmt.Errorf("set goose dialect: %w", err), db.Close())
	}
	if err := goose.Up(db, migrationsPath); err != nil {
		return "", errors.Join(fmt.Errorf("run goose migrations: %w", err), db.Close())
	}
	// A template database cannot be cloned while anyone is connected to it.
	if err := db.Close(); err != nil {
		return "", fmt.Errorf("close migration db: %w", err)
	}
	return withDatabase(template, "postgres")
}

func withDatabase(dsn, database string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/" + database
	return u.String(), nil
}

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pkg.NewPostgresDB(testDSN(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// helperCreateOrg also creates the organization's owner; helperOrgOwner returns it.
func helperCreateOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, login, name string) uuid.UUID {
	t.Helper()
	orgID := uuid.New()
	_, err := pool.Exec(ctx, `WITH u AS (INSERT INTO users (id, username, password_hash) VALUES ($4, $5, 'unused') RETURNING id),
o AS (INSERT INTO organizations (id, login, name) VALUES ($1, $2, $3) RETURNING id)
INSERT INTO organization_members (organization_id, user_id, role) SELECT o.id, u.id, 'owner' FROM o, u`,
		orgID, login, name, uuid.New(), "owner_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:20],
	)
	require.NoError(t, err)
	return orgID
}

func helperOrgOwner(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	var owner uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT user_id FROM organization_members WHERE organization_id = $1 AND role = 'owner' LIMIT 1`, org).Scan(&owner))
	return owner
}

func helperCreatePackage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, problemID, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	pkgID, err := dbfixtures.InsertPackage(ctx, pool, problemID, orgID, time.Time{})
	require.NoError(t, err)
	return pkgID
}

func TestUsersRepo_Integration(t *testing.T) {
	t.Parallel()
	pool := getTestPool(t)
	repo := postgres.NewUsersRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	username := "u_" + suffix
	email := "u_" + suffix + "@example.com"
	avatar := "https://example.com/avatar_" + suffix + ".png"
	exp := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	userID := uuid.New()

	err := repo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:              userID,
		Username:        username,
		Role:            accessmodel.UserRoleUser,
		PasswordHash:    "dummy_hash",
		Email:           &email,
		AvatarUrl:       &avatar,
		ExpiresAt:       &exp,
		IsEmailVerified: true,
	})
	require.NoError(t, err)

	user, err := repo.GetUserById(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, userID, user.Id)
	require.Equal(t, username, user.Username)
	require.Equal(t, accessmodel.UserRoleUser, user.Role)
	require.Equal(t, &email, user.Email)
	require.Equal(t, &avatar, user.AvatarUrl)

	byUsername, err := repo.GetUserByUsername(ctx, username)
	require.NoError(t, err)
	require.Equal(t, userID, byUsername.Id)

	// Duplicate usernames map to ErrConflict.
	err = repo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:           uuid.New(),
		Username:     username,
		Role:         accessmodel.UserRoleUser,
		PasswordHash: "dummy_hash",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrConflict), "expected ErrConflict on duplicate username, got: %v", err)

	// Missing users map to ErrNotFound.
	_, err = repo.GetUserById(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound for missing user, got: %v", err)

	newAvatar := "https://example.com/new_avatar.png"
	newEmail := "new_" + suffix + "@example.com"
	err = repo.UpdateUser(ctx, identitymodel.UpdateUserParams{
		Id:        userID,
		AvatarUrl: &newAvatar,
		Email:     &newEmail,
	})
	require.NoError(t, err)

	updatedUser, err := repo.GetUserById(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, &newAvatar, updatedUser.AvatarUrl)
	require.Equal(t, &newEmail, updatedUser.Email)

	adminRole := accessmodel.UserRoleAdmin
	err = repo.UpdateUser(ctx, identitymodel.UpdateUserParams{
		Id:   userID,
		Role: &adminRole,
	})
	require.NoError(t, err)
	adminUser, err := repo.GetUserById(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, accessmodel.UserRoleAdmin, adminUser.Role)
}

func TestProblemsRepo_Integration(t *testing.T) {
	t.Parallel()
	pool := getTestPool(t)
	repo := postgres.NewProblemsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	orgID := helperCreateOrg(t, ctx, pool, "org_prob_"+suffix, "Problem Org "+suffix)

	probID := uuid.New()
	title := "Test Problem " + suffix
	shortName := "p_" + suffix

	err := dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: probID, OrganizationID: orgID, Owner: helperOrgOwner(t, ctx, pool, orgID), Visibility: "public", Title: title, ShortName: shortName})
	require.NoError(t, err)

	prob, err := repo.GetProblemById(ctx, probID)
	require.NoError(t, err)
	require.Equal(t, probID, prob.ID)
	require.Equal(t, title, prob.Title)
	require.Equal(t, shortName, prob.ShortName)
	require.Equal(t, orgID, prob.OrganizationID)
	require.Equal(t, 1000, prob.TimeLimitMs)
	require.Equal(t, 256, prob.MemoryLimitMb)

	// Duplicate short names within an organization map to ErrConflict.
	err = repo.CreateProblem(ctx, &problemsmodel.CreateProblemParams{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Title:          "Another Title",
		ShortName:      shortName,
		Visibility:     problemsmodel.ProblemVisibilityPublic,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrConflict), "expected ErrConflict on duplicate short_name in org, got: %v", err)

	// Missing organizations map to ErrBadInput.
	err = repo.CreateProblem(ctx, &problemsmodel.CreateProblemParams{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Title:          "Orphan Problem",
		ShortName:      "orphan_" + suffix,
		Visibility:     problemsmodel.ProblemVisibilityPublic,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "violates foreign key constraint")

	// Missing problems map to ErrNotFound.
	_, err = repo.GetProblemById(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	list, err := repo.ListProblems(ctx, &problemsmodel.ProblemsFilter{
		OrganizationID: &orgID,
		Page:           1,
		PageSize:       10,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(list.Problems), 1)
	require.Equal(t, probID, list.Problems[0].ID)
}

func TestContestsRepo_Integration(t *testing.T) {
	t.Parallel()
	pool := getTestPool(t)
	repo := postgres.NewContestsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	orgID := helperCreateOrg(t, ctx, pool, "org_c_"+suffix, "Contest Org "+suffix)

	contestID := uuid.New()
	login := "contest_" + suffix
	title := "Contest Title " + suffix
	err := dbfixtures.CreateContest(ctx, pool, contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: orgID,
		Title:          title,
		Login:          login,
		Description:    "Description for contest",
		Visibility:     contestsmodel.ContestVisibilityPublic,
	}, helperOrgOwner(t, ctx, pool, orgID))
	require.NoError(t, err)

	c, err := repo.GetContest(ctx, contestID)
	require.NoError(t, err)
	require.Equal(t, contestID, c.ID)
	require.Equal(t, login, c.Login)
	require.Equal(t, title, c.Title)
	require.Equal(t, orgID, c.OrganizationID)
	require.Equal(t, contestsmodel.ContestSettings{
		JoinPolicy:  accessmodel.JoinOpen,
		Scoreboard:  contestsmodel.Phased[contestsmodel.ContestScope]{During: contestsmodel.ScopeParticipant, After: contestsmodel.ScopePublic},
		Submissions: contestsmodel.Phased[contestsmodel.ContestScope]{During: contestsmodel.ScopeModerator, After: contestsmodel.ScopeParticipant},
		Sources:     contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator),
		Feedback:     contestsmodel.Phased[contestsmodel.FeedbackLevel]{During: contestsmodel.FeedbackVerdict, After: contestsmodel.FeedbackFull},
		FreezeStatus: contestsmodel.FreezeStatusAuto, AllowClarifications: true, EnableDrafts: true, EnableUpsolving: true, Languages: []string{},
	}, c.Settings)

	// Duplicate contest logins map to ErrConflict.
	err = repo.CreateContest(ctx, &contestsmodel.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Title:          "Duplicate Login Contest",
		Login:          login,
		Visibility:     contestsmodel.ContestVisibilityPublic,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrConflict), "expected ErrConflict on duplicate contest login, got: %v", err)

	// Missing contest organizations map to ErrBadInput.
	err = dbfixtures.CreateContest(ctx, pool, contestsmodel.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Title:          "Orphan Contest",
		Login:          "orphan_" + suffix,
		Visibility:     contestsmodel.ContestVisibilityPublic,
	}, helperOrgOwner(t, ctx, pool, orgID))
	require.Error(t, err)
	require.ErrorContains(t, err, "violates foreign key constraint")

	// Missing contests map to ErrNotFound.
	_, err = repo.GetContest(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	now := time.Now().Truncate(time.Microsecond)
	end := now.Add(2 * time.Hour).Truncate(time.Microsecond)
	updatedTitle := "Updated " + title
	err = repo.UpdateContest(ctx, contestsmodel.ContestUpdateInput{
		ID:           contestID,
		Title:        &updatedTitle,
		StartTime:    &now,
		StartTimeSet: true,
		EndTime:      &end,
		EndTimeSet:   true,
	})
	require.NoError(t, err)

	updatedC, err := repo.GetContest(ctx, contestID)
	require.NoError(t, err)
	require.Equal(t, updatedTitle, updatedC.Title)
	require.NotNil(t, updatedC.StartTime)
	require.NotNil(t, updatedC.EndTime)

	// Settings patches preserve false values and fields they omit.
	for _, allowed := range []bool{true, false} {
		require.NoError(t, repo.UpdateContest(ctx, contestsmodel.ContestUpdateInput{ID: contestID, Settings: contestsmodel.ContestSettingsPatch{AllowClarifications: &allowed}}))
		stored, err := repo.GetContest(ctx, contestID)
		require.NoError(t, err)
		require.Equal(t, allowed, stored.Settings.AllowClarifications)

		for _, settingsPatch := range []contestsmodel.ContestSettingsPatch{{}, {HideStatements: new(true)}} {
			require.NoError(t, repo.UpdateContest(ctx, contestsmodel.ContestUpdateInput{ID: contestID, Settings: settingsPatch}))
			stored, err = repo.GetContest(ctx, contestID)
			require.NoError(t, err)
			require.Equal(t, allowed, stored.Settings.AllowClarifications)
		}
	}
}

func TestSubmissionsRepo_Integration(t *testing.T) {
	t.Parallel()
	pool := getTestPool(t)
	repo := postgres.NewSubmissionsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	orgID := helperCreateOrg(t, ctx, pool, "org_sub_"+suffix, "Sub Org "+suffix)

	userID := uuid.New()
	uUsername := "sub_u_" + suffix
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, 'hash', 'user')`,
		userID, uUsername,
	)
	require.NoError(t, err)

	contestID := uuid.New()
	err = dbfixtures.InsertContest(ctx, pool, dbfixtures.ContestSpec{ID: contestID, OrganizationID: orgID, Owner: userID, Login: "sub_c_" + suffix, Title: "Sub Contest", Visibility: "public"})
	require.NoError(t, err)

	problemID := uuid.New()
	err = dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: orgID, Owner: helperOrgOwner(t, ctx, pool, orgID), Title: "Sub Problem", ShortName: "sub_p_" + suffix})
	require.NoError(t, err)

	pkgID := helperCreatePackage(t, ctx, pool, problemID, orgID)
	_, err = pool.Exec(ctx,
		`INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal) VALUES ($1, $2, $3, 1)`,
		contestID, problemID, pkgID,
	)
	require.NoError(t, err)

	subID, err := repo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
		Kind:      submissionsmodel.KindContest,
		ContestId: contestID,
		ProblemId: problemID,
		UserId:    userID,
		Solution:  "print('hello world')",
		Language:  "python3",
		Penalty:   10,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, subID)

	sub, err := repo.GetSubmission(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, subID, sub.ID)
	require.Equal(t, submissionsmodel.Saved, sub.State)
	require.Equal(t, "print('hello world')", sub.Submission)
	require.Equal(t, "python3", sub.Language)
	require.Equal(t, int32(10), sub.Penalty)
	require.GreaterOrEqual(t, sub.Revision, int64(1))

	failedTestIdx := int32(3)
	change, err := repo.UpdateSubmission(ctx, subID, &submissionsmodel.SubmissionUpdate{
		State:      submissionsmodel.Accepted,
		Score:      100,
		TimeStat:   75,
		MemoryStat: 4096,
		FailedTest: &failedTestIdx,
		TestDetails: &submissionsmodel.SubmissionTestDetails{
			Tests: []submissionsmodel.TestDetailItem{
				{TestIndex: 1, Verdict: "AC", TimeMs: 20, MemoryKb: 1024},
				{TestIndex: 2, Verdict: "AC", TimeMs: 25, MemoryKb: 1024},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Saved, change.PreviousState)
	require.Greater(t, change.Revision, sub.Revision)

	updatedSub, err := repo.GetSubmission(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Accepted, updatedSub.State)
	require.Equal(t, points.Value(100), updatedSub.Score)
	require.Equal(t, int32(75), updatedSub.TimeStat)
	require.Equal(t, int32(4096), updatedSub.MemoryStat)
	require.Equal(t, &failedTestIdx, updatedSub.FailedTest)
	require.Len(t, updatedSub.TestDetails.Tests, 2)

	// Missing submissions map to ErrNotFound.
	_, err = repo.GetSubmission(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	// Missing submission contests map to ErrBadInput.
	_, err = repo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
		Kind:      submissionsmodel.KindContest,
		ContestId: uuid.New(),
		ProblemId: problemID,
		UserId:    userID,
		Solution:  "solution",
		Language:  "python3",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "violates foreign key constraint")
}

func TestBlogsRepo_Integration(t *testing.T) {
	t.Parallel()
	pool := getTestPool(t)
	repo := postgres.NewBlogsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	authorID := uuid.New()
	authorName := "author_" + suffix
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, password_hash) VALUES ($1, $2, 'unused')`, authorID, authorName)
	require.NoError(t, err)
	imgKey := "blogs/cover_" + suffix + ".jpg"

	postID, err := repo.CreatePost(ctx, blogsmodel.CreatePostParams{
		Title:       "Blog Post " + suffix,
		Text:        "Full markdown text content " + suffix,
		Description: "Short summary " + suffix,
		AuthorID:    authorID,
		ImageKey:    imgKey,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, postID)

	post, err := repo.GetPostByID(ctx, postID)
	require.NoError(t, err)
	require.Equal(t, postID, post.ID)
	require.Equal(t, "Blog Post "+suffix, post.Title)
	require.Equal(t, "Full markdown text content "+suffix, post.Text)
	require.Equal(t, "Short summary "+suffix, post.Description)
	require.Equal(t, authorID, post.AuthorID)
	require.Equal(t, authorName, post.AuthorName)
	require.Equal(t, imgKey, post.ImageKey)

	newTitle := "Updated Title " + suffix
	newText := "Updated Text " + suffix
	newDesc := "Updated desc"
	err = repo.UpdatePost(ctx, blogsmodel.UpdatePostParams{
		ID:          postID,
		Title:       &newTitle,
		Text:        &newText,
		Description: &newDesc,
	})
	require.NoError(t, err)

	updatedPost, err := repo.GetPostByID(ctx, postID)
	require.NoError(t, err)
	require.Equal(t, newTitle, updatedPost.Title)
	require.Equal(t, newText, updatedPost.Text)

	// Missing posts map to ErrNotFound.
	_, err = repo.GetPostByID(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	err = repo.DeletePost(ctx, postID)
	require.NoError(t, err)

	_, err = repo.GetPostByID(ctx, postID)
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound after deletion, got: %v", err)

	// Deleting a missing post maps to ErrNotFound.
	err = repo.DeletePost(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound when deleting non-existent post, got: %v", err)
}
