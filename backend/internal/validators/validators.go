package validators

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// Excludes 0, 1, o, l, i to avoid visual ambiguity in generated addresses.
const randomAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

var (
	domainRegex    = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	ErrInvalidChar = errors.New("local-part hanya boleh berisi a-z, 0-9, titik (.), garis bawah (_), atau tanda hubung (-)")
)

func NormalizeEmail(raw string) (localPart string, domain string, fullEmail string, err error) {
	cleaned := strings.ToLower(strings.TrimSpace(raw))
	if cleaned == "" || len(cleaned) > 254 {
		return "", "", "", errors.New("format alamat email tidak valid")
	}
	atIdx := strings.LastIndexByte(cleaned, '@')
	if atIdx <= 0 || atIdx == len(cleaned)-1 || strings.IndexByte(cleaned, '@') != atIdx {
		return "", "", "", errors.New("format alamat email tidak valid")
	}
	localPart = cleaned[:atIdx]
	domain = cleaned[atIdx+1:]
	if err := ValidateLocalPart(localPart, nil); err != nil {
		return "", "", "", err
	}
	if err := ValidateDomainName(domain); err != nil {
		return "", "", "", err
	}
	return localPart, domain, localPart + "@" + domain, nil
}

func ValidateLocalPart(localPart string, reserved map[string]struct{}) error {
	lp := strings.ToLower(strings.TrimSpace(localPart))
	if len(lp) < 1 || len(lp) > 64 {
		return errors.New("panjang local-part harus antara 1 hingga 64 karakter")
	}
	if strings.Contains(lp, "+") {
		return errors.New("karakter '+' tidak diizinkan pada local-part")
	}
	if strings.HasPrefix(lp, ".") || strings.HasSuffix(lp, ".") {
		return errors.New("local-part tidak boleh diawali atau diakhiri dengan titik")
	}
	if strings.Contains(lp, "..") {
		return errors.New("local-part tidak boleh memiliki titik berurutan")
	}
	for i := 0; i < len(lp); i++ {
		c := lp[i]
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isAllowedSymbol := c == '.' || c == '_' || c == '-'
		if !isLower && !isDigit && !isAllowedSymbol {
			return ErrInvalidChar
		}
	}
	if reserved != nil {
		if _, isReserved := reserved[lp]; isReserved {
			return fmt.Errorf("local-part '%s' termasuk dalam daftar reserved", lp)
		}
	}
	return nil
}

func ValidateDomainName(domain string) error {
	d := strings.ToLower(strings.TrimSpace(domain))
	if len(d) < 3 || len(d) > 190 {
		return errors.New("panjang nama domain tidak valid")
	}
	if !domainRegex.MatchString(d) {
		return errors.New("format nama domain tidak valid")
	}
	return nil
}

func GenerateRandomLocalPart(length int) (string, error) {
	if length < 6 || length > 64 {
		return "", errors.New("panjang alamat acak harus antara 6 hingga 64 karakter")
	}
	out := make([]byte, length)
	max := big.NewInt(int64(len(randomAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = randomAlphabet[n.Int64()]
	}
	return string(out), nil
}

