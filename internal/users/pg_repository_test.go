package users

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func setupTest(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	tearDown := func() {
		db.Close()
	}

	return sqlxDB, mock, tearDown
}

func TestRepository_CreateUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	user := &models.UserCreation{
		Id:       userID,
		Username: "testuser",
		Role:     "user",
		KratosId: nil,
	}

	t.Run("Success", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		mock.ExpectQuery(regexp.QuoteMeta(CreateUserQuery)).
			WithArgs(user.Id, user.Username, user.Role, user.KratosId).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(user.Id))

		id, err := repo.CreateUser(ctx, user)

		assert.NoError(t, err)
		assert.Equal(t, user.Id, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("QueryError", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		originErr := errors.New("db error")

		mock.ExpectQuery(regexp.QuoteMeta(CreateUserQuery)).
			WithArgs(user.Id, user.Username, user.Role, user.KratosId).
			WillReturnError(originErr)

		id, err := repo.CreateUser(ctx, user)

		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, id)
		assert.ErrorIs(t, err, originErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ScanError", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		// wrong column
		mock.ExpectQuery(regexp.QuoteMeta(CreateUserQuery)).
			WithArgs(user.Id, user.Username, user.Role, user.KratosId).
			WillReturnRows(sqlmock.NewRows([]string{"wrong"}).AddRow("wrong"))

		id, err := repo.CreateUser(ctx, user)

		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRepository_CreateUserWithKratosId(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	kratosId := "kratos-123"

	user := &models.UserCreation{
		Id:       userID,
		Username: "kratosuser",
		Role:     "user",
		KratosId: &kratosId,
	}

	db, mock, tearDown := setupTest(t)
	defer tearDown()
	repo := NewRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta(CreateUserQuery)).
		WithArgs(user.Id, user.Username, user.Role, user.KratosId).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(user.Id))

	id, err := repo.CreateUser(ctx, user)

	assert.NoError(t, err)
	assert.Equal(t, user.Id, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetUserById(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	expectedUser := &models.User{
		Id:        userID,
		Username:  "testuser",
		Role:      "user",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	t.Run("Success", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		rows := sqlmock.NewRows([]string{"id", "username", "role", "created_at"}).
			AddRow(expectedUser.Id, expectedUser.Username, expectedUser.Role, expectedUser.CreatedAt)

		mock.ExpectQuery(regexp.QuoteMeta(GetUserByIdQuery)).
			WithArgs(userID).
			WillReturnRows(rows)

		user, err := repo.GetUserById(ctx, userID)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, expectedUser.Username, user.Username)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("NotFound", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		mock.ExpectQuery(regexp.QuoteMeta(GetUserByIdQuery)).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserById(ctx, userID)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DBError", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		originErr := errors.New("db error")

		mock.ExpectQuery(regexp.QuoteMeta(GetUserByIdQuery)).
			WithArgs(userID).
			WillReturnError(originErr)

		user, err := repo.GetUserById(ctx, userID)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, originErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRepository_GetUserByKratosId(t *testing.T) {
	ctx := context.Background()
	kratosId := "kratos-123"

	expectedUser := &models.User{
		Id:        uuid.New(),
		Username:  "kratos-user",
		Role:      "admin",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	t.Run("Success", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		rows := sqlmock.NewRows([]string{"id", "username", "role", "created_at"}).
			AddRow(expectedUser.Id, expectedUser.Username, expectedUser.Role, expectedUser.CreatedAt)

		mock.ExpectQuery(regexp.QuoteMeta(GetUserByKratosIdQuery)).
			WithArgs(kratosId).
			WillReturnRows(rows)

		user, err := repo.GetUserByKratosId(ctx, kratosId)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, expectedUser.Id, user.Id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("NotFound", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		mock.ExpectQuery(regexp.QuoteMeta(GetUserByKratosIdQuery)).
			WithArgs(kratosId).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByKratosId(ctx, kratosId)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRepository_SearchUsers(t *testing.T) {
	ctx := context.Background()

	search := &models.UsersSearch{
		Search: "john",
		Role:   "user",
		Page:   2,
	}

	offset := (search.Page - 1) * search.Page

	t.Run("Success", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		// Count
		mock.ExpectQuery(regexp.QuoteMeta(CountSearchUsersQuery)).
			WithArgs(search.Search, search.Role).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))

		// List
		rows := sqlmock.NewRows([]string{"id", "username", "role", "created_at"}).
			AddRow(uuid.New(), "john1", "user", time.Now()).
			AddRow(uuid.New(), "john2", "user", time.Now())

		mock.ExpectQuery(regexp.QuoteMeta(SearchUsersQuery)).
			WithArgs(search.Search, search.Role, search.Page, offset).
			WillReturnRows(rows)

		result, err := repo.SearchUsers(ctx, search)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(5), result.Pagination.Total)
		assert.Len(t, result.Users, 2)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CountError", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		mock.ExpectQuery(regexp.QuoteMeta(CountSearchUsersQuery)).
			WithArgs(search.Search, search.Role).
			WillReturnError(errors.New("db count error"))

		list, err := repo.SearchUsers(ctx, search)

		assert.Error(t, err)
		assert.Nil(t, list)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SelectError", func(t *testing.T) {
		db, mock, tearDown := setupTest(t)
		defer tearDown()
		repo := NewRepository(db)

		mock.ExpectQuery(regexp.QuoteMeta(CountSearchUsersQuery)).
			WithArgs(search.Search, search.Role).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

		mock.ExpectQuery(regexp.QuoteMeta(SearchUsersQuery)).
			WithArgs(search.Search, search.Role, search.Page, offset).
			WillReturnError(errors.New("select error"))

		list, err := repo.SearchUsers(ctx, search)

		assert.Error(t, err)
		assert.Nil(t, list)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
