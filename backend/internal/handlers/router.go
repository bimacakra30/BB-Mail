package handlers

import (
	"log/slog"
	"net/http"

	"baramail/backend/internal/config"
	"baramail/backend/internal/middleware"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	authSvc *services.AuthService,
	mailboxSvc *services.MailboxService,
	ingestionSvc *services.IngestionService,
	mailboxRepo *repositories.MailboxRepository,
	msgRepo *repositories.MessageRepository,
	adminRepo *repositories.AdminRepository,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestContextAndLogger(cfg, logger))

	adminHandler := NewAdminHandler(cfg, authSvc, mailboxSvc, mailboxRepo, msgRepo, adminRepo)
	inboxHandler := NewInboxHandler(cfg, authSvc, mailboxSvc, msgRepo)
	webhookHandler := NewWebhookHandler(cfg, ingestionSvc)

	loginLimiter := middleware.NewRateLimiter(cfg.RateLimitLoginPerMin)
	accessLimiter := middleware.NewRateLimiter(cfg.RateLimitAccessPerMin)
	messagesLimiter := middleware.NewRateLimiter(cfg.RateLimitMessagesPerMin)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			WriteSuccess(w, http.StatusOK, map[string]string{"status": "ok"}, "OK")
		})

		r.With(middleware.RateLimitByIP(loginLimiter)).Post("/admin/login", adminHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin(cfg, authSvc))
			r.Post("/admin/logout", adminHandler.Logout)
			r.Get("/admin/me", adminHandler.Me)

			r.Get("/admin/domains", adminHandler.ListDomains)
			r.Post("/admin/domains", adminHandler.CreateDomain)
			r.Patch("/admin/domains/{id}", adminHandler.UpdateDomain)

			r.Get("/admin/mailboxes", adminHandler.ListMailboxes)
			r.Post("/admin/mailboxes", adminHandler.CreateMailbox)
			r.Post("/admin/mailboxes/bulk", adminHandler.BulkCreateMailboxes)
			r.Post("/admin/mailboxes/bulk-action", adminHandler.BulkActionMailboxes)
			r.Get("/admin/mailboxes/export", adminHandler.ExportMailboxesCSV)
			r.Get("/admin/mailboxes/{id}", adminHandler.GetMailbox)
			r.Patch("/admin/mailboxes/{id}", adminHandler.UpdateMailbox)
			r.Delete("/admin/mailboxes/{id}", adminHandler.DeleteMailbox)
			r.Get("/admin/mailboxes/{id}/messages", adminHandler.ListMailboxMessages)

			r.Get("/admin/stats", adminHandler.Stats)
			r.Get("/admin/audit-logs", adminHandler.AuditLogs)
		})

		r.With(middleware.RateLimitByIP(accessLimiter)).Post("/inbox/access", inboxHandler.Access)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireInboxToken(authSvc, mailboxSvc))
			r.Use(middleware.RateLimitByInboxToken(messagesLimiter))
			r.Get("/inbox/messages", inboxHandler.ListMessages)
			r.Get("/inbox/messages/{id}", inboxHandler.GetMessage)
		})

		r.Post("/webhooks/email", webhookHandler.ReceiveEmail)
	})

	return r
}

