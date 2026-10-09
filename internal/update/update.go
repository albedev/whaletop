// Package update checks GitHub for newer whaletop releases and self-updates
// binaries that were installed without a package manager (install.sh, tarball).
// The atomic binary swap (with rollback) is github.com/minio/selfupdate; release
// lookup and sha256 verification mirror install.sh. See .knowledge/release.md.
package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"
)

const (
	repo        = "albedev/whaletop"
	checkMaxAge = 24 * time.Hour
)

// githubURL is a variable so tests can point it at an httptest server.
var githubURL = "https://github.com"

// Method is how the running binary was installed; it decides how to upgrade it.
type Method int

const (
	MethodManual  Method = iota // install.sh, tarball, make install of a release: self-update allowed
	MethodBrew                  // Homebrew cask
	MethodPackage               // .deb / .rpm in /usr/bin
	MethodGo                    // go install
	MethodDev                   // built from source (non-release version)
)

// Hint is the command the user should run to upgrade.
func (m Method) Hint() string {
	switch m {
	case MethodBrew:
		return "brew upgrade --cask whaletop"
	case MethodPackage:
		return "install the new .deb/.rpm from https://github.com/" + repo + "/releases/latest"
	case MethodGo:
		return "go install github.com/" + repo + "/cmd/whaletop@latest"
	case MethodDev:
		return "git pull && make install"
	}
	return "whaletop update"
}

// Detect infers the install method from the executable path and the version string.
func Detect(exe, version string) Method {
	if !IsRelease(version) {
		return MethodDev
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	switch {
	case strings.Contains(exe, "/Caskroom/") || strings.Contains(exe, "/Cellar/"):
		return MethodBrew
	case strings.HasPrefix(exe, "/usr/bin/") || strings.HasPrefix(exe, "/usr/sbin/") || strings.HasPrefix(exe, "/bin/"):
		return MethodPackage
	}
	for _, dir := range goBinDirs() {
		if filepath.Dir(exe) == dir {
			return MethodGo
		}
	}
	return MethodManual
}

func goBinDirs() []string {
	var dirs []string
	if d := os.Getenv("GOBIN"); d != "" {
		dirs = append(dirs, filepath.Clean(d))
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			gopath = filepath.Join(home, "go")
		}
	}
	for _, p := range filepath.SplitList(gopath) {
		dirs = append(dirs, filepath.Join(p, "bin"))
	}
	return dirs
}

// IsRelease reports whether version is a clean release tag like v1.2.3
// (git-describe builds such as v0.3.0-4-gabc1234 or "dev" are not).
func IsRelease(version string) bool {
	return semver.IsValid(version) && semver.Prerelease(version) == "" && semver.Build(version) == ""
}

// Newer reports whether latest is a newer release than current.
func Newer(latest, current string) bool {
	return IsRelease(latest) && IsRelease(current) && semver.Compare(latest, current) > 0
}

type cache struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "whaletop", "update-check.json"), nil
}

// Latest returns the newest release tag, asking GitHub at most once per day
// (the answer is cached in the user cache dir).
func Latest(ctx context.Context) (string, error) {
	path, cerr := cachePath()
	if cerr == nil {
		var c cache
		if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &c) == nil &&
			time.Since(c.Checked) < checkMaxAge && c.Latest != "" {
			return c.Latest, nil
		}
	}
	latest, err := latestTag(ctx)
	if err != nil {
		return "", err
	}
	if cerr == nil {
		if b, err := json.Marshal(cache{Checked: time.Now(), Latest: latest}); err == nil {
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = os.WriteFile(path, b, 0o644)
		}
	}
	return latest, nil
}

var httpClient = &http.Client{Timeout: 2 * time.Minute}

// latestTag follows github.com/<repo>/releases/latest, which redirects to
// /releases/tag/vX.Y.Z: no API call, so no rate limit (same trick as install.sh).
func latestTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, githubURL+"/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	tag := path.Base(resp.Request.URL.Path)
	if resp.StatusCode != http.StatusOK || !IsRelease(tag) {
		return "", fmt.Errorf("cannot determine latest release (HTTP %d, %q)", resp.StatusCode, tag)
	}
	return tag, nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

// checksumFor finds the sha256 of name in a GoReleaser checksums.txt.
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && f[1] == name {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("%s not listed in checksums.txt", name)
}

// extract returns the content of the regular file called name inside a .tar.gz.
func extract(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("%s not found in archive: %w", name, err)
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == name {
			return io.ReadAll(io.LimitReader(tr, 200<<20))
		}
	}
}

// Apply replaces the running binary with the latest release after verifying
// its sha256 against checksums.txt. It refuses for package-managed or source builds.
func Apply(ctx context.Context, current string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p // update the real file, not the wtop symlink
	}
	switch m := Detect(exe, current); m {
	case MethodManual:
	case MethodDev:
		return "", fmt.Errorf("this is a development build (%s): run `%s`", current, m.Hint())
	default:
		return "", fmt.Errorf("%s is managed by another installer: run `%s`", exe, m.Hint())
	}

	latest, err := latestTag(ctx)
	if err != nil {
		return "", err
	}
	if !Newer(latest, current) {
		return fmt.Sprintf("whaletop %s is already the latest version", current), nil
	}

	// naming must match archives.name_template in .goreleaser.yaml
	name := fmt.Sprintf("whaletop_%s_%s_%s.tar.gz", strings.TrimPrefix(latest, "v"), runtime.GOOS, runtime.GOARCH)
	base := githubURL + "/" + repo + "/releases/download/" + latest + "/"
	sums, err := download(ctx, base+"checksums.txt")
	if err != nil {
		return "", err
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return "", err
	}
	archive, err := download(ctx, base+name)
	if err != nil {
		return "", err
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != want {
		return "", fmt.Errorf("checksum mismatch for %s: refusing to update", name)
	}
	bin, err := extract(archive, "whaletop")
	if err != nil {
		return "", err
	}
	if err := selfupdate.Apply(bytes.NewReader(bin), selfupdate.Options{TargetPath: exe}); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return "", fmt.Errorf("update failed and rollback failed too, reinstall with install.sh: %w", rerr)
		}
		if errors.Is(err, os.ErrPermission) {
			return "", fmt.Errorf("cannot replace %s: permission denied (try with sudo)", exe)
		}
		return "", err
	}
	if p, err := cachePath(); err == nil {
		_ = os.Remove(p)
	}
	return fmt.Sprintf("whaletop updated %s → %s (%s)", current, latest, exe), nil
}
