package judging

import (
	"context"
	"fmt"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
)

// SystemTest rejudges on all tests the contest submissions judged on pretests whose contest has ended.
func SystemTest(ctx context.Context, transactions JudgeTransactions) (int, error) {
	var count int
	err := transactions.Within(ctx, func(tx JudgeTxStore) error {
		resets, err := tx.ResetFinishedPretests(ctx)
		if err != nil {
			return err
		}
		for _, reset := range resets {
			if _, err := tx.ScheduleJob(ctx, reset.ID); err != nil {
				return err
			}
			if err := tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCreated, ID: reset.ID}); err != nil {
				return fmt.Errorf("notify system test of submission %s: %w", reset.ID, err)
			}
		}
		count = len(resets)
		return nil
	})
	return count, err
}
