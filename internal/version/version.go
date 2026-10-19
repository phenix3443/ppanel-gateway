// Package version compares dotted numeric release versions.
package version

import (
	"strconv"
	"strings"
)

// Compare returns -1, 0, or +1 comparing dotted numeric versions a and b
// (semver-style, a leading "v" is tolerated). Non-numeric segments compare as 0.
func Compare(a, b string) int {
	as := strings.Split(normalize(a), ".")
	bs := strings.Split(normalize(b), ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai = numericPrefix(as[i])
		}
		if i < len(bs) {
			bi = numericPrefix(bs[i])
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

// normalize strips a leading "v" and surrounding space.
func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// numericPrefix parses the leading integer of s (e.g. "3-rc1" -> 3), else 0.
func numericPrefix(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}
