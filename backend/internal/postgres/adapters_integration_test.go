//go:build integration
// +build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testContainerOnce sync.Once
	testDSN           string
	testInitErr       error
)

func getSharedTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	testContainerOnce.Do(func() {
		dsn := os.Getenv("TEST_DATABASE_URL")
		if dsn == "" {
			dsn = os.Getenv("WORKSPACE_TEST_DSN")
		}

		if dsn == "" {
			container, err := postgres.Run(ctx,
				"postgres:18-alpine",
				postgres.WithDatabase("gate_adapters_test"),
				postgres.WithUsername("postgres"),
				postgres.WithPassword("postgres"),
				testcontainers.WithWaitStrategy(
					wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(15*time.Second),
				),
			)
			if err != nil {
				testInitErr = fmt.Errorf("start postgres container: %w", err)
				return
			}
			dsn, err = container.ConnectionString(ctx, "sslmode=disable")
			if err != nil {
				testInitErr = fmt.Errorf("get container dsn: %w", err)
				return
			}
		}

		_, b, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(b)
		migrationsPath := filepath.Join(basepath, "../../migrations")

		db, err := sql.Open("pgx", dsn)
		if err != nil {
			testInitErr = fmt.Errorf("open sql db for migrations: %w", err)
			return
		}
		defer db.Close()

		if err := goose.SetDialect("postgres"); err != nil {
			testInitErr = fmt.Errorf("set goose dialect: %w", err)
			return
		}
		if err := goose.Up(db, migrationsPath); err != nil {
			testInitErr = fmt.Errorf("run goose migrations: %w", err)
			return
		}

		testDSN = dsn
	})

	require.NoError(t, testInitErr)

	pool, err := pkg.NewPostgresDB(testDSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

func helperCreateOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, login, name string) uuid.UUID {
	t.Helper()
	orgID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, login, name) VALUES ($1, $2, $3)`,
		orgID, login, name,
	)
	require.NoError(t, err)
	return orgID
}

func helperCreatePackage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, problemID, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	pkgID := uuid.New()
	hash := "1111111111111111111111111111111111111111111111111111111111111111"
	_, err := pool.Exec(ctx,
		`INSERT INTO problem_packages (id, problem_id, organization_id, package_hash, status, version)
		 VALUES ($1, $2, $3, $4, 'ready', 1)`,
		pkgID, problemID, orgID, hash,
	)
	require.NoError(t, err)
	return pkgID
}

func TestUsersRepo_Integration(t *testing.T) {
	pool := getSharedTestPool(t)
	repo := NewUsersRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	username := "u_" + suffix
	email := "u_" + suffix + "@example.com"
	avatar := "https://example.com/avatar_" + suffix + ".png"
	exp := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	userID := uuid.New()

	// 1. CreateUser with all fields populated
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

	// 2. GetUserById: verify all fields and mappings
	user, err := repo.GetUserById(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, userID, user.Id)
	require.Equal(t, username, user.Username)
	require.Equal(t, accessmodel.UserRoleUser, user.Role)
	require.Equal(t, &email, user.Email)
	require.Equal(t, &avatar, user.AvatarUrl)

	// 3. GetUserByUsername
	byUsername, err := repo.GetUserByUsername(ctx, username)
	require.NoError(t, err)
	require.Equal(t, userID, byUsername.Id)

	// 4. Duplicate username constraint violation -> pkg.ErrConflict via HandlePgErr
	err = repo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:           uuid.New(),
		Username:     username,
		Role:         accessmodel.UserRoleUser,
		PasswordHash: "dummy_hash",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrConflict), "expected ErrConflict on duplicate username, got: %v", err)

	// 5. Non-existent ID -> pkg.ErrNotFound via HandlePgErr
	_, err = repo.GetUserById(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound for missing user, got: %v", err)

	// 6. UpdateUser: update avatar and email
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

	// 7. UpdateUserRole
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
	pool := getSharedTestPool(t)
	repo := NewProblemsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	orgID := helperCreateOrg(t, ctx, pool, "org_prob_"+suffix, "Problem Org "+suffix)

	probID := uuid.New()
	title := "Test Problem " + suffix
	shortName := "p_" + suffix

	// 1. CreateProblem
	err := repo.CreateProblem(ctx, &problemsmodel.CreateProblemParams{
		ID:             probID,
		OrganizationID: orgID,
		Title:          title,
		ShortName:      shortName,
		Visibility:     problemsmodel.ProblemVisibilityPublic,
	})
	require.NoError(t, err)

	// 2. GetProblemById: verify fields
	prob, err := repo.GetProblemById(ctx, probID)
	require.NoError(t, err)
	require.Equal(t, probID, prob.ID)
	require.Equal(t, title, prob.Title)
	require.Equal(t, shortName, prob.ShortName)
	require.Equal(t, orgID, prob.OrganizationID)
	require.Equal(t, 1000, prob.TimeLimitMs)
	require.Equal(t, 256, prob.MemoryLimitMb)

	// 3. Unique violation: same shortName in same org -> pkg.ErrConflict
	err = repo.CreateProblem(ctx, &problemsmodel.CreateProblemParams{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Title:          "Another Title",
		ShortName:      shortName,
		Visibility:     problemsmodel.ProblemVisibilityPublic,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrConflict), "expected ErrConflict on duplicate short_name in org, got: %v", err)

	// 4. Foreign key violation: non-existent organization_id -> pkg.ErrBadInput
	err = repo.CreateProblem(ctx, &problemsmodel.CreateProblemParams{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Title:          "Orphan Problem",
		ShortName:      "orphan_" + suffix,
		Visibility:     problemsmodel.ProblemVisibilityPublic,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrBadInput), "expected ErrBadInput on foreign key violation, got: %v", err)

	// 5. Get non-existent problem -> pkg.ErrNotFound
	_, err = repo.GetProblemById(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	// 6. ListProblems with filter
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
	pool := getSharedTestPool(t)
	repo := NewContestsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	orgID := helperCreateOrg(t, ctx, pool, "org_c_"+suffix, "Contest Org "+suffix)

	contestID := uuid.New()
	login := "contest_" + suffix
	title := "Contest Title " + suffix
	settings := map[string]interface{}{
		"freeze_monitor":    true,
		"penalty_per_wrong": float64(20),
	}

	// 1. CreateContest with JSON settings
	err := repo.CreateContest(ctx, &contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: orgID,
		Title:          title,
		Login:          login,
		Description:    "Description for contest",
		Visibility:     contestsmodel.ContestVisibilityPublic,
		Settings:       settings,
	})
	require.NoError(t, err)

	// 2. GetContest & verify JSON unmarshaling
	c, err := repo.GetContest(ctx, contestID)
	require.NoError(t, err)
	require.Equal(t, contestID, c.ID)
	require.Equal(t, login, c.Login)
	require.Equal(t, title, c.Title)
	require.Equal(t, orgID, c.OrganizationID)
	require.Equal(t, true, c.Settings["freeze_monitor"])
	require.Equal(t, float64(20), c.Settings["penalty_per_wrong"])

	// 3. Unique violation: duplicate login -> pkg.ErrConflict
	err = repo.CreateContest(ctx, &contestsmodel.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Title:          "Duplicate Login Contest",
		Login:          login,
		Visibility:     contestsmodel.ContestVisibilityPublic,
		Settings:       map[string]interface{}{},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrConflict), "expected ErrConflict on duplicate contest login, got: %v", err)

	// 4. Foreign key violation: invalid org -> pkg.ErrBadInput
	err = repo.CreateContest(ctx, &contestsmodel.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Title:          "Orphan Contest",
		Login:          "orphan_" + suffix,
		Visibility:     contestsmodel.ContestVisibilityPublic,
		Settings:       map[string]interface{}{},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrBadInput), "expected ErrBadInput on foreign key violation, got: %v", err)

	// 5. Get non-existent contest -> pkg.ErrNotFound
	_, err = repo.GetContest(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	// 6. UpdateContest: start/end time and title
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

	// Settings patches preserve false values and unrelated existing settings.
	for _, allowed := range []bool{true, false} {
		patch := map[string]interface{}{"allow_clarifications": allowed}
		require.NoError(t, repo.UpdateContest(ctx, contestsmodel.ContestUpdateInput{ID: contestID, Settings: &patch}))
		stored, err := repo.GetContest(ctx, contestID)
		require.NoError(t, err)
		require.Equal(t, allowed, stored.Settings["allow_clarifications"])
		require.Equal(t, true, stored.Settings["freeze_monitor"])

		// Omitting the flag, including when changing another setting, preserves it.
		for _, settingsPatch := range []*map[string]interface{}{nil, {"hide_statements": true}} {
			require.NoError(t, repo.UpdateContest(ctx, contestsmodel.ContestUpdateInput{ID: contestID, Settings: settingsPatch}))
			stored, err = repo.GetContest(ctx, contestID)
			require.NoError(t, err)
			require.Equal(t, allowed, stored.Settings["allow_clarifications"])
		}
	}
}

func TestSubmissionsRepo_Integration(t *testing.T) {
	pool := getSharedTestPool(t)
	repo := NewSubmissionsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	orgID := helperCreateOrg(t, ctx, pool, "org_sub_"+suffix, "Sub Org "+suffix)

	// Create user
	userID := uuid.New()
	uUsername := "sub_u_" + suffix
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, 'hash', 'user')`,
		userID, uUsername,
	)
	require.NoError(t, err)

	// Create contest
	contestID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO contests (id, organization_id, visibility, title, login, description, settings)
		 VALUES ($1, $2, 'public', 'Sub Contest', $3, 'desc', '{}')`,
		contestID, orgID, "sub_c_"+suffix,
	)
	require.NoError(t, err)

	// Create problem
	problemID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO problems (id, organization_id, title, short_name, time_limit_ms, memory_limit_mb)
		 VALUES ($1, $2, 'Sub Problem', $3, 1000, 256)`,
		problemID, orgID, "sub_p_"+suffix,
	)
	require.NoError(t, err)

	pkgID := helperCreatePackage(t, ctx, pool, problemID, orgID)
	_, err = pool.Exec(ctx,
		`INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal) VALUES ($1, $2, $3, 1)`,
		contestID, problemID, pkgID,
	)
	require.NoError(t, err)

	// 1. CreateSubmission
	subID, err := repo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
		ContestId: contestID,
		ProblemId: problemID,
		UserId:    userID,
		Solution:  "print('hello world')",
		Language:  submissionsmodel.Python,
		Penalty:   10,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, subID)

	// 2. GetSubmission: verify fields & default state
	sub, err := repo.GetSubmission(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, subID, sub.ID)
	require.Equal(t, submissionsmodel.Saved, sub.State)
	require.Equal(t, "print('hello world')", sub.Submission)
	require.Equal(t, submissionsmodel.Python, sub.Language)
	require.Equal(t, int32(10), sub.Penalty)
	require.GreaterOrEqual(t, sub.Revision, int64(1))

	// 3. UpdateSubmission: state, score, test_details JSON
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
	require.Equal(t, int32(100), updatedSub.Score)
	require.Equal(t, int32(75), updatedSub.TimeStat)
	require.Equal(t, int32(4096), updatedSub.MemoryStat)
	require.Equal(t, &failedTestIdx, updatedSub.FailedTest)
	require.Len(t, updatedSub.TestDetails.Tests, 2)

	// 4. Non-existent submission -> pkg.ErrNotFound
	_, err = repo.GetSubmission(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	// 5. Foreign key error: invalid contest ID -> pkg.ErrBadInput
	_, err = repo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
		ContestId: uuid.New(),
		ProblemId: problemID,
		UserId:    userID,
		Solution:  "solution",
		Language:  submissionsmodel.Python,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrBadInput), "expected ErrBadInput for invalid contest, got: %v", err)
}

func TestBlogsRepo_Integration(t *testing.T) {
	pool := getSharedTestPool(t)
	repo := NewBlogsRepo(pool)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	authorID := uuid.New()
	authorName := "author_" + suffix
	imgKey := "blogs/cover_" + suffix + ".jpg"

	// 1. CreatePost with optional image_key
	postID, err := repo.CreatePost(ctx, blogsmodel.CreatePostParams{
		Title:       "Blog Post " + suffix,
		Text:        "Full markdown text content " + suffix,
		Description: "Short summary " + suffix,
		AuthorUUID:  authorID,
		AuthorName:  authorName,
		ImageKey:    imgKey,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, postID)

	// 2. GetPostByID: verify fields and nullable image_key
	post, err := repo.GetPostByID(ctx, postID)
	require.NoError(t, err)
	require.Equal(t, postID, post.ID)
	require.Equal(t, "Blog Post "+suffix, post.Title)
	require.Equal(t, "Full markdown text content "+suffix, post.Text)
	require.Equal(t, "Short summary "+suffix, post.Description)
	require.Equal(t, authorID, post.AuthorUUID)
	require.Equal(t, authorName, post.AuthorName)
	require.Equal(t, imgKey, post.ImageKey)

	// 3. UpdatePost
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

	// 4. Non-existent post -> pkg.ErrNotFound
	_, err = repo.GetPostByID(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound, got: %v", err)

	// 5. DeletePost
	err = repo.DeletePost(ctx, postID)
	require.NoError(t, err)

	_, err = repo.GetPostByID(ctx, postID)
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound after deletion, got: %v", err)

	// 6. Delete non-existent post -> pkg.ErrNotFound
	err = repo.DeletePost(ctx, uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, pkg.ErrNotFound), "expected ErrNotFound when deleting non-existent post, got: %v", err)
}
