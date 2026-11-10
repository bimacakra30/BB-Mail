package handlers

import (
	"errors"
	"io"
	"net/http"

	"baramail/backend/internal/config"
	"baramail/backend/internal/services"
)

type WebhookHandler struct {
	cfg          *config.Config
	ingestionSvc *services.IngestionService
}

func NewWebhookHandler(cfg *config.Config, ingestionSvc *services.IngestionService) *WebhookHandler {
	return &WebhookHandler{
		cfg:          cfg,
		ingestionSvc: ingestionSvc,
	}
}

func (h *WebhookHandler) ReceiveEmail(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxEmailSizeBytes+1024)
	rawMIME, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran email melebihi batas maksimum.")
			return
		}
		WriteError(w, http.StatusBadRequest, "INVALID_MESSAGE", "Gagal membaca body email.")
		return
	}

	res, err := h.ingestionSvc.IngestEmail(r.Context(), services.IngestInput{
		Secret:        r.Header.Get("X-BaraMail-Secret"),
		EnvelopeTo:    r.Header.Get("X-BaraMail-Envelope-To"),
		EnvelopeFrom:  r.Header.Get("X-BaraMail-Envelope-From"),
		ReceivedAtRaw: r.Header.Get("X-BaraMail-Received-At"),
		RawMIME:       rawMIME,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrWebhookUnauthorized):
			WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi webhook gagal.")
		case errors.Is(err, services.ErrPayloadTooLarge):
			WriteError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran email melebihi batas maksimum.")
		case errors.Is(err, services.ErrMailboxNotAvailable):
			WriteError(w, http.StatusNotFound, "MAILBOX_NOT_AVAILABLE", "Mailbox tidak tersedia.")
		case errors.Is(err, services.ErrInvalidMIME):
			WriteError(w, http.StatusUnprocessableEntity, "INVALID_MESSAGE", "Format pesan MIME tidak valid.")
		default:
			WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Kesalahan internal saat menyimpan email.")
		}
		return
	}

	if res.Status == "duplicate" {
		WriteSuccess(w, http.StatusOK, res, "OK")
		return
	}
	WriteSuccess(w, http.StatusCreated, res, "Stored")
}

