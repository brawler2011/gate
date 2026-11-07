package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"slices"

	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
)

// Fixed-width UUIDs and separate namespaces distinguish independent lock domains.
// Hash collisions only cause extra contention, never a loss of mutual exclusion.
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

// LockSubmissionTuple must precede submission row locks and standings reads. It
// also protects an absent standings row and the user/problem block predicate.
// Call only on a transaction-bound repository, retaining it through recalculation.
func (r *ContestsRepo) LockSubmissionTuple(ctx context.Context, contest, user, problem uuid.UUID) error {
	return HandlePgErr(r.queries.AdvisoryTransactionLock(ctx, submissionTupleKey(contest, user, problem)))
}

func (r *DraftsRepo) LockDraftOwner(ctx context.Context, contest, user uuid.UUID) error {
	return HandlePgErr(r.queries.AdvisoryTransactionLock(ctx, advisoryKey("gate:draft-owner:", contest, user)))
}

// Capture candidates before waiting. A bulk command affects exactly this set,
// acquiring all tuple locks in key order before acquiring any submission rows.
func (r *SubmissionsRepo) lockCandidates(ctx context.Context, filter sqlc.SubmissionCandidatesParams) ([]uuid.UUID, error) {
	candidates, err := r.queries.SubmissionCandidates(ctx, filter)
	if err != nil {
		return nil, HandlePgErr(err)
	}
	ids := make([]uuid.UUID, 0, len(candidates))
	keys := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
		if candidate.ContestID != nil && candidate.OwnerID != nil && candidate.ProblemID != nil {
			keys = append(keys, submissionTupleKey(*candidate.ContestID, *candidate.OwnerID, *candidate.ProblemID))
		}
	}
	slices.Sort(keys)
	for _, key := range slices.Compact(keys) {
		if err := r.queries.AdvisoryTransactionLock(ctx, key); err != nil {
			return nil, HandlePgErr(err)
		}
	}
	// SubmissionCandidates orders IDs, so overlapping bulk operations lock rows
	// identically even when some foreign-key identifiers have been deleted.
	for _, id := range ids {
		if _, err := r.queries.LockSubmission(ctx, id); err != nil {
			return nil, HandlePgErr(err)
		}
	}
	return ids, nil
}

// Call only on transaction-bound repositories; acquire before reading or changing order/versions.
func (r *ContestsRepo) LockContestProblemOrder(ctx context.Context, contest uuid.UUID) error {
	return HandlePgErr(r.queries.AdvisoryTransactionLock(ctx, advisoryKey("gate:contest-problem-order:", contest)))
}
func (r *PackagesRepo) LockPackageVersions(ctx context.Context, problem uuid.UUID) error {
	return HandlePgErr(r.queries.AdvisoryTransactionLock(ctx, advisoryKey("gate:package-versions:", problem)))
}
