// Package github looks up module releases on the GitHub REST API, selects the
// asset for the running platform, and downloads it. Latest-version lookups are
// cached per repository for cacheTTL.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// cacheTTL is the latest-version cache window.
const cacheTTL = 30 * time.Minute

// SystemKeyword is "<goos>-<goarch>", e.g. "linux-amd64". It is the substring
// a release asset's name must contain to be selected for the running platform.
func SystemKeyword() string { return runtime.GOOS + "-" + runtime.GOARCH }

// ParseRepoURL splits a GitHub repository URL into owner and repo. A trailing
// "/" is stripped and the path must yield at least two segments.
func ParseRepoURL(repoURL string) (owner, repo string, err error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repoURL), "/")
	u, perr := url.Parse(trimmed)
	if perr != nil {
		return "", "", fmt.Errorf("parse repo url %q: %w", repoURL, perr)
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "github.com") &&
		!strings.EqualFold(u.Host, "www.github.com") {
		return "", "", fmt.Errorf("not a GitHub repository url: %q", repoURL)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repository url missing owner/repo: %q", repoURL)
	}
	return parts[0], parts[1], nil
}

// Asset is one published release asset.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Release is the subset of the GitHub release object the gateway reads.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// SystemAssetURL returns the download URL of the asset whose name contains the
// running system's keyword, erroring when none matches.
func (r *Release) SystemAssetURL() (string, error) {
	kw := SystemKeyword()
	for _, a := range r.Assets {
		if strings.Contains(a.Name, kw) {
			return a.DownloadURL, nil
		}
	}
	return "", fmt.Errorf("no release asset matches system keyword %q", kw)
}

// Client fetches releases from the GitHub REST API, caching the latest version
// per repo for cacheTTL. The zero value is not usable; call NewClient.
type Client struct {
	api  string
	http *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry // key "owner/repo"
}

type cacheEntry struct {
	version string
	at      time.Time
}

// NewClient returns a Client pointed at the public GitHub API.
func NewClient() *Client {
	return &Client{
		api:   "https://api.github.com",
		http:  &http.Client{Timeout: 30 * time.Second},
		cache: make(map[string]cacheEntry),
	}
}

// WithAPI overrides the API base (used by tests to point at a mock server).
func (c *Client) WithAPI(base string) *Client { c.api = strings.TrimSuffix(base, "/"); return c }

// LatestRelease fetches the latest release of owner/repo (uncached).
func (c *Client) LatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.api, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github latest release: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode github release: %w", err)
	}
	return &rel, nil
}

// LatestVersion returns the latest release tag for the repo URL, served from a
// cacheTTL cache: a cache hit within the window returns the stored value,
// otherwise it refetches and restamps.
func (c *Client) LatestVersion(ctx context.Context, repoURL string) (string, error) {
	owner, repo, err := ParseRepoURL(repoURL)
	if err != nil {
		return "", err
	}
	key := owner + "/" + repo

	c.mu.Lock()
	if e, ok := c.cache[key]; ok && e.version != "" && time.Since(e.at) < cacheTTL {
		v := e.version
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	rel, err := c.LatestRelease(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.cache[key] = cacheEntry{version: rel.TagName, at: time.Now()}
	c.mu.Unlock()
	return rel.TagName, nil
}

// SystemAssetURL fetches the latest release of the repo URL and returns the
// download URL of the asset matching the running system keyword.
func (c *Client) SystemAssetURL(ctx context.Context, repoURL, _ string) (string, error) {
	owner, repo, err := ParseRepoURL(repoURL)
	if err != nil {
		return "", err
	}
	rel, err := c.LatestRelease(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return rel.SystemAssetURL()
}

// Download writes the asset at url to dest (created/truncated).
func (c *Client) Download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}
