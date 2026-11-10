package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/models"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/validators"
)

var (
	ErrMailboxNotAvailable = errors.New("mailbox tidak tersedia")
	ErrValidation          = errors.New("validasi gagal")
	ErrConflict            = errors.New("data sudah ada")
)

type CreateMailboxInput struct {
	DomainID      uint64
	Mode          string
	LocalPart     string
	Length        int
	ExpiresInDays int
	OwnerRef      *string
	Note          *string
	CreatedVia    models.CreatedVia
}

type BulkCreateInput struct {
	DomainID      uint64
	Mode          string
	Addresses     []string
	Prefix        string
	Start         int
	Count         int
	Length        int
	ExpiresInDays int
	OwnerRef      *string
	Note          *string
}

type BulkItemResult struct {
	Input     string `json:"input,omitempty"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	MailboxID uint64 `json:"mailbox_id,omitempty"`
}

type BulkCreateReport struct {
	TotalRequested int              `json:"total_requested"`
	CreatedCount   int              `json:"created_count"`
	SkippedCount   int              `json:"skipped_count"`
	InvalidCount   int              `json:"invalid_count"`
	Items          []BulkItemResult `json:"items"`
}

type UpdateMailboxInput struct {
	Status        *string
	ExpiresInDays *int
	ExpiresAt     *time.Time
	ExtendDays    *int
	OwnerRef      *string
	Note          *string
}

type MailboxService struct {
	cfg         *config.Config
	domainRepo  *repositories.DomainRepository
	mailboxRepo *repositories.MailboxRepository
	msgRepo     *repositories.MessageRepository
	adminRepo   *repositories.AdminRepository
}

func NewMailboxService(
	cfg *config.Config,
	domainRepo *repositories.DomainRepository,
	mailboxRepo *repositories.MailboxRepository,
	msgRepo *repositories.MessageRepository,
	adminRepo *repositories.AdminRepository,
) *MailboxService {
	return &MailboxService{
		cfg:         cfg,
		domainRepo:  domainRepo,
		mailboxRepo: mailboxRepo,
		msgRepo:     msgRepo,
		adminRepo:   adminRepo,
	}
}

func (s *MailboxService) ListDomains(ctx context.Context) ([]models.Domain, error) {
	return s.domainRepo.ListDomains(ctx)
}

func (s *MailboxService) CreateDomain(ctx context.Context, adminID uint64, name, ip string) (*models.Domain, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if err := validators.ValidateDomainName(name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	d, err := s.domainRepo.CreateDomain(ctx, name)
	if err != nil {
		if errors.Is(err, repositories.ErrDuplicateEntry) {
			return nil, fmt.Errorf("%w: domain sudah terdaftar", ErrConflict)
		}
		return nil, err
	}
	targetType := "domain"
	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "domain.create", &targetType, &d.ID, map[string]any{"name": d.Name}, ip)
	return d, nil
}

func (s *MailboxService) UpdateDomainStatus(ctx context.Context, adminID, domainID uint64, status string, ip string) (*models.Domain, error) {
	st := models.DomainStatus(strings.ToLower(strings.TrimSpace(status)))
	if st != models.DomainStatusActive && st != models.DomainStatusRetired {
		return nil, fmt.Errorf("%w: status domain harus 'active' atau 'retired'", ErrValidation)
	}
	d, err := s.domainRepo.UpdateStatus(ctx, domainID, st)
	if err != nil {
		return nil, err
	}
	targetType := "domain"
	action := "domain.activate"
	if st == models.DomainStatusRetired {
		action = "domain.retire"
	}
	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, action, &targetType, &d.ID, map[string]any{"name": d.Name, "status": d.Status}, ip)
	return d, nil
}

func (s *MailboxService) CreateMailbox(ctx context.Context, adminID uint64, in CreateMailboxInput, ip string) (*models.Mailbox, error) {
	domain, err := s.domainRepo.GetByID(ctx, in.DomainID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, fmt.Errorf("%w: domain tidak ditemukan", ErrValidation)
		}
		return nil, err
	}
	if domain.Status != models.DomainStatusActive {
		return nil, fmt.Errorf("%w: mailbox hanya dapat dibuat pada domain aktif", ErrValidation)
	}

	expiresDays := in.ExpiresInDays
	if expiresDays == 0 {
		expiresDays = s.cfg.DefaultMailboxExpiryDays
	}
	if expiresDays < 1 || (expiresDays > s.cfg.MaxMailboxExpiryDays && !s.cfg.AllowUnlimitedExpiry) {
		return nil, fmt.Errorf("%w: masa aktif harus antara 1 hingga %d hari", ErrValidation, s.cfg.MaxMailboxExpiryDays)
	}

	ownerRef := cleanOptionalString(in.OwnerRef, 100)
	note := cleanOptionalString(in.Note, 255)

	if ownerRef != nil && s.cfg.MaxMailboxesPerOwner > 0 {
		count, err := s.mailboxRepo.CountByOwnerRef(ctx, *ownerRef)
		if err != nil {
			return nil, err
		}
		if count >= s.cfg.MaxMailboxesPerOwner {
			return nil, fmt.Errorf("%w: batas maksimum mailbox untuk owner '%s' (%d) telah tercapai", ErrValidation, *ownerRef, s.cfg.MaxMailboxesPerOwner)
		}
	}

	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if mode == "" {
		mode = "custom"
	}
	createdVia := in.CreatedVia
	if createdVia == "" {
		createdVia = models.CreatedViaAdmin
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expiresDays) * 24 * time.Hour)

	var created *models.Mailbox
	switch mode {
	case "custom":
		lp := strings.ToLower(strings.TrimSpace(in.LocalPart))
		if err := validators.ValidateLocalPart(lp, s.cfg.ReservedLocalparts); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		fullEmail := lp + "@" + domain.Name
		if len(fullEmail) > 254 {
			return nil, fmt.Errorf("%w: panjang alamat email melebihi 254 karakter", ErrValidation)
		}
		mb := &models.Mailbox{
			DomainID:     domain.ID,
			DomainName:   domain.Name,
			DomainStatus: domain.Status,
			LocalPart:    lp,
			Email:        fullEmail,
			Status:       models.MailboxStatusActive,
			ExpiresAt:    expiresAt,
			OwnerRef:     ownerRef,
			Note:         note,
			CreatedVia:   createdVia,
		}
		created, err = s.mailboxRepo.CreateMailbox(ctx, mb)
		if err != nil {
			if errors.Is(err, repositories.ErrDuplicateEntry) {
				return nil, fmt.Errorf("%w: alamat email sudah terdaftar", ErrConflict)
			}
			return nil, err
		}

	case "random":
		length := in.Length
		if length == 0 {
			length = s.cfg.RandomLocalpartLen
		}
		for attempt := 0; attempt < 10; attempt++ {
			lp, err := validators.GenerateRandomLocalPart(length)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrValidation, err)
			}
			if err := validators.ValidateLocalPart(lp, s.cfg.ReservedLocalparts); err != nil {
				continue
			}
			fullEmail := lp + "@" + domain.Name
			mb := &models.Mailbox{
				DomainID:     domain.ID,
				DomainName:   domain.Name,
				DomainStatus: domain.Status,
				LocalPart:    lp,
				Email:        fullEmail,
				Status:       models.MailboxStatusActive,
				ExpiresAt:    expiresAt,
				OwnerRef:     ownerRef,
				Note:         note,
				CreatedVia:   createdVia,
			}
			created, err = s.mailboxRepo.CreateMailbox(ctx, mb)
			if errors.Is(err, repositories.ErrDuplicateEntry) {
				continue
			}
			if err != nil {
				return nil, err
			}
			break
		}
		if created == nil {
			return nil, errors.New("gagal menghasilkan alamat unik setelah beberapa percobaan")
		}

	default:
		return nil, fmt.Errorf("%w: mode harus 'custom' atau 'random'", ErrValidation)
	}

	if createdVia == models.CreatedViaAdmin {
		targetType := "mailbox"
		_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "mailbox.create", &targetType, &created.ID, map[string]any{
			"email":      created.Email,
			"mode":       mode,
			"expires_at": created.ExpiresAt,
			"owner_ref":  created.OwnerRef,
		}, ip)
	}

	return created, nil
}

func (s *MailboxService) BulkCreateMailboxes(ctx context.Context, adminID uint64, in BulkCreateInput, ip string) (*BulkCreateReport, error) {
	domain, err := s.domainRepo.GetByID(ctx, in.DomainID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, fmt.Errorf("%w: domain tidak ditemukan", ErrValidation)
		}
		return nil, err
	}
	if domain.Status != models.DomainStatusActive {
		return nil, fmt.Errorf("%w: mailbox hanya dapat dibuat pada domain aktif", ErrValidation)
	}

	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	var requestedCount int
	switch mode {
	case "list":
		requestedCount = len(in.Addresses)
	case "sequence", "random":
		requestedCount = in.Count
	default:
		return nil, fmt.Errorf("%w: mode bulk harus 'list', 'sequence', atau 'random'", ErrValidation)
	}

	if requestedCount < 1 || requestedCount > s.cfg.MaxBulkCreate {
		return nil, fmt.Errorf("%w: jumlah bulk create harus antara 1 hingga %d", ErrValidation, s.cfg.MaxBulkCreate)
	}

	report := &BulkCreateReport{
		TotalRequested: requestedCount,
		Items:          make([]BulkItemResult, 0, requestedCount),
	}

	switch mode {
	case "list":
		for _, raw := range in.Addresses {
			lp := strings.ToLower(strings.TrimSpace(raw))
			// Allow pasting either local-part or local-part@matching-domain.
			if idx := strings.IndexByte(lp, '@'); idx > 0 {
				domPart := lp[idx+1:]
				if domPart != domain.Name {
					report.InvalidCount++
					report.Items = append(report.Items, BulkItemResult{
						Input:  raw,
						Status: "invalid",
						Reason: "domain pada alamat tidak sesuai dengan domain yang dipilih",
					})
					continue
				}
				lp = lp[:idx]
			}
			s.processBulkSingleItem(ctx, adminID, domain, "custom", lp, in.Length, in.ExpiresInDays, in.OwnerRef, in.Note, raw, report, ip)
		}

	case "sequence":
		prefix := strings.ToLower(strings.TrimSpace(in.Prefix))
		if prefix == "" {
			return nil, fmt.Errorf("%w: prefix wajib diisi untuk mode sequence", ErrValidation)
		}
		start := in.Start
		if start < 1 {
			start = 1
		}
		for i := 0; i < in.Count; i++ {
			lp := fmt.Sprintf("%s%d", prefix, start+i)
			s.processBulkSingleItem(ctx, adminID, domain, "custom", lp, in.Length, in.ExpiresInDays, in.OwnerRef, in.Note, lp, report, ip)
		}

	case "random":
		for i := 0; i < in.Count; i++ {
			s.processBulkSingleItem(ctx, adminID, domain, "random", "", in.Length, in.ExpiresInDays, in.OwnerRef, in.Note, "", report, ip)
		}
	}

	targetType := "domain"
	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "mailbox.bulk_create", &targetType, &domain.ID, map[string]any{
		"mode":      mode,
		"requested": report.TotalRequested,
		"created":   report.CreatedCount,
		"skipped":   report.SkippedCount,
		"invalid":   report.InvalidCount,
		"owner_ref": in.OwnerRef,
	}, ip)

	return report, nil
}

func (s *MailboxService) processBulkSingleItem(
	ctx context.Context,
	adminID uint64,
	domain *models.Domain,
	mode string,
	localPart string,
	length int,
	expiresInDays int,
	ownerRef *string,
	note *string,
	rawInput string,
	report *BulkCreateReport,
	ip string,
) {
	mb, err := s.CreateMailbox(ctx, adminID, CreateMailboxInput{
		DomainID:      domain.ID,
		Mode:          mode,
		LocalPart:     localPart,
		Length:        length,
		ExpiresInDays: expiresInDays,
		OwnerRef:      ownerRef,
		Note:          note,
		CreatedVia:    models.CreatedViaBulk,
	}, ip)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			report.SkippedCount++
			report.Items = append(report.Items, BulkItemResult{
				Input:  rawInput,
				Email:  localPart + "@" + domain.Name,
				Status: "skipped_duplicate",
				Reason: "alamat sudah terdaftar",
			})
			return
		}
		report.InvalidCount++
		report.Items = append(report.Items, BulkItemResult{
			Input:  rawInput,
			Status: "invalid",
			Reason: err.Error(),
		})
		return
	}

	report.CreatedCount++
	report.Items = append(report.Items, BulkItemResult{
		Input:     rawInput,
		Email:     mb.Email,
		Status:    "created",
		MailboxID: mb.ID,
	})
}

func (s *MailboxService) UpdateMailbox(ctx context.Context, adminID, mailboxID uint64, in UpdateMailboxInput, ip string) (*models.Mailbox, error) {
	current, err := s.mailboxRepo.GetByID(ctx, mailboxID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	newStatus := current.Status
	if in.Status != nil {
		st := models.MailboxStatus(strings.ToLower(strings.TrimSpace(*in.Status)))
		if st != models.MailboxStatusActive && st != models.MailboxStatusDisabled {
			return nil, fmt.Errorf("%w: status harus 'active' atau 'disabled'", ErrValidation)
		}
		newStatus = st
	}

	newExpiresAt := current.ExpiresAt
	if in.ExpiresAt != nil {
		newExpiresAt = in.ExpiresAt.UTC()
	} else if in.ExpiresInDays != nil {
		if *in.ExpiresInDays < 1 || (*in.ExpiresInDays > s.cfg.MaxMailboxExpiryDays && !s.cfg.AllowUnlimitedExpiry) {
			return nil, fmt.Errorf("%w: masa aktif harus antara 1 hingga %d hari", ErrValidation, s.cfg.MaxMailboxExpiryDays)
		}
		newExpiresAt = now.Add(time.Duration(*in.ExpiresInDays) * 24 * time.Hour)
	} else if in.ExtendDays != nil {
		if *in.ExtendDays < 1 || *in.ExtendDays > s.cfg.MaxMailboxExpiryDays {
			return nil, fmt.Errorf("%w: perpanjangan harus antara 1 hingga %d hari", ErrValidation, s.cfg.MaxMailboxExpiryDays)
		}
		base := current.ExpiresAt
		if base.Before(now) {
			base = now
		}
		newExpiresAt = base.Add(time.Duration(*in.ExtendDays) * 24 * time.Hour)
	}

	// FR-006: cannot re-enable a mailbox whose expiration time is already in the past.
	if newStatus == models.MailboxStatusActive && !now.Before(newExpiresAt) {
		return nil, fmt.Errorf("%w: mailbox yang sudah kedaluwarsa harus diperpanjang masa aktifnya terlebih dahulu", ErrValidation)
	}

	newOwnerRef := current.OwnerRef
	if in.OwnerRef != nil {
		newOwnerRef = cleanOptionalString(in.OwnerRef, 100)
	}
	newNote := current.Note
	if in.Note != nil {
		newNote = cleanOptionalString(in.Note, 255)
	}

	updated, err := s.mailboxRepo.UpdateMailbox(ctx, mailboxID, newStatus, newExpiresAt, newOwnerRef, newNote)
	if err != nil {
		return nil, err
	}

	targetType := "mailbox"
	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "mailbox.update", &targetType, &mailboxID, map[string]any{
		"email":      updated.Email,
		"status":     updated.Status,
		"expires_at": updated.ExpiresAt,
		"owner_ref":  updated.OwnerRef,
	}, ip)

	return updated, nil
}

func (s *MailboxService) DeleteMailbox(ctx context.Context, adminID, mailboxID uint64, ip string) error {
	mb, err := s.mailboxRepo.GetByID(ctx, mailboxID)
	if err != nil {
		return err
	}
	if err := s.mailboxRepo.DeleteMailbox(ctx, mailboxID); err != nil {
		return err
	}
	targetType := "mailbox"
	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "mailbox.delete", &targetType, &mailboxID, map[string]any{"email": mb.Email}, ip)
	return nil
}

func (s *MailboxService) BulkAction(ctx context.Context, adminID uint64, action string, ids []uint64, ownerRef string, domainID uint64, extendDays int, ip string) (int64, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "extend" && (extendDays < 1 || extendDays > s.cfg.MaxMailboxExpiryDays) {
		return 0, fmt.Errorf("%w: extend_days harus antara 1 hingga %d", ErrValidation, s.cfg.MaxMailboxExpiryDays)
	}
	affected, err := s.mailboxRepo.BulkAction(ctx, action, ids, strings.TrimSpace(ownerRef), domainID, extendDays)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "mailbox.bulk_action", nil, nil, map[string]any{
		"action":      action,
		"ids_count":   len(ids),
		"owner_ref":   ownerRef,
		"domain_id":   domainID,
		"extend_days": extendDays,
		"affected":    affected,
	}, ip)
	return affected, nil
}

// OpenInbox validates public inbox access and returns a uniform error on any failure to prevent enumeration.
func (s *MailboxService) OpenInbox(ctx context.Context, rawEmail string) (*models.Mailbox, error) {
	_, _, normalized, err := validators.NormalizeEmail(rawEmail)
	if err != nil {
		return nil, ErrMailboxNotAvailable
	}
	mb, err := s.mailboxRepo.GetByEmail(ctx, normalized)
	if err != nil {
		return nil, ErrMailboxNotAvailable
	}
	if !mb.IsAccessible(time.Now().UTC()) {
		return nil, ErrMailboxNotAvailable
	}
	return mb, nil
}

// GetAccessibleMailboxByID reloads the mailbox from DB on every tokenized request per FR-008.
func (s *MailboxService) GetAccessibleMailboxByID(ctx context.Context, id uint64) (*models.Mailbox, error) {
	mb, err := s.mailboxRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrMailboxNotAvailable
	}
	if !mb.IsAccessible(time.Now().UTC()) {
		return nil, ErrMailboxNotAvailable
	}
	return mb, nil
}

func cleanOptionalString(val *string, maxLen int) *string {
	if val == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*val)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > maxLen {
		trimmed = trimmed[:maxLen]
	}
	return &trimmed
}

