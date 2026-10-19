package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRepoURL(t *testing.T) {
	cases := []struct {
		in      string
		owner   string
		repo    string
		wantErr bool
	}{
		{"https://github.com/perfect-panel/server", "perfect-panel", "server", false},
		{"https://github.com/perfect-panel/server/", "perfect-panel", "server", false},
		{"https://github.com/perfect-panel/frontend", "perfect-panel", "frontend", false},
		{"perfect-panel/server", "perfect-panel", "server", false}, // host-less path
		{"https://github.com/onlyowner", "", "", true},
		{"https://gitlab.com/a/b", "", "", true},
		{"", "", "", true},
	}
	for _, c := range cases {
		o, r, err := ParseRepoURL(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("%q: err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if c.wantErr {
			continue
		}
		if o != c.owner || r != c.repo {
			t.Fatalf("%q: got %s/%s want %s/%s", c.in, o, r, c.owner, c.repo)
		}
	}
}

func TestSystemAssetURL(t *testing.T) {
	kw := SystemKeyword()
	rel := &Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: "ppanel-server-windows-amd64.zip", DownloadURL: "https://x/win"},
			{Name: "ppanel-server-" + kw + ".tar.gz", DownloadURL: "https://x/match"},
		},
	}
	got, err := rel.SystemAssetURL()
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://x/match" {
		t.Fatalf("got %q", got)
	}
	// No matching asset -> error, not a fabricated URL.
	if _, err := (&Release{Assets: []Asset{{Name: "only-darwin-arm64"}}}).SystemAssetURL(); err == nil {
		// darwin-arm64 would match on that host; guard the assertion.
		if kw != "darwin-arm64" {
			t.Fatal("expected error when no asset matches the system keyword")
		}
	}
}

// LatestVersion hits the upstream once and serves the cache afterwards.
func TestLatestVersionCaches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/perfect-panel/server/releases/latest" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		calls++
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.20.3"})
	}))
	defer srv.Close()

	c := NewClient().WithAPI(srv.URL)
	for i := 0; i < 5; i++ {
		v, err := c.LatestVersion(context.Background(), "https://github.com/perfect-panel/server")
		if err != nil {
			t.Fatal(err)
		}
		if v != "v1.20.3" {
			t.Fatalf("got %q", v)
		}
	}
	if calls != 1 {
		t.Fatalf("hit upstream %d times, want 1 (30m cache)", calls)
	}
}

func TestLatestReleaseStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient().WithAPI(srv.URL)
	if _, err := c.LatestRelease(context.Background(), "x", "y"); err == nil {
		t.Fatal("expected error on 404")
	}
}
