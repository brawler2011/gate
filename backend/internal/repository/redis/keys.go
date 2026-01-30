package cache

import (
	"fmt"

	"github.com/google/uuid"
)

func UserKey(id uuid.UUID) string {
	return fmt.Sprintf("user:%s", id)
}

func UserByKratosIdKey(kratosId uuid.UUID) string {
	return fmt.Sprintf("user_by_kratos_id:%s", kratosId)
}

func UsersListKey(page, pageSize int32, search, role string) string {
	return fmt.Sprintf("users_list:%d:%d:%s:%s", page, pageSize, search, role)
}

func ContestKey(id uuid.UUID) string {
	return fmt.Sprintf("contest:%s", id)
}

func ProblemKey(id uuid.UUID) string {
	return fmt.Sprintf("problem:%s", id)
}

func PermissionKey(userID, resourceID uuid.UUID, action string) string {
	return fmt.Sprintf("perm:%s:%s:%s", userID, resourceID, action)
}

func ContestProblemKey(contestID, problemID uuid.UUID) string {
	return fmt.Sprintf("contest_problem:%s:%s", contestID, problemID)
}

func ContestMemberKey(contestID, userID uuid.UUID) string {
	return fmt.Sprintf("contest_member:%s:%s", contestID, userID)
}

func ProblemMemberKey(problemID, userID uuid.UUID) string {
	return fmt.Sprintf("problem_member:%s:%s", problemID, userID)
}

func ProblemTestsKey(problemID uuid.UUID) string {
	return fmt.Sprintf("problem_tests:%s", problemID)
}
