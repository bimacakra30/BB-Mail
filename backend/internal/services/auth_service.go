package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/models"
	"baramail/backend/internal/repositories"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrAccountLocked      = errors.New("akun dikunci sementara karena terlalu banyak percobaan gagal")
	ErrInvalidSession     = errors.New("sesi tidak valid atau telah berakhir")
	ErrInvalidToken       = errors.New("token akses tidak valid atau telah berakhir")
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
)

type InboxTokenClaims struct {
	MailboxID uint64 `json:"mid"`
	Email     string `json:"sub"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

type AuthService struct {
	cfg       *config.Config
	adminRepo *repositories.AdminRepository
	dummyHash string
}

func NewAuthService(cfg *config.Config, adminRepo *repositories.AdminRepository) *AuthService {
	dummy, _ := HashPassword("dummy-anti-timing-password")
	return &AuthService{
		cfg:       cfg,
		adminRepo: adminRepo,
		dummyHash: dummy,
	}
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

func VerifyPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, timeCost uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads)
	if err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(computed, expectedHash) == 1
}

func (s *AuthService) Login(ctx context.Context, username, password, ip, userAgent string) (*models.AdminUser, *models.AdminSession, string, error) {
	username = strings.TrimSpace(username)
	user, err := s.adminRepo.GetByUsername(ctx, username)
	if err != nil {
		// Perform dummy verification so non-existent usernames take similar time.
		_ = VerifyPassword(password, s.dummyHash)
		_ = s.adminRepo.CreateAuditLog(ctx, nil, "admin.login.failed", nil, nil, map[string]any{"reason": "invalid_credentials"}, ip)
		return nil, nil, "", ErrInvalidCredentials
	}

	now := time.Now().UTC()
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		_ = VerifyPassword(password, s.dummyHash)
		_ = s.adminRepo.CreateAuditLog(ctx, &user.ID, "admin.login.locked", nil, nil, nil, ip)
		return nil, nil, "", ErrAccountLocked
	}

	if !VerifyPassword(password, user.PasswordHash) {
		_ = s.adminRepo.RecordFailedLogin(ctx, user.ID, s.cfg.LoginMaxFailedAttempts, s.cfg.LoginLockoutDuration)
		_ = s.adminRepo.CreateAuditLog(ctx, &user.ID, "admin.login.failed", nil, nil, map[string]any{"reason": "bad_password"}, ip)
		return nil, nil, "", ErrInvalidCredentials
	}

	_ = s.adminRepo.ResetFailedLogin(ctx, user.ID)

	sessionID, err := randomHex(32)
	if err != nil {
		return nil, nil, "", err
	}
	csrfToken, err := randomHex(32)
	if err != nil {
		return nil, nil, "", err
	}

	if len(userAgent) > 250 {
		userAgent = userAgent[:250]
	}

	session := &models.AdminSession{
		ID:          sessionID,
		AdminUserID: user.ID,
		CSRFToken:   csrfToken,
		IP:          ip,
		UserAgent:   userAgent,
		ExpiresAt:   now.Add(s.cfg.AdminSessionTTL),
		CreatedAt:   now,
	}
	if err := s.adminRepo.CreateSession(ctx, session); err != nil {
		return nil, nil, "", err
	}

	_ = s.adminRepo.CreateAuditLog(ctx, &user.ID, "admin.login.success", nil, nil, nil, ip)
	cookieVal := s.signSessionID(sessionID)
	return user, session, cookieVal, nil
}

func (s *AuthService) ValidateSessionCookie(ctx context.Context, cookieVal string) (*models.AdminUser, *models.AdminSession, error) {
	sessionID, ok := s.verifySignedSessionID(cookieVal)
	if !ok {
		return nil, nil, ErrInvalidSession
	}
	session, err := s.adminRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, nil, ErrInvalidSession
	}
	if !time.Now().UTC().Before(session.ExpiresAt) {
		_ = s.adminRepo.DeleteSession(ctx, sessionID)
		return nil, nil, ErrInvalidSession
	}
	user, err := s.adminRepo.GetByID(ctx, session.AdminUserID)
	if err != nil {
		return nil, nil, ErrInvalidSession
	}
	return user, session, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string, adminID uint64, ip string) error {
	err := s.adminRepo.DeleteSession(ctx, sessionID)
	_ = s.adminRepo.CreateAuditLog(ctx, &adminID, "admin.logout", nil, nil, nil, ip)
	return err
}

func (s *AuthService) IssueInboxToken(mailbox *models.Mailbox) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(s.cfg.InboxTokenTTL)
	// FR-008: token expiry must never exceed the mailbox expiration timestamp.
	if exp.After(mailbox.ExpiresAt) {
		exp = mailbox.ExpiresAt
	}

	claims := InboxTokenClaims{
		MailboxID: mailbox.ID,
		Email:     mailbox.Email,
		ExpiresAt: exp.Unix(),
		IssuedAt:  now.Unix(),
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := s.computeHMAC(payloadB64, s.cfg.InboxTokenSecret)
	return payloadB64 + "." + sig, exp, nil
}

func (s *AuthService) VerifyInboxToken(token string) (*InboxTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}
	expectedSig := s.computeHMAC(parts[0], s.cfg.InboxTokenSecret)
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expectedSig)) != 1 {
		return nil, ErrInvalidToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims InboxTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if time.Now().UTC().Unix() >= claims.ExpiresAt {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func (s *AuthService) signSessionID(sessionID string) string {
	sig := s.computeHMAC(sessionID, s.cfg.AdminSessionSecret)
	return sessionID + "." + sig
}

func (s *AuthService) verifySignedSessionID(cookieVal string) (string, bool) {
	parts := strings.Split(cookieVal, ".")
	if len(parts) != 2 || parts[0] == "" {
		return "", false
	}
	expected := s.computeHMAC(parts[0], s.cfg.AdminSessionSecret)
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) != 1 {
		return "", false
	}
	return parts[0], true
}

func (s *AuthService) computeHMAC(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

