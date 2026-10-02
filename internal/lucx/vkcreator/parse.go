// Package vkcreator provides in-process VK call creation for qWDTT vk_hash.
//
// Portions adapted from https://github.com/ildarmaga/wdtt (pkg/vkhash, panel/vk_*),
// originally under GPL-3.0. Kept minimal and clearly marked; see CREDITS.md.
package vkcreator

import "strings"

const (
	MaxHashes   = 4
	JoinURLBase = "https://vk.ru/call/join/"
)

// StripOne extracts a bare token from a hash or …/call/join/… URL.
func StripOne(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if idx := strings.Index(lower, "/call/join/"); idx >= 0 {
		s = s[idx+len("/call/join/"):]
	} else if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ""
	} else {
		prefixes := []string{
			"https://vk.ru/call/join/", "http://vk.ru/call/join/",
			"https://vk.com/call/join/", "http://vk.com/call/join/",
			"https://m.vk.ru/call/join/", "http://m.vk.ru/call/join/",
			"https://m.vk.com/call/join/", "http://m.vk.com/call/join/",
			"m.vk.ru/call/join/", "vk.ru/call/join/",
			"m.vk.com/call/join/", "vk.com/call/join/",
			"https://vk.me/join/", "http://vk.me/join/", "vk.me/join/",
		}
		for _, p := range prefixes {
			if strings.HasPrefix(lower, p) {
				s = s[len(p):]
				break
			}
		}
	}
	if i := strings.IndexAny(s, "?#/"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(s, "/ ")
}

// Parse returns up to maxBare bare hashes (0 = MaxHashes).
func Parse(raw string, maxBare int) []string {
	if maxBare <= 0 {
		maxBare = MaxHashes
	}
	var out []string
	seen := make(map[string]struct{})
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		h := StripOne(part)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
		if len(out) >= maxBare {
			break
		}
	}
	return out
}

// Normalize stores bare tokens joined by commas (up to MaxHashes).
func Normalize(raw string) string {
	return strings.Join(Parse(raw, MaxHashes), ",")
}

// MergeHashes appends added into existing, deduped, capped at MaxHashes.
func MergeHashes(existing, added string) string {
	merged := Normalize(existing)
	add := Normalize(added)
	if add == "" {
		return merged
	}
	if merged == "" {
		return add
	}
	parts := append(Parse(merged, 0), Parse(add, 0)...)
	seen := make(map[string]struct{})
	var out []string
	for _, p := range parts {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
		if len(out) >= MaxHashes {
			break
		}
	}
	return strings.Join(out, ",")
}
