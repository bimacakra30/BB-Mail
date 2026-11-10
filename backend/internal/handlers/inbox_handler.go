package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"baramail/backend/internal/config"
	"baramail/backend/internal/middleware"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type InboxHandler struct {
	cfg        *config.Config
	authSvc    *services.AuthService
	mailboxSvc *services.MailboxService
	msgRepo    *repositories.MessageRepository
}

func NewInboxHandler(
	cfg *config.Config,
	authSvc *services.AuthService,
	mailboxSvc *services.MailboxService,
	msgRepo *repositories.MessageRepository,
) *InboxHandler {
	return &InboxHandler{
		cfg:        cfg,
		authSvc:    authSvc,
		mailboxSvc: mailboxSvc,
		msgRepo:    msgRepo,
	}
}

func (h *InboxHandler) Access(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}

	mb, err := h.mailboxSvc.OpenInbox(r.Context(), req.Email)
	if err != nil {
		// Uniform anti-enumeration response for not found, disabled, expired, or retired domain.
		WriteError(w, http.StatusNotFound, "MAILBOX_NOT_AVAILABLE", "Mailbox tidak tersedia.")
		return
	}

	token, tokenExp, err := h.authSvc.IssueInboxToken(mb)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal membuat token akses inbox.")
		return
	}

	WriteSuccess(w, http.StatusOK, map[string]any{
		"email":                 mb.Email,
		"expires_at":            mb.ExpiresAt,
		"access_token":          token,
		"token_expires_at":      tokenExp,
		"poll_interval_seconds": h.cfg.InboxPollIntervalSeconds,
	}, "OK")
}

func (h *InboxHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	mb := middleware.MailboxFromContext(r.Context())
	if mb == nil {
		WriteError(w, http.StatusNotFound, "MAILBOX_NOT_AVAILABLE", "Mailbox tidak tersedia.")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 20
	}
	beforeID, _ := strconv.ParseUint(q.Get("before_id"), 10, 64)
	afterID, _ := strconv.ParseUint(q.Get("after_id"), 10, 64)

	msgs, err := h.msgRepo.ListMessages(r.Context(), mb.ID, limit, beforeID, afterID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat daftar pesan.")
		return
	}

	var nextBeforeID uint64
	if len(msgs) == limit && afterID == 0 {
		nextBeforeID = msgs[len(msgs)-1].ID
	}

	WriteSuccess(w, http.StatusOK, map[string]any{
		"mailbox": map[string]any{
			"email":      mb.Email,
			"expires_at": mb.ExpiresAt,
		},
		"items":          msgs,
		"next_before_id": nextBeforeID,
	}, "OK")
}

func (h *InboxHandler) GetMessage(w http.ResponseWriter, r *http.Request) {
	mb := middleware.MailboxFromContext(r.Context())
	if mb == nil {
		WriteError(w, http.StatusNotFound, "MAILBOX_NOT_AVAILABLE", "Mailbox tidak tersedia.")
		return
	}

	msgID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || msgID == 0 {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Pesan tidak ditemukan.")
		return
	}

	msg, err := h.msgRepo.GetMessage(r.Context(), mb.ID, msgID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Pesan tidak ditemukan.")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat detail pesan.")
		return
	}

	WriteSuccess(w, http.StatusOK, msg, "OK")
}

