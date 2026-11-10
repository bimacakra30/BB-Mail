package mailparse

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"baramail/backend/internal/models"

	xhtml "golang.org/x/net/html"
	"golang.org/x/text/encoding/ianaindex"
)

var (
	ErrUnparseableMIME = errors.New("pesan MIME rusak dan tidak dapat diproses")
	multiSpaceRe       = regexp.MustCompile(`[ \t]+`)
	multiNewlineRe     = regexp.MustCompile(`\n{3,}`)
)

type ParsedEmail struct {
	MessageID  *string
	Sender     string
	Subject    string
	BodyText   string
	BodySource models.BodySource
}

// ComputeDedupeKey follows PRD 9.4: SHA-256(raw MIME bytes || 0x0A || normalized envelope recipient).
func ComputeDedupeKey(rawMIME []byte, envelopeRecipient string) []byte {
	h := sha256.New()
	h.Write(rawMIME)
	h.Write([]byte{0x0A})
	h.Write([]byte(strings.ToLower(strings.TrimSpace(envelopeRecipient))))
	return h.Sum(nil)
}

func ParseRawMIME(raw []byte, fallbackSender string) (*ParsedEmail, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, ErrUnparseableMIME
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		// Malformed headers that still contain readable text should be salvaged when possible.
		if utf8.Valid(trimmed) && bytes.Contains(trimmed, []byte("\n\n")) {
			parts := bytes.SplitN(trimmed, []byte("\n\n"), 2)
			body := strings.TrimSpace(string(parts[1]))
			if body != "" {
				return &ParsedEmail{
					Sender:     fallbackSender,
					Subject:    "",
					BodyText:   body,
					BodySource: models.BodySourceText,
				}, nil
			}
		}
		return nil, fmt.Errorf("%w: %v", ErrUnparseableMIME, err)
	}

	dec := new(mime.WordDecoder)
	dec.CharsetReader = charsetReader

	subject := decodeHeader(dec, msg.Header.Get("Subject"))
	fromHeader := decodeHeader(dec, msg.Header.Get("From"))
	sender := extractAddressOrRaw(fromHeader, fallbackSender)

	var msgID *string
	if rawID := strings.TrimSpace(msg.Header.Get("Message-ID")); rawID != "" {
		if len(rawID) > 998 {
			rawID = rawID[:998]
		}
		msgID = &rawID
	}

	plainParts, htmlParts, partErr := extractParts(msg.Header, msg.Body)
	if partErr != nil && len(plainParts) == 0 && len(htmlParts) == 0 {
		return nil, fmt.Errorf("%w: %v", ErrUnparseableMIME, partErr)
	}

	plainText := strings.TrimSpace(strings.Join(plainParts, "\n\n"))
	if plainText != "" {
		return &ParsedEmail{
			MessageID:  msgID,
			Sender:     sender,
			Subject:    subject,
			BodyText:   plainText,
			BodySource: models.BodySourceText,
		}, nil
	}

	htmlRaw := strings.TrimSpace(strings.Join(htmlParts, "\n\n"))
	if htmlRaw != "" {
		converted := strings.TrimSpace(HTMLToText(htmlRaw))
		if converted != "" {
			return &ParsedEmail{
				MessageID:  msgID,
				Sender:     sender,
				Subject:    subject,
				BodyText:   converted,
				BodySource: models.BodySourceHTMLConverted,
			}, nil
		}
	}

	return &ParsedEmail{
		MessageID:  msgID,
		Sender:     sender,
		Subject:    subject,
		BodyText:   "",
		BodySource: models.BodySourceEmpty,
	}, nil
}

type headerGetter interface {
	Get(key string) string
}

func extractParts(hdr headerGetter, body io.Reader) (plain []string, htmlBodies []string, err error) {
	ct := hdr.Get("Content-Type")
	if ct == "" {
		ct = "text/plain; charset=utf-8"
	}
	mediaType, params, parseErr := mime.ParseMediaType(ct)
	if parseErr != nil {
		mediaType = "text/plain"
		params = map[string]string{"charset": "utf-8"}
	}
	mediaType = strings.ToLower(mediaType)

	// Skip attachments on MVP.
	if disp := hdr.Get("Content-Disposition"); disp != "" {
		dispType, _, _ := mime.ParseMediaType(disp)
		if strings.EqualFold(dispType, "attachment") {
			_, _ = io.Copy(io.Discard, body)
			return nil, nil, nil
		}
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return nil, nil, errors.New("missing multipart boundary")
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, partErr := mr.NextPart()
			if errors.Is(partErr, io.EOF) {
				break
			}
			if partErr != nil {
				break
			}
			pPlain, pHTML, _ := extractParts(part.Header, part)
			plain = append(plain, pPlain...)
			htmlBodies = append(htmlBodies, pHTML...)
			_ = part.Close()
		}
		return plain, htmlBodies, nil
	}

	decodedReader := wrapTransferEncoding(hdr.Get("Content-Transfer-Encoding"), body)
	rawBytes, readErr := io.ReadAll(decodedReader)
	if readErr != nil && len(rawBytes) == 0 {
		return nil, nil, readErr
	}

	text := decodeCharset(rawBytes, params["charset"])
	switch mediaType {
	case "text/plain":
		return []string{text}, nil, nil
	case "text/html":
		return nil, []string{text}, nil
	default:
		return nil, nil, nil
	}
}

func wrapTransferEncoding(cte string, r io.Reader) io.Reader {
	switch strings.ToLower(strings.TrimSpace(cte)) {
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	case "base64":
		return &lenientBase64Reader{src: r}
	default:
		return r
	}
}

type lenientBase64Reader struct {
	src     io.Reader
	decoded io.Reader
}

func (l *lenientBase64Reader) Read(p []byte) (int, error) {
	if l.decoded == nil {
		raw, err := io.ReadAll(l.src)
		if err != nil && len(raw) == 0 {
			return 0, err
		}
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, raw)
		out := make([]byte, base64.StdEncoding.DecodedLen(len(cleaned)))
		n, decErr := base64.StdEncoding.Decode(out, cleaned)
		if decErr != nil && n == 0 {
			out, _ = base64.RawStdEncoding.DecodeString(string(cleaned))
			l.decoded = bytes.NewReader(out)
		} else {
			l.decoded = bytes.NewReader(out[:n])
		}
	}
	return l.decoded.Read(p)
}

func decodeCharset(data []byte, charset string) string {
	cs := strings.ToLower(strings.TrimSpace(charset))
	if cs == "" || cs == "utf-8" || cs == "utf8" || cs == "us-ascii" || cs == "ascii" {
		return normalizeNewlines(string(data))
	}
	enc, err := ianaindex.MIME.Encoding(cs)
	if err != nil || enc == nil {
		return normalizeNewlines(string(data))
	}
	decoded, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return normalizeNewlines(string(data))
	}
	return normalizeNewlines(string(decoded))
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	cs := strings.ToLower(strings.TrimSpace(charset))
	if cs == "" || cs == "utf-8" || cs == "utf8" || cs == "us-ascii" || cs == "ascii" {
		return input, nil
	}
	enc, err := ianaindex.MIME.Encoding(cs)
	if err != nil || enc == nil {
		return input, nil
	}
	return enc.NewDecoder().Reader(input), nil
}

func decodeHeader(dec *mime.WordDecoder, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	decoded, err := dec.DecodeHeader(raw)
	if err != nil {
		return raw
	}
	return strings.TrimSpace(decoded)
}

func extractAddressOrRaw(fromHeader, fallback string) string {
	if fromHeader == "" {
		return fallback
	}
	if addr, err := mail.ParseAddress(fromHeader); err == nil && addr.Address != "" {
		if addr.Name != "" {
			return fmt.Sprintf("%s <%s>", addr.Name, strings.ToLower(addr.Address))
		}
		return strings.ToLower(addr.Address)
	}
	return fromHeader
}

// HTMLToText strips HTML tags, drops script/style content, and preserves http/https link URLs.
func HTMLToText(htmlContent string) string {
	doc, err := xhtml.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return html.UnescapeString(htmlContent)
	}

	var buf strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "script" || tag == "style" || tag == "head" || tag == "noscript" {
				return
			}
			if isBlockElement(tag) {
				buf.WriteString("\n")
			}
			if tag == "br" {
				buf.WriteString("\n")
				return
			}
			if tag == "li" {
				buf.WriteString("\n- ")
			}
			if tag == "a" {
				var href string
				for _, attr := range n.Attr {
					if strings.EqualFold(attr.Key, "href") {
						href = strings.TrimSpace(attr.Val)
						break
					}
				}
				var linkTextBuf strings.Builder
				collectText(n, &linkTextBuf)
				linkText := strings.TrimSpace(multiSpaceRe.ReplaceAllString(linkTextBuf.String(), " "))
				if isHTTPURL(href) {
					if linkText == "" || linkText == href {
						buf.WriteString(" ")
						buf.WriteString(href)
						buf.WriteString(" ")
					} else {
						buf.WriteString(" ")
						buf.WriteString(linkText)
						buf.WriteString(" (")
						buf.WriteString(href)
						buf.WriteString(") ")
					}
				} else if linkText != "" {
					buf.WriteString(" ")
					buf.WriteString(linkText)
					buf.WriteString(" ")
				}
				return
			}
		}

		if n.Type == xhtml.TextNode {
			buf.WriteString(n.Data)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == xhtml.ElementNode && isBlockElement(strings.ToLower(n.Data)) {
			buf.WriteString("\n")
		}
	}

	walk(doc)

	raw := html.UnescapeString(buf.String())
	raw = normalizeNewlines(raw)
	lines := strings.Split(raw, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(multiSpaceRe.ReplaceAllString(l, " "))
	}
	joined := strings.Join(lines, "\n")
	joined = multiNewlineRe.ReplaceAllString(joined, "\n\n")
	return strings.TrimSpace(joined)
}

func collectText(n *xhtml.Node, out *strings.Builder) {
	if n.Type == xhtml.TextNode {
		out.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(c, out)
	}
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "section", "article", "header", "footer", "tr", "table",
		"h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "ul", "ol":
		return true
	default:
		return false
	}
}

func isHTTPURL(u string) bool {
	lower := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

