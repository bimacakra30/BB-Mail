package config

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	AppPort         string
	AppBaseURL      string
	AppTimezone     string
	TrustedProxies  []netip.Prefix
	TrustedProxyIPs map[netip.Addr]struct{}

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBParams   string

	AdminSessionSecret   string
	AdminSessionTTL      time.Duration
	AdminInitialUsername string
	AdminInitialPassword string
	InboxTokenSecret     string
	InboxTokenTTL      time.Duration
	WebhookSecret      string
	WebhookSecretPrev  string

	AllowedEmailDomains  []string
	ReservedLocalparts   map[string]struct{}
	RandomLocalpartLen   int
	MaxBulkCreate        int
	MaxMailboxesPerOwner int

	MaxEmailSizeBytes        int64
	InboxPollIntervalSeconds int
	DefaultMailboxExpiryDays int
	MaxMailboxExpiryDays     int
	AllowUnlimitedExpiry     bool

	MessageRetentionHours       int
	MaxMessagesPerMailbox       int
	ExpiredMailboxRetentionDays int
	CleanupInterval             time.Duration

	RateLimitLoginPerMin    int
	RateLimitAccessPerMin   int
	RateLimitMessagesPerMin int
	LoginMaxFailedAttempts  int
	LoginLockoutDuration    time.Duration
}

func Load(envPath string) (*Config, error) {
	if envPath != "" {
		_ = loadDotEnv(envPath)
	} else {
		_ = loadDotEnv(".env")
	}

	trustedPrefixes, trustedIPs := parseTrustedProxies(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1"))

	reservedMap := make(map[string]struct{})
	for _, part := range splitCSV(getEnv("RESERVED_LOCALPARTS", "postmaster,abuse,admin,root,noreply,support,security,hostmaster,webmaster")) {
		reservedMap[strings.ToLower(part)] = struct{}{}
	}

	var domains []string
	for _, d := range splitCSV(getEnv("ALLOWED_EMAIL_DOMAINS", "")) {
		domains = append(domains, strings.ToLower(d))
	}

	sessionTTLHours := getEnvInt("ADMIN_SESSION_TTL_HOURS", 12)
	inboxTTLMinutes := getEnvInt("INBOX_TOKEN_TTL_MINUTES", 30)
	cleanupMinutes := getEnvInt("CLEANUP_INTERVAL_MINUTES", 15)
	lockoutMinutes := getEnvInt("LOGIN_LOCKOUT_MINUTES", 15)

	cfg := &Config{
		AppEnv:          getEnv("APP_ENV", "production"),
		AppPort:         getEnv("APP_PORT", "8080"),
		AppBaseURL:      strings.TrimRight(getEnv("APP_BASE_URL", "http://localhost:8080"), "/"),
		AppTimezone:     getEnv("APP_TIMEZONE", "UTC"),
		TrustedProxies:  trustedPrefixes,
		TrustedProxyIPs: trustedIPs,

		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBName:     getEnv("DB_NAME", "baramail"),
		DBUser:     getEnv("DB_USER", "baramail_user"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBParams:   getEnv("DB_PARAMS", "parseTime=true&loc=UTC&charset=utf8mb4"),

		AdminSessionSecret:   getEnv("ADMIN_SESSION_SECRET", ""),
		AdminSessionTTL:      time.Duration(sessionTTLHours) * time.Hour,
		AdminInitialUsername: getEnv("ADMIN_INITIAL_USERNAME", ""),
		AdminInitialPassword: getEnv("ADMIN_INITIAL_PASSWORD", ""),
		InboxTokenSecret:     getEnv("INBOX_TOKEN_SECRET", ""),
		InboxTokenTTL:      time.Duration(inboxTTLMinutes) * time.Minute,
		WebhookSecret:      getEnv("WEBHOOK_SECRET", ""),
		WebhookSecretPrev:  getEnv("WEBHOOK_SECRET_PREVIOUS", ""),

		AllowedEmailDomains:  domains,
		ReservedLocalparts:   reservedMap,
		RandomLocalpartLen:   getEnvInt("RANDOM_LOCALPART_LENGTH", 12),
		MaxBulkCreate:        getEnvInt("MAX_BULK_CREATE", 200),
		MaxMailboxesPerOwner: getEnvInt("MAX_MAILBOXES_PER_OWNER", 0),

		MaxEmailSizeBytes:        int64(getEnvInt("MAX_EMAIL_SIZE_BYTES", 5242880)),
		InboxPollIntervalSeconds: getEnvInt("INBOX_POLL_INTERVAL_SECONDS", 5),
		DefaultMailboxExpiryDays: getEnvInt("DEFAULT_MAILBOX_EXPIRY_DAYS", 7),
		MaxMailboxExpiryDays:     getEnvInt("MAX_MAILBOX_EXPIRY_DAYS", 365),
		AllowUnlimitedExpiry:     getEnvBool("ALLOW_UNLIMITED_EXPIRY", false),

		MessageRetentionHours:       getEnvInt("MESSAGE_RETENTION_HOURS", 72),
		MaxMessagesPerMailbox:       getEnvInt("MAX_MESSAGES_PER_MAILBOX", 100),
		ExpiredMailboxRetentionDays: getEnvInt("EXPIRED_MAILBOX_RETENTION_DAYS", 7),
		CleanupInterval:             time.Duration(cleanupMinutes) * time.Minute,

		RateLimitLoginPerMin:    getEnvInt("RATE_LIMIT_LOGIN_PER_MIN", 5),
		RateLimitAccessPerMin:   getEnvInt("RATE_LIMIT_ACCESS_PER_MIN", 10),
		RateLimitMessagesPerMin: getEnvInt("RATE_LIMIT_MESSAGES_PER_MIN", 30),
		LoginMaxFailedAttempts:  getEnvInt("LOGIN_MAX_FAILED_ATTEMPTS", 5),
		LoginLockoutDuration:    time.Duration(lockoutMinutes) * time.Minute,
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBParams)
}

func (c *Config) IsTrustedProxy(ipStr string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ipStr))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	if _, ok := c.TrustedProxyIPs[addr]; ok {
		return true
	}
	for _, prefix := range c.TrustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseTrustedProxies(raw string) ([]netip.Prefix, map[netip.Addr]struct{}) {
	var prefixes []netip.Prefix
	ips := make(map[netip.Addr]struct{})
	for _, item := range splitCSV(raw) {
		if strings.Contains(item, "/") {
			if p, err := netip.ParsePrefix(item); err == nil {
				prefixes = append(prefixes, p)
			}
			continue
		}
		if addr, err := netip.ParseAddr(item); err == nil {
			ips[addr.Unmap()] = struct{}{}
		}
	}
	return prefixes, ips
}

func splitCSV(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return fallback
	}
	return b
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		v = strings.Trim(v, `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	return scanner.Err()
}

