package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

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
	recipientsByChannel := make(map[string][]sqlc.NotificationRecipient)
	for _, delivery := range deliveries {
		recipients, loaded := recipientsByChannel[delivery.ChannelID]
		if !loaded {
			var err error
			recipients, err = q.ListNotificationRecipients(ctx, delivery.ChannelID)
			if err != nil {
				return err
			}
			recipientsByChannel[delivery.ChannelID] = recipients
		}
		for _, recipient := range recipients {
			if recipient.Verified == 0 {
				continue
			}
			item := delivery
			item.RecipientID = recipient.ID
			if recipient.ID != delivery.ChannelID {
				sum := sha256.Sum256([]byte(delivery.ID + ":" + recipient.ID))
				item.ID = hex.EncodeToString(sum[:16])
			}
			if err := q.CreateNotificationDelivery(ctx, item); err != nil {
				return err
			}
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

// SaveNotificationChannel changes the channel and its allowlist atomically.
func (s *Store) SaveNotificationChannel(ctx context.Context, channel sqlc.NotificationChannel, targets []string, reset bool) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if channel.Revision == 0 {
			err := q.CreateNotificationChannel(ctx, sqlc.CreateNotificationChannelParams{
				ID: channel.ID, Name: channel.Name, Platform: channel.Platform, Target: channel.Target,
				CredentialRef: channel.CredentialRef, Enabled: channel.Enabled, Mode: channel.Mode,
			})
			if err != nil {
				return err
			}
		} else {
			count, err := q.UpdateNotificationChannel(ctx, sqlc.UpdateNotificationChannelParams{
				ID: channel.ID, Name: channel.Name, Platform: channel.Platform, Target: channel.Target,
				CredentialRef: channel.CredentialRef, Enabled: channel.Enabled, Revision: channel.Revision,
			})
			if err != nil {
				return err
			}
			if count != 1 {
				return ErrConflict
			}
		}
		existing, err := q.ListNotificationRecipients(ctx, channel.ID)
		if err != nil {
			return err
		}
		keep := make(map[string]bool, len(targets))
		for _, target := range targets {
			keep[target] = true
		}
		for _, recipient := range existing {
			if reset || !keep[recipient.Target] {
				if err := q.RetireNotificationRecipient(ctx, recipient.ID); err != nil {
					return err
				}
				continue
			}
			delete(keep, recipient.Target)
		}
		for _, target := range targets {
			if keep[target] {
				if err := q.CreateNotificationRecipient(ctx, sqlc.CreateNotificationRecipientParams{
					ID: rand.Text(), ChannelID: channel.ID, Target: target,
				}); err != nil {
					return err
				}
			}
		}
		return q.RefreshNotificationChannelVerification(ctx, channel.ID)
	})
}

func (s *Store) VerifyNotificationRecipient(ctx context.Context, channelID string, recipient sqlc.VerifyNotificationRecipientParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		rows, err := q.VerifyNotificationRecipient(ctx, recipient)
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrConflict
		}
		return q.RefreshNotificationChannelVerification(ctx, channelID)
	})
}
