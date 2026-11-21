package database

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func (s *Store) NotificationQueries() *sqlc.Queries { return s.queries }

func (s *Store) SaveNotificationBatch(ctx context.Context, rule sqlc.AdvanceNotificationRuleParams, deliveries []sqlc.CreateNotificationDeliveryParams, digestItems []sqlc.CreateNotificationDigestItemParams,
	candidates []sqlc.AddNotificationCandidateParams, matched []int64,
) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		count, err := q.AdvanceNotificationRule(ctx, rule)
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrConflict
		}
		for _, item := range digestItems {
			if err := q.CreateNotificationDigestItem(ctx, item); err != nil {
				return err
			}
		}
		for _, candidate := range candidates {
			if err := q.AddNotificationCandidate(ctx, candidate); err != nil {
				return err
			}
		}
		for _, seq := range matched {
			if err := q.RemoveNotificationCandidate(ctx, sqlc.RemoveNotificationCandidateParams{RuleID: rule.ID, EventSeq: seq}); err != nil {
				return err
			}
		}
		return insertDeliveries(ctx, q, deliveries)
	})
}

func (s *Store) ClaimNotificationTask(ctx context.Context, task sqlc.ClaimScheduledTaskParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		count, err := q.ClaimScheduledTask(ctx, task)
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrConflict
		}
		return q.StartScheduledRun(ctx, sqlc.StartScheduledRunParams{TaskID: task.ID, Occurrence: task.NextAt_2, TaskRevision: task.Revision})
	})
}

func (s *Store) CompleteNotificationTask(ctx context.Context, run sqlc.FinishScheduledRunParams, deliveries []sqlc.CreateNotificationDeliveryParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		count, err := q.FinishScheduledRun(ctx, run)
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrConflict
		}
		return insertDeliveries(ctx, q, deliveries)
	})
}

func (s *Store) EnqueueNotification(ctx context.Context, deliveries []sqlc.CreateNotificationDeliveryParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error { return insertDeliveries(ctx, q, deliveries) })
}

func insertDeliveries(ctx context.Context, q *sqlc.Queries, deliveries []sqlc.CreateNotificationDeliveryParams) error {
	for _, delivery := range deliveries {
		if err := q.CreateNotificationDelivery(ctx, delivery); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) QueueNotificationDigest(ctx context.Context, items []sqlc.NotificationDigestItem, deliveries []sqlc.CreateNotificationDeliveryParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		for _, item := range items {
			count, err := q.QueueNotificationDigestItem(ctx, item.ID)
			if err != nil {
				return err
			}
			if count != 1 {
				return ErrConflict
			}
		}
		return insertDeliveries(ctx, q, deliveries)
	})
}
