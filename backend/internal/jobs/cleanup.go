package jobs

import (
	"context"
	"log/slog"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/repositories"
)

type CleanupRunner struct {
	cfg       *config.Config
	msgRepo   *repositories.MessageRepository
	adminRepo *repositories.AdminRepository
	logger    *slog.Logger
}

func NewCleanupRunner(
	cfg *config.Config,
	msgRepo *repositories.MessageRepository,
	adminRepo *repositories.AdminRepository,
	logger *slog.Logger,
) *CleanupRunner {
	return &CleanupRunner{
		cfg:       cfg,
		msgRepo:   msgRepo,
		adminRepo: adminRepo,
		logger:    logger,
	}
}

func (c *CleanupRunner) Start(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.CleanupInterval)
	defer ticker.Stop()

	c.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.RunOnce(ctx)
		}
	}
}

func (c *CleanupRunner) RunOnce(ctx context.Context) {
	now := time.Now().UTC()

	if c.cfg.MessageRetentionHours > 0 {
		ttlCutoff := now.Add(-time.Duration(c.cfg.MessageRetentionHours) * time.Hour)
		deletedTTL, err := c.msgRepo.DeleteExpiredMessagesByTTL(ctx, ttlCutoff)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("cleanup_ttl_failed", slog.String("error", err.Error()))
			}
		} else if deletedTTL > 0 && c.logger != nil {
			c.logger.Info("cleanup_ttl_completed", slog.Int64("deleted_messages", deletedTTL))
		}
	}

	if c.cfg.ExpiredMailboxRetentionDays > 0 {
		expiredBefore := now.Add(-time.Duration(c.cfg.ExpiredMailboxRetentionDays) * 24 * time.Hour)
		deletedExpiredMB, err := c.msgRepo.DeleteMessagesOfExpiredMailboxes(ctx, expiredBefore)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("cleanup_expired_mailbox_messages_failed", slog.String("error", err.Error()))
			}
		} else if deletedExpiredMB > 0 && c.logger != nil {
			c.logger.Info("cleanup_expired_mailbox_messages_completed", slog.Int64("deleted_messages", deletedExpiredMB))
		}
	}

	if _, err := c.adminRepo.DeleteExpiredSessions(ctx, now); err != nil && c.logger != nil {
		c.logger.Error("cleanup_sessions_failed", slog.String("error", err.Error()))
	}
}

