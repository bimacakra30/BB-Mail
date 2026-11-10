package tests

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/mailparse"
	"baramail/backend/internal/middleware"
	"baramail/backend/internal/models"
	"baramail/backend/internal/services"
	"baramail/backend/internal/validators"
)

func TestValidatorsAndNormalization(t *testing.T) {
	reserved := map[string]struct{}{
		"admin":      {},
		"postmaster": {},
		"abuse":      {},
	}

	if err := validators.ValidateLocalPart("budi.santoso_01-a", reserved); err != nil {
		t.Fatalf("expected valid local-part, got error: %v", err)
	}

	invalidCases := []string{
		"",
		".leadingdot",
		"trailingdot.",
		"double..dot",
		"plus+tag",
		"space here",
		"admin",
		"POSTMASTER",
		strings.Repeat("a", 65),
	}
	for _, tc := range invalidCases {
		if err := validators.ValidateLocalPart(tc, reserved); err == nil {
			t.Errorf("expected local-part %q to be rejected", tc)
		}
	}

	lp, dom, full, err := validators.NormalizeEmail("  BuDi.01@Domain-A.TLD  ")
	if err != nil {
		t.Fatalf("unexpected normalize error: %v", err)
	}
	if lp != "budi.01" || dom != "domain-a.tld" || full != "budi.01@domain-a.tld" {
		t.Fatalf("unexpected normalized values: lp=%q dom=%q full=%q", lp, dom, full)
	}

	seen := make(map[string]bool)
	for i := 0; i < 25; i++ {
		rnd, err := validators.GenerateRandomLocalPart(12)
		if err != nil {
			t.Fatalf("GenerateRandomLocalPart failed: %v", err)
		}
		if len(rnd) != 12 {
			t.Fatalf("expected length 12, got %d", len(rnd))
		}
		if strings.ContainsAny(rnd, "01oli") {
			t.Fatalf("random local-part %q contains ambiguous characters", rnd)
		}
		seen[rnd] = true
	}
	if len(seen) < 20 {
		t.Fatalf("expected high entropy random local-parts, got %d unique out of 25", len(seen))
	}
}

func TestMailboxExpirationAndAccessibility(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	mb := &models.Mailbox{
		ID:           1,
		Email:        "budi@domain.tld",
		Status:       models.MailboxStatusActive,
		DomainStatus: models.DomainStatusActive,
		ExpiresAt:    now.Add(1 * time.Hour),
	}

	if !mb.IsAccessible(now) {
		t.Fatal("expected active non-expired mailbox on active domain to be accessible")
	}

	// Expired exactly at ExpiresAt.
	if mb.IsAccessible(mb.ExpiresAt) {
		t.Fatal("expected mailbox at expires_at to be considered expired")
	}
	if mb.ComputeEffectiveStatus(mb.ExpiresAt) != models.MailboxStatusExpired {
		t.Fatalf("expected effective status expired, got %s", mb.ComputeEffectiveStatus(mb.ExpiresAt))
	}

	// Retired domain blocks access without modifying mailbox row status.
	mb.DomainStatus = models.DomainStatusRetired
	if mb.IsAccessible(now) {
		t.Fatal("expected mailbox on retired domain to be inaccessible")
	}
}

func TestMIMEParserAndHTMLFallback(t *testing.T) {
	htmlOnlyMIME := strings.Join([]string{
		"From: Layanan Verifikasi <no-reply@service.example>",
		"To: budi@domain.tld",
		"Subject: =?UTF-8?B?S29kZSBPVFAgQW5kYQ==?=",
		"Message-ID: <msg-001@service.example>",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		`<html><head><style>.hidden{display:none}</style><script>alert(1)</script></head>`,
		`<body><p>Halo Budi,</p><p>Kode OTP Anda adalah <b>849201</b>.</p>`,
		`<p>Atau klik <a href="https://service.example/verify?token=abc123xyz">Verifikasi Akun</a> untuk melanjutkan.</p>`,
		`<a href="javascript:alert(1)">Jangan Klik</a></body></html>`,
	}, "\r\n")

	parsed, err := mailparse.ParseRawMIME([]byte(htmlOnlyMIME), "bounce@service.example")
	if err != nil {
		t.Fatalf("ParseRawMIME failed on HTML-only email: %v", err)
	}
	if parsed.BodySource != models.BodySourceHTMLConverted {
		t.Fatalf("expected body_source html_converted, got %s", parsed.BodySource)
	}
	if parsed.Subject != "Kode OTP Anda" {
		t.Fatalf("expected decoded subject 'Kode OTP Anda', got %q", parsed.Subject)
	}
	if !strings.Contains(parsed.BodyText, "849201") {
		t.Fatalf("expected OTP code in converted text, got %q", parsed.BodyText)
	}
	if !strings.Contains(parsed.BodyText, "Verifikasi Akun (https://service.example/verify?token=abc123xyz)") {
		t.Fatalf("expected verification URL preserved in converted text, got %q", parsed.BodyText)
	}
	if strings.Contains(parsed.BodyText, "alert(1)") || strings.Contains(parsed.BodyText, "javascript:") {
		t.Fatalf("expected script and javascript: URL stripped, got %q", parsed.BodyText)
	}

	// Multipart alternative emails should prefer the text/plain part over text/html.
	multipartMIME := strings.Join([]string{
		"From: sender@example.com",
		"To: user@domain.tld",
		"Content-Type: multipart/alternative; boundary=\"sep123\"",
		"",
		"--sep123",
		"Content-Type: text/plain; charset=iso-8859-1",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Kode verifikasi: 112233",
		"--sep123",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>HTML version 112233</p>",
		"--sep123--",
	}, "\r\n")

	parsedMulti, err := mailparse.ParseRawMIME([]byte(multipartMIME), "sender@example.com")
	if err != nil {
		t.Fatalf("ParseRawMIME failed on multipart email: %v", err)
	}
	if parsedMulti.BodySource != models.BodySourceText {
		t.Fatalf("expected body_source text, got %s", parsedMulti.BodySource)
	}
	if parsedMulti.BodyText != "Kode verifikasi: 112233" {
		t.Fatalf("unexpected body_text: %q", parsedMulti.BodyText)
	}
}

func TestDedupeKeyIsolationAndDeterminism(t *testing.T) {
	raw := []byte("From: a@b.com\r\n\r\nOTP 123456")
	k1 := mailparse.ComputeDedupeKey(raw, "User1@Domain.tld")
	k2 := mailparse.ComputeDedupeKey(raw, "  user1@domain.tld ")
	k3 := mailparse.ComputeDedupeKey(raw, "user2@domain.tld")

	if !bytes.Equal(k1, k2) {
		t.Fatal("expected identical dedupe key for same raw MIME and normalized recipient")
	}
	if bytes.Equal(k1, k3) {
		t.Fatal("expected different dedupe keys when same raw MIME is sent to two different recipients")
	}
}

func TestAuthPasswordAndInboxTokenIsolation(t *testing.T) {
	hash, err := services.HashPassword("S3cretP@ssw0rd!")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !services.VerifyPassword("S3cretP@ssw0rd!", hash) {
		t.Fatal("expected VerifyPassword to succeed for valid password")
	}
	if services.VerifyPassword("WrongPassword", hash) {
		t.Fatal("expected VerifyPassword to fail for wrong password")
	}

	cfg := &config.Config{
		InboxTokenSecret: "test-inbox-secret-32-bytes-long!!",
		InboxTokenTTL:    30 * time.Minute,
	}
	authSvc := services.NewAuthService(cfg, nil)

	mbExpires := time.Now().UTC().Add(10 * time.Minute)
	mb := &models.Mailbox{
		ID:        42,
		Email:     "k7x2m9qpa4zt@domain.tld",
		ExpiresAt: mbExpires,
	}

	tok, exp, err := authSvc.IssueInboxToken(mb)
	if err != nil {
		t.Fatalf("IssueInboxToken failed: %v", err)
	}
	// Token expiry must be capped by mailbox ExpiresAt (10m < 30m).
	if exp.After(mbExpires) {
		t.Fatalf("expected token expiry %v not to exceed mailbox expiry %v", exp, mbExpires)
	}

	claims, err := authSvc.VerifyInboxToken(tok)
	if err != nil {
		t.Fatalf("VerifyInboxToken failed: %v", err)
	}
	if claims.MailboxID != 42 || claims.Email != "k7x2m9qpa4zt@domain.tld" {
		t.Fatalf("unexpected token claims: %+v", claims)
	}

	// Tampering with token must fail verification.
	if _, err := authSvc.VerifyInboxToken(tok + "a"); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestClientIPAndWebhookSecretRotation(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,172.16.0.0/12")
	t.Setenv("WEBHOOK_SECRET", "current-secret-value")
	t.Setenv("WEBHOOK_SECRET_PREVIOUS", "previous-secret-value")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	// Request from trusted proxy honors CF-Connecting-IP.
	reqTrusted := httptest.NewRequest("GET", "/api/v1/health", nil)
	reqTrusted.RemoteAddr = "172.18.0.5:43210"
	reqTrusted.Header.Set("CF-Connecting-IP", "203.0.113.55")
	if got := middleware.ExtractClientIP(reqTrusted, cfg); got != "203.0.113.55" {
		t.Fatalf("expected 203.0.113.55 from trusted proxy, got %s", got)
	}

	// Request from untrusted IP ignores spoofed CF-Connecting-IP.
	reqUntrusted := httptest.NewRequest("GET", "/api/v1/health", nil)
	reqUntrusted.RemoteAddr = "198.51.100.99:54321"
	reqUntrusted.Header.Set("CF-Connecting-IP", "1.2.3.4")
	if got := middleware.ExtractClientIP(reqUntrusted, cfg); got != "198.51.100.99" {
		t.Fatalf("expected untrusted remote addr 198.51.100.99, got %s", got)
	}

	ingSvc := services.NewIngestionService(cfg, nil, nil)
	if !ingSvc.VerifySecret("current-secret-value") {
		t.Fatal("expected current webhook secret to be accepted")
	}
	if !ingSvc.VerifySecret("previous-secret-value") {
		t.Fatal("expected previous webhook secret to be accepted during rotation")
	}
	if ingSvc.VerifySecret("wrong-secret") {
		t.Fatal("expected wrong webhook secret to be rejected")
	}
}
