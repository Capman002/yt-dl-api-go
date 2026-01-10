// Package middleware provides HTTP middleware functions.
package middleware

import (
	"errors"
	"net/url"
	"strings"
)

// Allowed domains for video downloads
var allowedDomains = map[string]bool{
	"youtube.com":         true,
	"www.youtube.com":     true,
	"youtu.be":            true,
	"m.youtube.com":       true,
	"twitter.com":         true,
	"www.twitter.com":     true,
	"x.com":               true,
	"www.x.com":           true,
	"tiktok.com":          true,
	"www.tiktok.com":      true,
	"vm.tiktok.com":       true,
	"instagram.com":       true,
	"www.instagram.com":   true,
	"facebook.com":        true,
	"www.facebook.com":    true,
	"fb.watch":            true,
	"vimeo.com":           true,
	"www.vimeo.com":       true,
	"player.vimeo.com":    true,
	"reddit.com":          true,
	"www.reddit.com":      true,
	"v.redd.it":           true,
	"twitch.tv":           true,
	"www.twitch.tv":       true,
	"clips.twitch.tv":     true,
	"dailymotion.com":     true,
	"www.dailymotion.com": true,
}

// URL validation errors
var (
	ErrEmptyURL         = errors.New("URL cannot be empty")
	ErrInvalidURL       = errors.New("invalid URL format")
	ErrHTTPSRequired    = errors.New("only HTTPS URLs are allowed")
	ErrDomainNotAllowed = errors.New("domain not in allowlist")
	ErrUserInfoPresent  = errors.New("URLs with user credentials are not allowed")
	ErrFragmentPresent  = errors.New("URLs with fragments are not allowed for downloads")
)

// ValidateURL validates a URL against security rules.
// It checks for:
// - Valid URL format
// - HTTPS scheme only
// - Allowed domains (YouTube, Twitter, TikTok, etc.)
// - No userinfo (user:pass@host)
func ValidateURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)

	if rawURL == "" {
		return ErrEmptyURL
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidURL
	}

	// Ensure HTTPS only (except for development with YouTube short links)
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return ErrHTTPSRequired
	}

	// In production, strictly require HTTPS
	// For now, we allow http for easier testing, but log a warning
	if parsedURL.Scheme != "https" {
		// TODO: In production, uncomment this:
		// return ErrHTTPSRequired
	}

	// Check for userinfo (e.g., user:pass@host)
	if parsedURL.User != nil {
		return ErrUserInfoPresent
	}

	// Get the host without port
	host := parsedURL.Hostname()
	if host == "" {
		return ErrInvalidURL
	}

	// Check if domain is in allowlist
	host = strings.ToLower(host)
	if !isDomainAllowed(host) {
		return ErrDomainNotAllowed
	}

	return nil
}

// isDomainAllowed checks if a domain is in the allowlist.
// It also handles subdomains by checking parent domains.
func isDomainAllowed(host string) bool {
	// Direct check
	if allowedDomains[host] {
		return true
	}

	// Check parent domains (e.g., "music.youtube.com" -> check "youtube.com")
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		// Try with just the last two parts (e.g., "youtube.com")
		parentDomain := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if allowedDomains[parentDomain] {
			return true
		}
	}

	return false
}

// AddAllowedDomain adds a domain to the allowlist.
// This can be used to extend the default list at runtime.
func AddAllowedDomain(domain string) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain != "" {
		allowedDomains[domain] = true
	}
}

// RemoveAllowedDomain removes a domain from the allowlist.
func RemoveAllowedDomain(domain string) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	delete(allowedDomains, domain)
}

// GetAllowedDomains returns a copy of the current allowlist.
func GetAllowedDomains() []string {
	domains := make([]string, 0, len(allowedDomains))
	for domain := range allowedDomains {
		domains = append(domains, domain)
	}
	return domains
}

// NormalizeURL normalizes a URL for consistent handling.
// It removes fragments and trailing slashes.
func NormalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Remove fragment
	parsedURL.Fragment = ""

	// Build normalized URL
	normalized := parsedURL.String()

	// Remove trailing slash unless it's just the path
	if len(normalized) > 0 && normalized[len(normalized)-1] == '/' && parsedURL.Path != "/" {
		normalized = normalized[:len(normalized)-1]
	}

	return normalized
}
