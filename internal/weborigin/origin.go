package weborigin

import (
	"fmt"
	"net/url"
	"strings"
)

func Canonicalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("origin is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse origin: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("origin must be an absolute URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	if u.User != nil {
		return "", fmt.Errorf("origin must not include user info")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origin must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("origin must not include query or fragment")
	}

	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func CanonicalizeList(rawOrigins []string) ([]string, error) {
	seen := make(map[string]bool, len(rawOrigins))
	origins := make([]string, 0, len(rawOrigins))
	for _, raw := range rawOrigins {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		origin, err := Canonicalize(raw)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", raw, err)
		}
		if seen[origin] {
			continue
		}
		seen[origin] = true
		origins = append(origins, origin)
	}
	return origins, nil
}

func FromURL(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("URL must be absolute")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https")
	}
	if u.User != nil {
		return "", fmt.Errorf("URL must not include user info")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func Contains(origins []string, origin string) bool {
	canonical, err := Canonicalize(origin)
	if err != nil {
		return false
	}
	for _, item := range origins {
		if item == canonical {
			return true
		}
	}
	return false
}
