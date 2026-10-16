package utils

import "testing"

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.2.0", "1.1.9", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0", "1.9.9", 1},
		{"1.0.0", "v1.0.0", 0},
		{"1.2.3-rc1", "1.2.3", 0}, // numeric prefix only
	}
	for _, c := range cases {
		if got := CompareVersion(c.a, c.b); got != c.want {
			t.Errorf("CompareVersion(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestBuildDownloadURL(t *testing.T) {
	got := BuildDownloadURL("perfect-panel", "server", "v0.1.18", "ppanel-server-linux-amd64")
	want := "https://github.com/perfect-panel/server/releases/download/v0.1.18/ppanel-server-linux-amd64"
	if got != want {
		t.Errorf("BuildDownloadURL = %q want %q", got, want)
	}
}

func TestGetSystemAssetURL(t *testing.T) {
	kw := detectSystemKeyword() // e.g. linux-amd64
	assets := []string{"ppanel-server-darwin-arm64", "ppanel-server-" + kw, "checksums.txt"}
	got, ok := GetSystemAssetURL("perfect-panel", "server", "v0.1.18", assets)
	if !ok {
		t.Fatalf("expected a match for keyword %q", kw)
	}
	want := "https://github.com/perfect-panel/server/releases/download/v0.1.18/ppanel-server-" + kw
	if got != want {
		t.Errorf("GetSystemAssetURL = %q want %q", got, want)
	}
	if _, ok := GetSystemAssetURL("o", "r", "t", []string{"no-match"}); ok {
		t.Error("expected no match")
	}
}

func TestParseRepoURL(t *testing.T) {
	owner, repo, err := parseRepoURL("https://github.com/perfect-panel/frontend")
	if err != nil || owner != "perfect-panel" || repo != "frontend" {
		t.Fatalf("parseRepoURL = %q,%q,%v", owner, repo, err)
	}
}
