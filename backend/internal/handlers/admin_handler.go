package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/middleware"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type AdminHandler struct {
	cfg         *config.Config
	authSvc     *services.AuthService
	mailboxSvc  *services.MailboxService
	mailboxRepo *repositories.MailboxRepository
	msgRepo     *repositories.MessageRepository
	adminRepo   *repositories.AdminRepository
}

func NewAdminHandler(
	cfg *config.Config,
	authSvc *services.AuthService,
	mailboxSvc *services.MailboxService,
	mailboxRepo *repositories.MailboxRepository,
	msgRepo *repositories.MessageRepository,
	adminRepo *repositories.AdminRepository,
) *AdminHandler {
	return &AdminHandler{
		cfg:         cfg,
		authSvc:     authSvc,
		mailboxSvc:  mailboxSvc,
		mailboxRepo: mailboxRepo,
		msgRepo:     msgRepo,
		adminRepo:   adminRepo,
	}
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Username dan password wajib diisi.")
		return
	}

	ip := middleware.ClientIPFromContext(r.Context())
	user, session, cookieVal, err := h.authSvc.Login(r.Context(), req.Username, req.Password, ip, r.UserAgent())
	if err != nil {
		if errors.Is(err, services.ErrAccountLocked) {
			WriteError(w, http.StatusTooManyRequests, "ACCOUNT_LOCKED", "Terlalu banyak percobaan login gagal. Silakan coba lagi nanti.")
			return
		}
		WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Username atau password salah.")
		return
	}

	secureCookie := h.cfg.AppEnv == "production" || strings.HasPrefix(strings.ToLower(h.cfg.AppBaseURL), "https://")
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AdminSessionCookie,
		Value:    cookieVal,
		Path:     "/",
		Expires:  session.ExpiresAt,
		MaxAge:   int(h.cfg.AdminSessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	WriteSuccess(w, http.StatusOK, map[string]any{
		"user":       user,
		"csrf_token": session.CSRFToken,
		"expires_at": session.ExpiresAt,
	}, "Login berhasil")
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user := middleware.AdminUserFromContext(r.Context())
	session := middleware.AdminSessionFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	if session != nil && user != nil {
		_ = h.authSvc.Logout(r.Context(), session.ID, user.ID, ip)
	}

	secureCookie := h.cfg.AppEnv == "production" || strings.HasPrefix(strings.ToLower(h.cfg.AppBaseURL), "https://")
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AdminSessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	WriteSuccess(w, http.StatusOK, map[string]any{}, "Logout berhasil")
}

func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.AdminUserFromContext(r.Context())
	session := middleware.AdminSessionFromContext(r.Context())
	WriteSuccess(w, http.StatusOK, map[string]any{
		"user":       user,
		"csrf_token": session.CSRFToken,
		"expires_at": session.ExpiresAt,
		"config": map[string]any{
			"random_localpart_length":     h.cfg.RandomLocalpartLen,
			"default_mailbox_expiry_days": h.cfg.DefaultMailboxExpiryDays,
			"max_mailbox_expiry_days":     h.cfg.MaxMailboxExpiryDays,
			"max_bulk_create":             h.cfg.MaxBulkCreate,
		},
	}, "OK")
}

func (h *AdminHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.mailboxSvc.ListDomains(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat daftar domain.")
		return
	}
	WriteSuccess(w, http.StatusOK, domains, "OK")
}

func (h *AdminHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}
	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	d, err := h.mailboxSvc.CreateDomain(r.Context(), user.ID, req.Name, ip)
	if err != nil {
		if errors.Is(err, services.ErrConflict) {
			WriteError(w, http.StatusConflict, "DOMAIN_EXISTS", err.Error())
			return
		}
		if errors.Is(err, services.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menambahkan domain.")
		return
	}
	WriteSuccess(w, http.StatusCreated, d, "Domain berhasil ditambahkan")
}

func (h *AdminHandler) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "ID domain tidak valid.")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())
	d, err := h.mailboxSvc.UpdateDomainStatus(r.Context(), user.ID, id, req.Status, ip)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Domain tidak ditemukan.")
			return
		}
		if errors.Is(err, services.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengubah status domain.")
		return
	}
	WriteSuccess(w, http.StatusOK, d, "Status domain berhasil diperbarui")
}

func (h *AdminHandler) ListMailboxes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	domainID, _ := strconv.ParseUint(q.Get("domain_id"), 10, 64)

	filter := repositories.MailboxFilter{
		Search:   q.Get("search"),
		Status:   q.Get("status"),
		DomainID: domainID,
		OwnerRef: q.Get("owner_ref"),
		Limit:    limit,
		Offset:   (page - 1) * limit,
	}

	items, total, err := h.mailboxRepo.ListMailboxes(r.Context(), filter)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat daftar mailbox.")
		return
	}

	WriteSuccess(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	}, "OK")
}

func (h *AdminHandler) CreateMailbox(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DomainID      uint64  `json:"domain_id"`
		Mode          string  `json:"mode"`
		LocalPart     string  `json:"local_part"`
		Length        int     `json:"length"`
		ExpiresInDays int     `json:"expires_in_days"`
		OwnerRef      *string `json:"owner_ref"`
		Note          *string `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	mb, err := h.mailboxSvc.CreateMailbox(r.Context(), user.ID, services.CreateMailboxInput{
		DomainID:      req.DomainID,
		Mode:          req.Mode,
		LocalPart:     req.LocalPart,
		Length:        req.Length,
		ExpiresInDays: req.ExpiresInDays,
		OwnerRef:      req.OwnerRef,
		Note:          req.Note,
	}, ip)
	if err != nil {
		if errors.Is(err, services.ErrConflict) {
			WriteError(w, http.StatusConflict, "MAILBOX_EXISTS", err.Error())
			return
		}
		if errors.Is(err, services.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal membuat mailbox.")
		return
	}

	WriteSuccess(w, http.StatusCreated, mb, "Mailbox berhasil dibuat")
}

func (h *AdminHandler) BulkCreateMailboxes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DomainID      uint64   `json:"domain_id"`
		Mode          string   `json:"mode"`
		Addresses     []string `json:"addresses"`
		Prefix        string   `json:"prefix"`
		Start         int      `json:"start"`
		Count         int      `json:"count"`
		Length        int      `json:"length"`
		ExpiresInDays int      `json:"expires_in_days"`
		OwnerRef      *string  `json:"owner_ref"`
		Note          *string  `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	report, err := h.mailboxSvc.BulkCreateMailboxes(r.Context(), user.ID, services.BulkCreateInput{
		DomainID:      req.DomainID,
		Mode:          req.Mode,
		Addresses:     req.Addresses,
		Prefix:        req.Prefix,
		Start:         req.Start,
		Count:         req.Count,
		Length:        req.Length,
		ExpiresInDays: req.ExpiresInDays,
		OwnerRef:      req.OwnerRef,
		Note:          req.Note,
	}, ip)
	if err != nil {
		if errors.Is(err, services.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memproses bulk create.")
		return
	}

	WriteSuccess(w, http.StatusOK, report, "Bulk create selesai diproses")
}

func (h *AdminHandler) BulkActionMailboxes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action     string   `json:"action"`
		IDs        []uint64 `json:"ids"`
		ExtendDays int      `json:"extend_days"`
		Filter     struct {
			OwnerRef string `json:"owner_ref"`
			DomainID uint64 `json:"domain_id"`
		} `json:"filter"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	affected, err := h.mailboxSvc.BulkAction(r.Context(), user.ID, req.Action, req.IDs, req.Filter.OwnerRef, req.Filter.DomainID, req.ExtendDays, ip)
	if err != nil {
		if errors.Is(err, services.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menjalankan aksi massal.")
		return
	}

	WriteSuccess(w, http.StatusOK, map[string]any{
		"action":   req.Action,
		"affected": affected,
	}, "Aksi massal berhasil dijalankan")
}

func (h *AdminHandler) ExportMailboxesCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	domainID, _ := strconv.ParseUint(q.Get("domain_id"), 10, 64)
	filter := repositories.MailboxFilter{
		Search:   q.Get("search"),
		Status:   q.Get("status"),
		DomainID: domainID,
		OwnerRef: q.Get("owner_ref"),
	}

	list, err := h.mailboxRepo.ExportMailboxes(r.Context(), filter)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengekspor data mailbox.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())
	_ = h.adminRepo.CreateAuditLog(r.Context(), &user.ID, "mailbox.export", nil, nil, map[string]any{
		"count":     len(list),
		"domain_id": domainID,
		"owner_ref": filter.OwnerRef,
		"status":    filter.Status,
	}, ip)

	filename := fmt.Sprintf("baramail-mailboxes-%s.csv", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"email", "domain", "status", "effective_status", "expires_at", "owner_ref", "note", "created_at"})
	for _, m := range list {
		ownerRef := ""
		if m.OwnerRef != nil {
			ownerRef = *m.OwnerRef
		}
		note := ""
		if m.Note != nil {
			note = *m.Note
		}
		_ = cw.Write([]string{
			m.Email,
			m.DomainName,
			string(m.Status),
			string(m.EffectiveStatus),
			m.ExpiresAt.UTC().Format(time.RFC3339),
			ownerRef,
			note,
			m.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	cw.Flush()
}

func (h *AdminHandler) GetMailbox(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "ID mailbox tidak valid.")
		return
	}
	mb, err := h.mailboxRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Mailbox tidak ditemukan.")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil detail mailbox.")
		return
	}
	WriteSuccess(w, http.StatusOK, mb, "OK")
}

func (h *AdminHandler) UpdateMailbox(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "ID mailbox tidak valid.")
		return
	}
	var req struct {
		Status        *string    `json:"status"`
		ExpiresInDays *int       `json:"expires_in_days"`
		ExpiresAt     *time.Time `json:"expires_at"`
		ExtendDays    *int       `json:"extend_days"`
		OwnerRef      *string    `json:"owner_ref"`
		Note          *string    `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Format request tidak valid.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	updated, err := h.mailboxSvc.UpdateMailbox(r.Context(), user.ID, id, services.UpdateMailboxInput{
		Status:        req.Status,
		ExpiresInDays: req.ExpiresInDays,
		ExpiresAt:     req.ExpiresAt,
		ExtendDays:    req.ExtendDays,
		OwnerRef:      req.OwnerRef,
		Note:          req.Note,
	}, ip)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Mailbox tidak ditemukan.")
			return
		}
		if errors.Is(err, services.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memperbarui mailbox.")
		return
	}
	WriteSuccess(w, http.StatusOK, updated, "Mailbox berhasil diperbarui")
}

func (h *AdminHandler) DeleteMailbox(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "ID mailbox tidak valid.")
		return
	}

	user := middleware.AdminUserFromContext(r.Context())
	ip := middleware.ClientIPFromContext(r.Context())

	if err := h.mailboxSvc.DeleteMailbox(r.Context(), user.ID, id, ip); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Mailbox tidak ditemukan.")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menghapus mailbox.")
		return
	}
	WriteSuccess(w, http.StatusOK, map[string]any{"id": id}, "Mailbox berhasil dihapus")
}

func (h *AdminHandler) ListMailboxMessages(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "ID mailbox tidak valid.")
		return
	}
	if _, err := h.mailboxRepo.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Mailbox tidak ditemukan.")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat mailbox.")
		return
	}

	q := r.URL.Query()
	if msgIDStr := q.Get("message_id"); msgIDStr != "" {
		msgID, _ := strconv.ParseUint(msgIDStr, 10, 64)
		msg, err := h.msgRepo.GetMessage(r.Context(), id, msgID)
		if err != nil {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Pesan tidak ditemukan.")
			return
		}
		WriteSuccess(w, http.StatusOK, msg, "OK")
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	beforeID, _ := strconv.ParseUint(q.Get("before_id"), 10, 64)
	afterID, _ := strconv.ParseUint(q.Get("after_id"), 10, 64)

	msgs, err := h.msgRepo.ListMessages(r.Context(), id, limit, beforeID, afterID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat pesan mailbox.")
		return
	}
	WriteSuccess(w, http.StatusOK, map[string]any{"items": msgs}, "OK")
}

func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.mailboxRepo.GetStats(r.Context(), time.Now().UTC())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat statistik.")
		return
	}
	WriteSuccess(w, http.StatusOK, stats, "OK")
}

func (h *AdminHandler) AuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 30
	}
	action := strings.TrimSpace(q.Get("action"))

	logs, total, err := h.adminRepo.ListAuditLogs(r.Context(), action, limit, (page-1)*limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memuat audit log.")
		return
	}

	WriteSuccess(w, http.StatusOK, map[string]any{
		"items": logs,
		"total": total,
		"page":  page,
		"limit": limit,
	}, "OK")
}

