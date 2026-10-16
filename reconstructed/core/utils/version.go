// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Mirrors updater/core/utils. Symbols [E]: CompareVersion, normalizeVersion,
// detectSystemKeyword, parseRepoURL, Release.BuildDownloadURL. Bodies [I]:
// reimplemented from the observed GitHub-release URL template and go-release
// asset-naming conventions; original bodies were not decompiled.
package utils

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// normalizeVersion strips a leading "v" and surrounding space. [E] symbol.
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// CompareVersion returns -1, 0, or +1 comparing dotted numeric versions a and b
// (semver-style, leading "v" tolerated). Non-numeric segments compare as 0.
// [E] symbol CompareVersion; ordering semantics [I].
func CompareVersion(a, b string) int {
	as := strings.Split(normalizeVersion(a), ".")
	bs := strings.Split(normalizeVersion(b), ".")
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

// BuildDownloadURL renders the observed release-asset URL template
// https://github.com/<owner>/<repo>/releases/download/<tag>/<asset> [E].
func BuildDownloadURL(owner, repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, asset)
}

// detectSystemKeyword returns the release-asset keyword for the running
// platform, "<goos>-<goarch>" (e.g. "linux-amd64"). [E] — decompiled
// (updater/core/utils.detectSystemKeyword @0x7a0f80 builds Sprintf("linux-%s",
// "amd64"); general form is GOOS-GOARCH).
func detectSystemKeyword() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// GetSystemAssetURL picks, from a release's asset names, the download URL whose
// asset name contains the system keyword from detectSystemKeyword. [E] —
// decompiled (updater/core/utils.(*Release).GetSystemAssetURL @0x7a1080 calls
// detectSystemKeyword then matches an asset by that keyword).
func GetSystemAssetURL(owner, repo, tag string, assetNames []string) (string, bool) {
	kw := detectSystemKeyword()
	for _, name := range assetNames {
		if strings.Contains(name, kw) {
			return BuildDownloadURL(owner, repo, tag, name), true
		}
	}
	return "", false
}

// parseRepoURL splits a github.com repository URL into owner and repo.
// [E] symbol parseRepoURL.
func parseRepoURL(repoURL string) (owner, repo string, err error) {
	s := strings.TrimSuffix(repoURL, "/")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "github.com/")
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from %q", repoURL)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}
