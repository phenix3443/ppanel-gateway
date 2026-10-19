package proxy

import "testing"

func TestMatchPrefixLongestWins(t *testing.T) {
	r := NewRouter()
	r.routes["/api"] = nil
	r.routes["/api/v2"] = nil
	r.routes["/sub"] = nil

	cases := map[string]string{
		"/api/v2/users": "/api/v2",
		"/api/users":    "/api",
		"/sub/abc":      "/sub",
		"/nope":         "",
	}
	for path, want := range cases {
		if got := r.matchPrefix(path); got != want {
			t.Errorf("matchPrefix(%q)=%q want %q", path, got, want)
		}
	}
}
