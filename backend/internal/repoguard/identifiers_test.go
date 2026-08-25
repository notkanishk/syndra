// Package repoguard holds checks about the REPOSITORY rather than about any
// package in it.
//
// It exists for one reason: `github.com/notkanishk/syndra` is public, and a
// deployment identifier committed to a public repository is published
// permanently, whatever a later commit removes. Scrubbing the working tree does
// not un-publish history, so the only version of this that works is the one
// that runs before the commit.
package repoguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Nothing in this file names the deployment it is protecting. A guard written
// as "must not contain <the real hostname>" publishes the real hostname, which
// is the thing it was written to prevent. Every rule below is a SHAPE.

var (
	// A private IPv4 HOST address. Four octets, so package versions like
	// `10.4.1` in a lockfile are not addresses and never match — that
	// false positive is why this is spelled out rather than approximated.
	privateHost = regexp.MustCompile(
		`\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}` +
			`|192\.168\.\d{1,3}\.\d{1,3}` +
			`|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})\b`)

	// CIDR notation for a whole private range is configuration, not a host:
	// `10.0.0.0/8` in a proxy denylist identifies nobody.
	privateCIDR = regexp.MustCompile(`/\d{1,2}\b`)

	// A hostname in a place a hostname actually appears: after a scheme, after
	// an `@`, or on the right of a host-shaped setting.
	//
	// Context rather than shape, because shape alone cannot work here. A dotted
	// token ending in a plausible TLD matches `user.team`, `url.host`,
	// `data.services`, `navigator.onLine` and several hundred more — this
	// codebase is full of them, and a guard that reports those gets switched
	// off, which is the usual way a check like this dies.
	hostInURL     = regexp.MustCompile(`(?:https?|wss?|ftps?|ssh|postgres(?:ql)?|redis|amqp|mongodb)://(?:[^\s/@"'` + "`" + `]*@)?([a-z0-9][a-z0-9.-]*\.[a-z]{2,})`)
	hostAfterAt   = regexp.MustCompile(`@([a-z0-9][a-z0-9.-]*\.[a-z]{2,})\b`)
	hostInSetting = regexp.MustCompile(`(?i)\b(?:host|hostname|domain|server|endpoint|fqdn|share_host|base_url|url)\b\s*[=:]\s*["'` + "`" + `]?([a-z0-9][a-z0-9.-]*\.[a-z]{2,})`)

	// A disk or chassis serial: a long unbroken run of upper-case and digits
	// that is not a Go/TS identifier or an HTTP constant.
	serialish = regexp.MustCompile(`\b[A-Z]{2}[0-9A-Z]{6,}\b`)
)

// reservedTLD is allowed wherever it appears: RFC 2606 / RFC 6761 set these
// aside so documentation and private networks have names that can never be
// somebody's real host.
var reservedTLD = map[string]bool{
	"local": true, "internal": true, "test": true, "invalid": true,
	"localhost": true, "example": true, "localdomain": true,
	"lan": true, "home": true, "corp": true, "intranet": true,
}

// knownTLD is what makes a captured token a HOSTNAME rather than a field
// access. `host: connection.host` and `os.environ` both survive the context
// anchor; neither ends in a top-level domain.
var knownTLD = map[string]bool{
	"com": true, "org": true, "net": true, "edu": true, "gov": true, "mil": true,
	"int": true, "io": true, "dev": true, "app": true, "cloud": true, "tools": true,
	"tech": true, "sh": true, "ai": true, "co": true, "me": true, "info": true,
	"biz": true, "xyz": true, "site": true, "online": true, "systems": true,
	"services": true, "network": true, "host": false, "space": true, "zone": true,
	"link": true, "live": true, "life": true, "world": true, "works": true,
	"team": false, "group": false, "company": true, "agency": true, "page": true,
	"uk": true, "de": true, "fr": true, "nl": true, "eu": true, "in": true,
	"us": true, "ca": true, "au": true, "jp": true, "cn": true, "br": true,
	"ru": true, "ch": true, "se": true, "no": true, "fi": true, "dk": true,
	"it": true, "es": true, "pl": true, "ie": true, "nz": true, "za": true,
}

// permittedDomains is an ALLOWLIST, because a denylist of real hostnames would
// have to contain them. Reserved and example names, plus the third parties this
// project genuinely documents.
var permittedDomains = map[string]bool{
	// RFC 2606 / RFC 6761 reserved for documentation and testing.
	"example.com": true, "example.org": true, "example.net": true,
	"example.edu": true, "invalid": true, "test": true, "localhost": true,
	// Reserved TLDs for private use.
	"local": true, "internal": true, "localdomain": true, "arpa": true,
	// Third parties this repository legitimately references.
	"github.com": true, "githubusercontent.com": true, "github.io": true,
	"zitadel.com": true, "zitadel.cloud": true, "truenas.com": true,
	"ixsystems.com": true, "golang.org": true, "go.dev": true, "docker.io": true,
	"docker.com": true, "npmjs.com": true, "bun.sh": true, "nextjs.org": true,
	"nodejs.org": true, "letsencrypt.org": true, "ietf.org": true, "rfc-editor.org": true,
	"apache.org": true, "mozilla.org": true, "w3.org": true, "postgresql.org": true,
	"redis.io": true, "gnu.org": true, "cloudflare.com": true, "google.com": true,
	"googleapis.com": true, "gstatic.com": true, "anthropic.com": true, "claude.com": true,
	"schema.org": true, "json-schema.org": true, "opensource.org": true,
	"unicode.org": true, "iana.org": true, "samba.org": true, "kernel.org": true,
	"debian.org": true, "alpinelinux.org": true, "proxmox.com": true,
	"shields.io": true, "zitadel.dev": true, "img.shields.io": true,
}

// Paths whose content is not ours to police.
var skipPath = []string{
	"design_handoff", "ui/bun.lock", "bun.lockb", "package-lock.json",
	"go.sum", "ui/public/", "docs/assets/", ".png", ".jpg", ".jpeg",
	".woff", ".woff2", ".ico", ".pdf",
	// This file names the shapes; matching itself proves nothing.
	"internal/repoguard/",
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("not in a git work tree: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func trackedFiles(t *testing.T) (string, []string) {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var keep []string
	for _, p := range strings.Fields(string(out)) {
		skip := false
		for _, s := range skipPath {
			if strings.Contains(p, s) {
				skip = true
				break
			}
		}
		if !skip {
			keep = append(keep, p)
		}
	}
	return root, keep
}

// readText reads the WORKING TREE, never HEAD.
//
// The first version of this read HEAD and fell back to the tree, which checks
// what has already been published — the one state a guard can do nothing
// about. What has to be clean is what is about to be committed.
func readText(root, path string) (string, bool) {
	body, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return "", false
	}
	s := string(body)
	if strings.ContainsRune(s, 0) {
		return "", false // binary
	}
	return s, true
}

// No private host address may be committed.
//
// This repository is public. An RFC 1918 address in it maps somebody's internal
// network for anybody who reads it, and no test, fixture or document needs a
// real one: RFC 5737 reserves 192.0.2.0/24, 198.51.100.0/24 and 203.0.113.0/24
// for exactly this.
func TestNoPrivateHostAddressIsCommitted(t *testing.T) {
	root, files := trackedFiles(t)
	var found []string
	for _, path := range files {
		body, ok := readText(root, path)
		if !ok {
			continue
		}
		for i, line := range strings.Split(body, "\n") {
			for _, hit := range privateHost.FindAllString(line, -1) {
				// `10.0.0.0/8` is a range in a config, not a host.
				rest := line[strings.Index(line, hit)+len(hit):]
				if privateCIDR.MatchString(rest[:min(3, len(rest))]) {
					continue
				}
				found = append(found, path+":"+itoa(i+1)+" "+hit)
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("private network addresses in a PUBLIC repository:\n  %s\n\n"+
			"Use the documentation ranges instead — 192.0.2.0/24, 198.51.100.0/24, "+
			"203.0.113.0/24 (RFC 5737). A whole-range CIDR like 10.0.0.0/8 is "+
			"configuration and is allowed; a host address is somebody's network.",
			strings.Join(found, "\n  "))
	}
}

// No hostname outside the allowlist may be committed.
//
// An allowlist rather than a denylist, because a denylist would have to spell
// out the real hostnames in order to ban them — publishing them in the guard
// that exists to keep them unpublished.
func TestNoRealHostnameIsCommitted(t *testing.T) {
	root, files := trackedFiles(t)
	seen := map[string][]string{}
	for _, path := range files {
		body, ok := readText(root, path)
		if !ok {
			continue
		}
		for i, line := range strings.Split(body, "\n") {
			lower := strings.ToLower(line)
			for _, re := range []*regexp.Regexp{hostInURL, hostAfterAt, hostInSetting} {
				for _, m := range re.FindAllStringSubmatch(lower, -1) {
					host := strings.TrimRight(m[1], ".")
					tld := host[strings.LastIndex(host, ".")+1:]
					// Not a hostname at all: a field access that happened to
					// sit to the right of something called `host` or `url`.
					if !knownTLD[tld] && !reservedTLD[tld] {
						continue
					}
					if reservedTLD[tld] {
						continue // never anybody's real host
					}
					if permittedDomains[host] || permittedDomains[registrable(host)] {
						continue
					}
					// A filename or a version, not a host.
					if looksLikeFilenameOrVersion(host) {
						continue
					}
					key := registrable(host)
					if len(seen[key]) < 3 {
						seen[key] = append(seen[key], path+":"+itoa(i+1))
					}
				}
			}
		}
	}
	if len(seen) > 0 {
		var lines []string
		for host, where := range seen {
			lines = append(lines, host+"  ("+strings.Join(where, ", ")+")")
		}
		t.Errorf("hostnames that are not documentation names, in a PUBLIC repository:\n  %s\n\n"+
			"Use example.org / example.com / *.local, or add the domain to "+
			"permittedDomains if it is a third party this project legitimately "+
			"documents.", strings.Join(lines, "\n  "))
	}
}

// No hardware serial may be committed. A disk serial identifies a specific
// machine in a specific rack, and every use of one here has been illustrative.
func TestNoHardwareSerialIsCommitted(t *testing.T) {
	root, files := trackedFiles(t)
	// Upper-case runs that are ordinary vocabulary rather than serials.
	// Upper-case constants that are vocabulary, not serials.
	notASerial := map[string]bool{
		// The static LM-hash sentinel every SMB implementation carries; it
		// identifies no machine and is the same value everywhere.
		"AAD3B435B51404EE": true,
		// This repository's own placeholder, used where a real serial was.
		"SERIAL0000": true,
		// A password-complexity fixture. Upper-case by design, because that is
		// the rule it exercises.
		"ALLUPPERCASE1": true,
	}
	known := regexp.MustCompile(`^(?:NT_STATUS|EACCES|EINVAL|ENOENT|RFC\d+|` +
		`SMB\d?|NFS|ISCSI|TRUENAS|SYNDRA|ZITADEL|POSTGRES|REDIS|HTTP\w*|JSON\w*|` +
		`[A-Z]+_[A-Z_0-9]+)$`)
	var found []string
	for _, path := range files {
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".ts") &&
			!strings.HasSuffix(path, ".tsx") && !strings.HasSuffix(path, ".md") &&
			!strings.HasSuffix(path, ".json") {
			continue
		}
		body, ok := readText(root, path)
		if !ok {
			continue
		}
		for i, line := range strings.Split(body, "\n") {
			for _, hit := range serialish.FindAllString(line, -1) {
				if notASerial[hit] || known.MatchString(hit) || !hasDigitAndLetter(hit) {
					continue
				}
				found = append(found, path+":"+itoa(i+1)+" "+hit)
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("values shaped like hardware serials in a PUBLIC repository:\n  %s\n\n"+
			"Use a placeholder. A serial names one machine in one rack.",
			strings.Join(found, "\n  "))
	}
}

func registrable(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func looksLikeFilenameOrVersion(s string) bool {
	// `v1.2.3`, `1.25.0`, `foo.go`, `page.tsx`.
	if regexp.MustCompile(`^v?\d`).MatchString(s) {
		return true
	}
	ext := s[strings.LastIndex(s, ".")+1:]
	switch ext {
	case "go", "ts", "tsx", "js", "jsx", "mjs", "cjs", "json", "md", "yml", "yaml",
		"sh", "sql", "css", "html", "txt", "lock", "mod", "sum", "env", "example",
		"png", "svg", "ico", "toml", "conf", "log", "py", "tf", "gz", "crt", "key", "pem":
		return true
	}
	return false
}

func hasDigitAndLetter(s string) bool {
	var d, l bool
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			d = true
		case r >= 'A' && r <= 'Z':
			l = true
		}
	}
	return d && l
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
