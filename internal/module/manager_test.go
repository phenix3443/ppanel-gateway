package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// fakeReleases is an injected releaseSource: it serves a fixed latest version
// and copies a prebuilt tarball on Download (no network).
type fakeReleases struct {
	latest    string
	tarball   string
	downloads int
}

func (f *fakeReleases) LatestVersion(_ context.Context, _ string) (string, error) {
	return f.latest, nil
}
func (f *fakeReleases) SystemAssetURL(_ context.Context, _, _ string) (string, error) {
	return "stub://" + f.tarball, nil
}
func (f *fakeReleases) Download(_ context.Context, _, dest string) error {
	f.downloads++
	in, err := os.Open(f.tarball)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// stubServerTarGz writes a tar.gz whose single entry is an executable named
// "ppanel-server" that sleeps (so the supervisor sees a live child).
func stubServerTarGz(t *testing.T, marker string) string {
	t.Helper()
	script := "#!/bin/sh\necho " + marker + "\nexec sleep 300\n"
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: "ppanel-server", Mode: 0o755, Size: int64(len(script)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(script)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), marker+".tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUpdateModuleInstallsAndStarts(t *testing.T) {
	work := t.TempDir()
	fake := &fakeReleases{latest: "v2.0.0", tarball: stubServerTarGz(t, "v2")}
	mgr := NewManager(work, fake)
	mgr.RegisterModule(&Info{
		ServiceName:    "ppanel-server",
		Repository:     "https://github.com/perfect-panel/server",
		ServiceVersion: "v1.0.0",
		Kind:           KindService,
	})
	t.Cleanup(func() { _ = mgr.supervisor("ppanel-server").Stop() })

	if err := mgr.UpdateModule(context.Background(), "ppanel-server"); err != nil {
		t.Fatalf("update: %v", err)
	}
	bin := filepath.Join(work, "modules", "ppanel-server")
	st, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("installed binary not executable: %v", st.Mode())
	}
	if !mgr.supervisor("ppanel-server").Running() || mgr.supervisor("ppanel-server").PID() == 0 {
		t.Fatal("child not running after update")
	}
	if info, _ := mgr.FindModule("ppanel-server"); info.ServiceVersion != "v2.0.0" {
		t.Fatalf("version = %q, want v2.0.0", info.ServiceVersion)
	}
	// version.lock released.
	if _, err := os.Stat(filepath.Join(work, "version.lock")); !os.IsNotExist(err) {
		t.Fatal("version.lock not released")
	}
}

func TestUpdateModuleNoopWhenCurrent(t *testing.T) {
	fake := &fakeReleases{latest: "v2.0.0", tarball: stubServerTarGz(t, "noop")}
	mgr := NewManager(t.TempDir(), fake)
	mgr.RegisterModule(&Info{ServiceName: "ppanel-server", Repository: "https://github.com/perfect-panel/server", ServiceVersion: "v2.0.0", Kind: KindService})
	if err := mgr.UpdateModule(context.Background(), "ppanel-server"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if fake.downloads != 0 {
		t.Fatalf("downloaded %d times though already current", fake.downloads)
	}
}

func TestUpdateModuleBakSwapOnUpgrade(t *testing.T) {
	work := t.TempDir()
	fake := &fakeReleases{latest: "v2.0.0", tarball: stubServerTarGz(t, "v2")}
	mgr := NewManager(work, fake)
	mgr.RegisterModule(&Info{ServiceName: "ppanel-server", Repository: "https://github.com/perfect-panel/server", ServiceVersion: "v1.0.0", Kind: KindService})
	t.Cleanup(func() { _ = mgr.supervisor("ppanel-server").Stop() })

	if err := mgr.UpdateModule(context.Background(), "ppanel-server"); err != nil {
		t.Fatalf("first update: %v", err)
	}
	// Upgrade again: a .bak of the previous binary must be left behind.
	fake.latest = "v3.0.0"
	fake.tarball = stubServerTarGz(t, "v3")
	if err := mgr.UpdateModule(context.Background(), "ppanel-server"); err != nil {
		t.Fatalf("second update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "modules", "ppanel-server.bak")); err != nil {
		t.Fatalf(".bak backup missing after upgrade: %v", err)
	}
	if info, _ := mgr.FindModule("ppanel-server"); info.ServiceVersion != "v3.0.0" {
		t.Fatalf("version = %q, want v3.0.0", info.ServiceVersion)
	}
}

func TestHeartbeatMarksUnhealthyAfterFailures(t *testing.T) {
	mgr := NewManager(t.TempDir(), &fakeReleases{})
	// Unreachable heartbeat URL -> probe always fails.
	mgr.RegisterModule(&Info{ServiceName: "svc", HeartbeatURL: "http://127.0.0.1:1/down"})
	for i := 0; i <= maxHeartbeatFailures; i++ {
		mgr.pollOnce(context.Background())
	}
	if healthy, _ := mgr.IsHealthy("svc"); healthy {
		t.Fatalf("module should be unhealthy after >%d failures", maxHeartbeatFailures)
	}
}
