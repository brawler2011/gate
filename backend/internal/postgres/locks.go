package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"

	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
)

// Hash collisions only add contention; fixed-width UUIDs and namespaces separate lock domains.
func advisoryKey(namespace string, ids ...uuid.UUID) int64 {
	h := sha256.New()
	h.Write([]byte(namespace))
	for _, id := range ids {
		h.Write(id[:])
	}
	return int64(binary.BigEndian.Uint64(h.Sum(nil)[:8]) & math.MaxInt64)
}

func submissionTupleKey(contest, user, problem uuid.UUID) int64 {
	return advisoryKey("gate:submission-tuple:", contest, user, problem)
}

// Transaction only: serializes submitting against blocking the same user and problem.
func (r *ContestsRepo) LockSubmissionTuple(ctx context.Context, contest, user, problem uuid.UUID) error {
	return dbError("take advisory lock", r.queries.AdvisoryTransactionLock(ctx, submissionTupleKey(contest, user, problem)))
}

func (r *DraftsRepo) LockDraftOwner(ctx context.Context, contest, user uuid.UUID) error {
	return dbError("take advisory lock", r.queries.AdvisoryTransactionLock(ctx, advisoryKey("gate:draft-owner:", contest, user)))
}

// Candidate IDs define row-lock order even if foreign-key targets have been deleted.
func (r *SubmissionsRepo) lockCandidates(ctx context.Context, filter sqlc.SubmissionCandidatesParams) ([]uuid.UUID, error) {
	ids, err := r.queries.SubmissionCandidates(ctx, filter)
	if err != nil {
		return nil, dbError("lock candidates", err)
	}
	for _, id := range ids {
		if _, err := r.queries.LockSubmission(ctx, id); err != nil {
			return nil, dbError("lock candidates", err)
		}
	}
	return ids, nil
}

// Call only on transaction-bound repositories; acquire before reading or changing order/versions.
func (r *ContestsRepo) LockContestProblemOrder(ctx context.Context, contest uuid.UUID) error {
	return dbError("take advisory lock", r.queries.AdvisoryTransactionLock(ctx, advisoryKey("gate:contest-problem-order:", contest)))
}
func (r *PackagesRepo) LockPackageVersions(ctx context.Context, problem uuid.UUID) error {
	return dbError("take advisory lock", r.queries.AdvisoryTransactionLock(ctx, advisoryKey("gate:package-versions:", problem)))
}
