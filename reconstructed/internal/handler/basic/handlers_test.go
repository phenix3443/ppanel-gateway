package basic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ppanel-gateway/reconstructed/internal/module"
	"ppanel-gateway/reconstructed/internal/types"
)

// fakeRel satisfies module's (unexported) releaseSource via structural typing.
type fakeRel struct{}

func (fakeRel) LatestVersion(context.Context, string) (string, error)          { return "v9.9.9", nil }
func (fakeRel) SystemAssetURL(context.Context, string, string) (string, error) { return "", nil }
func (fakeRel) Download(context.Context, string, string) error                 { return nil }

func newMux(t *testing.T) (*http.ServeMux, *module.Manager) {
	t.Helper()
	mgr := module.NewManager(t.TempDir(), fakeRel{})
	d := &Deps{Manager: mgr}
	mux := http.NewServeMux()
	mux.Handle("POST /basic/register", RegisterServiceHandler(d))
	mux.Handle("GET /basic/heartbeat", HeartbeatHandler(d))
	mux.Handle("GET /basic/check/version", CheckServiceVersionHandler(d))
	mux.Handle("POST /basic/update", UpdateServiceHandler(d))
	return mux, mgr
}

func do(t *testing.T, mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestControlPlaneContract(t *testing.T) {
	mux, _ := newMux(t)

	reg := `{"service_name":"ppanel-server","secret":"s3cr3t","repository":"https://github.com/perfect-panel/server","service_version":"1.0.0"}`
	if rr := do(t, mux, "POST", "/basic/register", reg); rr.Code != http.StatusOK {
		t.Fatalf("register: %d %s", rr.Code, rr.Body)
	}

	// heartbeat: correct secret -> 200 alive=true (healthy on registration).
	rr := do(t, mux, "GET", "/basic/heartbeat?service_name=ppanel-server&secret=s3cr3t", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("heartbeat ok: %d %s", rr.Code, rr.Body)
	}
	var hb map[string]bool
	_ = json.Unmarshal(rr.Body.Bytes(), &hb)
	if !hb["alive"] {
		t.Fatalf("expected alive=true, got %v", hb)
	}

	// wrong secret -> 401.
	if rr := do(t, mux, "GET", "/basic/heartbeat?service_name=ppanel-server&secret=nope", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("heartbeat wrong secret: want 401 got %d", rr.Code)
	}
	// unknown module -> 404.
	if rr := do(t, mux, "GET", "/basic/heartbeat?service_name=ghost&secret=s3cr3t", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("heartbeat unknown: want 404 got %d", rr.Code)
	}

	// check/version: registered v1.0.0 vs latest v9.9.9 -> update needed.
	rr = do(t, mux, "GET", "/basic/check/version?service_name=ppanel-server&secret=s3cr3t", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("check: %d %s", rr.Code, rr.Body)
	}
	var cv types.ServiceVersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cv); err != nil {
		t.Fatal(err)
	}
	if cv.CurrentVersion != "v1.0.0" || cv.LatestVersion != "v9.9.9" || !cv.UpdateNeeded {
		t.Fatalf("check response wrong: %+v", cv)
	}
}
