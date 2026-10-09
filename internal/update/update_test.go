package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsReleaseAndNewer(t *testing.T) {
	for v, want := range map[string]bool{
		"v0.3.0": true, "v1.2.3": true,
		"dev": false, "v0.3.0-4-gabc1234": false, "v0.3.0-4-gabc1234-dirty": false, "0.3.0": false,
		"v0.0.0-SNAPSHOT-a009636": false,
	} {
		if got := IsRelease(v); got != want {
			t.Errorf("IsRelease(%q) = %v", v, got)
		}
	}
	if !Newer("v0.4.0", "v0.3.0") || Newer("v0.3.0", "v0.3.0") || Newer("v0.2.9", "v0.3.0") || Newer("v0.4.0", "dev") {
		t.Fatal("Newer comparisons wrong")
	}
}

func TestDetect(t *testing.T) {
	gopath := t.TempDir()
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOBIN", "")
	cases := map[string]Method{
		"/opt/homebrew/Caskroom/whaletop/0.3.0/whaletop": MethodBrew,
		"/usr/local/Cellar/whaletop/0.3.0/bin/whaletop":  MethodBrew,
		"/usr/bin/whaletop":                              MethodPackage,
		filepath.Join(gopath, "bin", "whaletop"):         MethodGo,
		"/home/me/.local/bin/whaletop":                   MethodManual,
	}
	for exe, want := range cases {
		if got := Detect(exe, "v0.3.0"); got != want {
			t.Errorf("Detect(%q) = %v, want %v", exe, got, want)
		}
	}
	if Detect("/home/me/.local/bin/whaletop", "v0.3.0-2-gdeadbee") != MethodDev {
		t.Error("git-describe build must be MethodDev")
	}
}

func TestDetectFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	cask := filepath.Join(dir, "Caskroom", "whaletop", "0.3.0")
	if err := os.MkdirAll(cask, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(cask, "whaletop")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "wtop")
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	if got := Detect(link, "v0.3.0"); got != MethodBrew {
		t.Fatalf("wtop symlink into the Caskroom detected as %v", got)
	}
}

func TestChecksumFor(t *testing.T) {
	sums := []byte("abc123  whaletop_0.3.0_linux_amd64.tar.gz\ndef456  whaletop_0.3.0_darwin_arm64.tar.gz\n")
	if got, err := checksumFor(sums, "whaletop_0.3.0_darwin_arm64.tar.gz"); err != nil || got != "def456" {
		t.Fatal(got, err)
	}
	if _, err := checksumFor(sums, "whaletop_0.3.0_darwin_amd64.tar.gz"); err == nil {
		t.Fatal("missing entry must fail")
	}
}

func TestExtract(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"README.md": "readme", "whaletop": "BINARY"} {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	got, err := extract(buf.Bytes(), "whaletop")
	if err != nil || string(got) != "BINARY" {
		t.Fatal(string(got), err)
	}
	if _, err := extract(buf.Bytes(), "nope"); err == nil {
		t.Fatal("missing file must fail")
	}
}

// A tampered archive must be rejected before the running binary is touched.
func TestApplyRejectsChecksumMismatch(t *testing.T) {
	name := fmt.Sprintf("whaletop_9.9.9_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	mux.HandleFunc("/albedev/whaletop/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/albedev/whaletop/releases/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/albedev/whaletop/releases/tag/v9.9.9", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/albedev/whaletop/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%064d  %s\n", 0, name)
	})
	mux.HandleFunc("/albedev/whaletop/releases/download/v9.9.9/"+name, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("evil"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	old := githubURL
	githubURL = srv.URL
	defer func() { githubURL = old }()

	exe, _ := os.Executable()
	before, _ := os.ReadFile(exe)
	_, err := Apply(context.Background(), "v0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum mismatch, got %v", err)
	}
	after, _ := os.ReadFile(exe)
	if !bytes.Equal(before, after) {
		t.Fatal("binary was modified")
	}
}
