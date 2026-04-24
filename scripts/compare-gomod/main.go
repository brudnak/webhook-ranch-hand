// compare-gomod generates and indexes github.com/rancher/* drift reports
// between rancher/rancher and rancher/webhook go.mod files.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

const rancherPrefix = "github.com/rancher/"

var ignoredPaths = map[string]bool{
	"github.com/rancher/rancher/pkg/apis":   true,
	"github.com/rancher/rancher/pkg/client": true,
	"github.com/rancher/rke":                true,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "compare":
		cmdCompare(os.Args[2:])
	case "index":
		cmdIndex(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  %s compare -version VERSION [-webhook WEBHOOK] [-webhook-build BUILD]
              [-rancher-published RFC3339] [-webhook-published RFC3339]
              [-reports-dir DIR] <rancher.mod> <webhook.mod>
  %s index   [-reports-dir DIR] [-readme FILE]
`, os.Args[0], os.Args[0])
}

type dep struct {
	path        string
	required    string
	indirect    bool
	replaceVer  string // non-empty: replaced to this version
	replacePath string // non-empty: replaced to this filesystem path
}

func (d *dep) effective() string {
	if d == nil {
		return ""
	}
	if d.replacePath != "" {
		return d.replacePath
	}
	if d.replaceVer != "" {
		return d.replaceVer
	}
	return d.required
}

func parseGoMod(path string) (map[string]*dep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, err
	}
	deps := map[string]*dep{}
	get := func(p string) *dep {
		if _, ok := deps[p]; !ok {
			deps[p] = &dep{path: p}
		}
		return deps[p]
	}
	for _, r := range f.Require {
		if !strings.HasPrefix(r.Mod.Path, rancherPrefix) {
			continue
		}
		d := get(r.Mod.Path)
		d.required = r.Mod.Version
		d.indirect = r.Indirect
	}
	for _, r := range f.Replace {
		if !strings.HasPrefix(r.Old.Path, rancherPrefix) {
			continue
		}
		d := get(r.Old.Path)
		if strings.HasPrefix(r.New.Path, ".") || strings.HasPrefix(r.New.Path, "/") {
			d.replacePath = r.New.Path
		} else {
			d.replaceVer = r.New.Version
		}
	}
	return deps, nil
}

type status int

const (
	statusMatch status = iota
	statusMismatch
	statusIgnored
	statusOnlyRancher
	statusOnlyWebhook
)

type row struct {
	path    string
	rancher *dep
	webhook *dep
	status  status
	notes   []string
}

func classify(rancherDeps, webhookDeps map[string]*dep) []row {
	seen := map[string]bool{}
	for p := range rancherDeps {
		seen[p] = true
	}
	for p := range webhookDeps {
		seen[p] = true
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	rows := make([]row, 0, len(paths))
	for _, p := range paths {
		r := rancherDeps[p]
		w := webhookDeps[p]
		if ignoredPaths[p] {
			rows = append(rows, row{path: p, rancher: r, webhook: w, status: statusIgnored})
			continue
		}
		if r == nil {
			rows = append(rows, row{path: p, rancher: r, webhook: w, status: statusOnlyWebhook})
			continue
		}
		if w == nil {
			rows = append(rows, row{path: p, rancher: r, webhook: w, status: statusOnlyRancher})
			continue
		}
		var notes []string
		if r.replaceVer != "" && r.replaceVer != r.required {
			notes = append(notes, "rancher replace pin "+r.replaceVer)
		}
		if w.replaceVer != "" && w.replaceVer != w.required {
			notes = append(notes, "webhook replace pin "+w.replaceVer)
		}
		if w.indirect {
			notes = append(notes, "indirect in webhook")
		}
		if r.indirect {
			notes = append(notes, "indirect in rancher")
		}
		s := statusMatch
		if r.effective() != w.effective() {
			s = statusMismatch
		}
		rows = append(rows, row{path: p, rancher: r, webhook: w, status: s, notes: notes})
	}
	return rows
}

func shortName(p string) string { return strings.TrimPrefix(p, rancherPrefix) }

var lineRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.`)

func extractLine(version string) string {
	m := lineRE.FindStringSubmatch(version)
	if len(m) != 3 {
		return ""
	}
	return fmt.Sprintf("v%s.%s", m[1], m[2])
}

func cmdCompare(args []string) {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	version := fs.String("version", "", "Rancher version tag (required, e.g. v2.14.0-alpha12)")
	webhook := fs.String("webhook", "", "Webhook version tag (e.g. v0.10.0-rc.11)")
	webhookBuild := fs.String("webhook-build", "", "Webhook build string (e.g. 109.0.0+up0.10.0-rc.11)")
	rancherPublished := fs.String("rancher-published", "", "Rancher tag release date (RFC3339)")
	webhookPublished := fs.String("webhook-published", "", "Webhook tag release date (RFC3339)")
	reportsDir := fs.String("reports-dir", "reports", "Root directory for reports")
	_ = fs.Parse(args)

	if *version == "" || fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "compare: missing -version or go.mod arguments")
		os.Exit(2)
	}

	line := extractLine(*version)
	if line == "" {
		fmt.Fprintf(os.Stderr, "compare: could not parse release line from %q\n", *version)
		os.Exit(2)
	}

	rDeps, err := parseGoMod(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare: %v\n", err)
		os.Exit(1)
	}
	wDeps, err := parseGoMod(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare: %v\n", err)
		os.Exit(1)
	}
	rows := classify(rDeps, wDeps)

	outDir := filepath.Join(*reportsDir, line)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "compare: %v\n", err)
		os.Exit(1)
	}
	outPath := filepath.Join(outDir, *version+".md")

	body := renderReport(reportInput{
		version:          *version,
		webhook:          *webhook,
		webhookBuild:     *webhookBuild,
		rancherPublished: parseMaybeTime(*rancherPublished),
		webhookPublished: parseMaybeTime(*webhookPublished),
		rows:             rows,
		now:              time.Now().UTC(),
	})
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "compare: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(outPath)
}

func parseMaybeTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

type reportInput struct {
	version          string
	webhook          string
	webhookBuild     string
	rancherPublished time.Time
	webhookPublished time.Time
	rows             []row
	now              time.Time
}

func renderReport(in reportInput) string {
	var mismatches, matches, ignored, onlyR, onlyW []row
	for _, r := range in.rows {
		switch r.status {
		case statusMismatch:
			mismatches = append(mismatches, r)
		case statusMatch:
			matches = append(matches, r)
		case statusIgnored:
			ignored = append(ignored, r)
		case statusOnlyRancher:
			onlyR = append(onlyR, r)
		case statusOnlyWebhook:
			onlyW = append(onlyW, r)
		}
	}

	var b strings.Builder

	b.WriteString("<!-- meta\n")
	fmt.Fprintf(&b, "version: %s\n", in.version)
	if in.webhook != "" {
		fmt.Fprintf(&b, "webhook: %s\n", in.webhook)
	}
	if in.webhookBuild != "" {
		fmt.Fprintf(&b, "webhook_build: %s\n", in.webhookBuild)
	}
	if !in.rancherPublished.IsZero() {
		fmt.Fprintf(&b, "rancher_published: %s\n", in.rancherPublished.Format(time.RFC3339))
	}
	if !in.webhookPublished.IsZero() {
		fmt.Fprintf(&b, "webhook_published: %s\n", in.webhookPublished.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "generated: %s\n", in.now.Format(time.RFC3339))
	fmt.Fprintf(&b, "mismatches: %d\n", len(mismatches))
	fmt.Fprintf(&b, "matches: %d\n", len(matches))
	fmt.Fprintf(&b, "ignored: %d\n", len(ignored))
	fmt.Fprintf(&b, "only_rancher: %d\n", len(onlyR))
	fmt.Fprintf(&b, "only_webhook: %d\n", len(onlyW))
	b.WriteString("-->\n\n")

	line := extractLine(in.version)
	fmt.Fprintf(&b, "[← Back to %s](README.md) · [Back to dashboard](../../README.md)\n\n", line)
	fmt.Fprintf(&b, "# %s\n\n", in.version)

	b.WriteString("| Side | Tag | Released | Source |\n| --- | --- | --- | --- |\n")
	rancherSrc := fmt.Sprintf("[go.mod](https://github.com/rancher/rancher/blob/%s/go.mod) · [build.yaml](https://github.com/rancher/rancher/blob/%s/build.yaml)", in.version, in.version)
	fmt.Fprintf(&b, "| Rancher | `%s` | %s | %s |\n", in.version, fmtDateCell(in.rancherPublished), rancherSrc)
	if in.webhook != "" {
		webhookTagCell := "`" + in.webhook + "`"
		if in.webhookBuild != "" {
			webhookTagCell += " · from `" + in.webhookBuild + "`"
		}
		webhookSrc := fmt.Sprintf("[go.mod](https://github.com/rancher/webhook/blob/%s/go.mod)", in.webhook)
		fmt.Fprintf(&b, "| Webhook | %s | %s | %s |\n", webhookTagCell, fmtDateCell(in.webhookPublished), webhookSrc)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "**Checked:** %s\n\n", in.now.Format("2006-01-02 15:04 UTC"))

	if len(mismatches) == 0 {
		b.WriteString("## ✅ Clean\n\n")
		b.WriteString("No mismatches. Rancher and webhook agree on every shared module.\n\n")
	} else {
		fmt.Fprintf(&b, "## ⚠️ %d %s\n\n", len(mismatches), plural("mismatch", "mismatches", len(mismatches)))
		b.WriteString("| Module | Rancher | Webhook | Notes |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
		for _, r := range mismatches {
			rv := effectiveOrDash(r.rancher)
			wv := effectiveOrDash(r.webhook)
			notes := strings.Join(r.notes, "; ")
			if notes == "" {
				notes = "—"
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", shortName(r.path), rv, wv, notes)
		}
		b.WriteString("\n")
	}

	if len(matches) > 0 {
		fmt.Fprintf(&b, "<details><summary>Matches (%d)</summary>\n\n", len(matches))
		b.WriteString("| Module | Version |\n| --- | --- |\n")
		for _, r := range matches {
			v := effectiveOrDash(r.rancher)
			if v == "-" {
				v = effectiveOrDash(r.webhook)
			}
			fmt.Fprintf(&b, "| `%s` | `%s` |\n", shortName(r.path), v)
		}
		b.WriteString("\n</details>\n\n")
	}

	if len(ignored) > 0 {
		fmt.Fprintf(&b, "<details><summary>Ignored (%d)</summary>\n\n", len(ignored))
		b.WriteString("| Module | Rancher | Webhook |\n| --- | --- | --- |\n")
		for _, r := range ignored {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` |\n", shortName(r.path), effectiveOrDash(r.rancher), effectiveOrDash(r.webhook))
		}
		b.WriteString("\n_Expected drift. `pkg/apis` and `pkg/client` are in-tree replaces in rancher (their \"versions\" are stale pseudo-versions); `rke` is a weak dependency where patch-level mismatches are accepted policy._\n\n")
		b.WriteString("</details>\n\n")
	}

	if len(onlyR)+len(onlyW) > 0 {
		fmt.Fprintf(&b, "<details><summary>Present on only one side (%d)</summary>\n\n", len(onlyR)+len(onlyW))
		b.WriteString("| Module | Side | Version |\n| --- | --- | --- |\n")
		for _, r := range onlyR {
			fmt.Fprintf(&b, "| `%s` | rancher only | `%s` |\n", shortName(r.path), effectiveOrDash(r.rancher))
		}
		for _, r := range onlyW {
			label := "webhook only"
			if r.webhook != nil && r.webhook.indirect {
				label = "webhook only (indirect)"
			}
			fmt.Fprintf(&b, "| `%s` | %s | `%s` |\n", shortName(r.path), label, effectiveOrDash(r.webhook))
		}
		b.WriteString("\n</details>\n\n")
	}

	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "[← Back to %s](README.md) · [Back to dashboard](../../README.md)\n", line)
	return b.String()
}

func effectiveOrDash(d *dep) string {
	v := d.effective()
	if v == "" {
		return "-"
	}
	return v
}

func fmtDateCell(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04 UTC")
}

func fmtDateShort(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}

func plural(singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

type reportMeta struct {
	version          string
	line             string
	webhook          string
	webhookBuild     string
	rancherPublished time.Time
	webhookPublished time.Time
	generated        time.Time
	mismatches       int
	matches          int
	ignored          int
	onlyRancher      int
	onlyWebhook      int
	relPath          string
}

func (m reportMeta) status() string {
	if m.mismatches == 0 {
		return "✅ Clean"
	}
	return fmt.Sprintf("⚠️ %d %s", m.mismatches, plural("mismatch", "mismatches", m.mismatches))
}

func cmdIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	reportsDir := fs.String("reports-dir", "reports", "Root directory for reports")
	readme := fs.String("readme", "README.md", "Top-level README to update")
	_ = fs.Parse(args)

	reports, err := scanReports(*reportsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "index: %v\n", err)
		os.Exit(1)
	}

	byLine := map[string][]reportMeta{}
	for _, r := range reports {
		byLine[r.line] = append(byLine[r.line], r)
	}
	for line := range byLine {
		rs := byLine[line]
		sort.SliceStable(rs, func(i, j int) bool {
			if !rs[i].generated.Equal(rs[j].generated) {
				return rs[i].generated.After(rs[j].generated)
			}
			return versionLess(rs[j].version, rs[i].version)
		})
		byLine[line] = rs
	}

	for line, rs := range byLine {
		body := renderLineIndex(line, rs)
		path := filepath.Join(*reportsDir, line, "README.md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "index: writing %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	if err := updateDashboard(*readme, byLine); err != nil {
		fmt.Fprintf(os.Stderr, "index: %v\n", err)
		os.Exit(1)
	}
}

func scanReports(reportsDir string) ([]reportMeta, error) {
	var out []reportMeta
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "v") {
			continue
		}
		lineDir := filepath.Join(reportsDir, e.Name())
		err := filepath.WalkDir(lineDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || d.Name() == "README.md" {
				return nil
			}
			meta, err := parseReportMeta(path, reportsDir)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
			out = append(out, meta)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func parseReportMeta(path, reportsDir string) (reportMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return reportMeta{}, err
	}
	defer f.Close()

	m := reportMeta{}
	rel, err := filepath.Rel(reportsDir, path)
	if err == nil {
		m.relPath = filepath.ToSlash(rel)
	}

	scanner := bufio.NewScanner(f)
	inMeta := false
	for scanner.Scan() {
		line := scanner.Text()
		if !inMeta {
			if strings.TrimSpace(line) == "<!-- meta" {
				inMeta = true
			}
			continue
		}
		if strings.TrimSpace(line) == "-->" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "version":
			m.version = val
			m.line = extractLine(val)
		case "webhook":
			m.webhook = val
		case "webhook_build":
			m.webhookBuild = val
		case "generated":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				m.generated = t
			}
		case "rancher_published":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				m.rancherPublished = t
			}
		case "webhook_published":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				m.webhookPublished = t
			}
		case "mismatches":
			m.mismatches, _ = strconv.Atoi(val)
		case "matches":
			m.matches, _ = strconv.Atoi(val)
		case "ignored":
			m.ignored, _ = strconv.Atoi(val)
		case "only_rancher":
			m.onlyRancher, _ = strconv.Atoi(val)
		case "only_webhook":
			m.onlyWebhook, _ = strconv.Atoi(val)
		}
	}
	if err := scanner.Err(); err != nil {
		return m, err
	}
	if m.version == "" {
		return m, fmt.Errorf("no <!-- meta --> block found")
	}
	return m, nil
}

func renderLineIndex(line string, reports []reportMeta) string {
	var b strings.Builder
	b.WriteString("[← Back to dashboard](../../README.md)\n\n")
	fmt.Fprintf(&b, "# %s reports\n\n", line)
	fmt.Fprintf(&b, "%d report(s) for release line %s.\n\n", len(reports), line)
	b.WriteString("| Alpha | Released | Status | Webhook | Webhook released | Checked | Report |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range reports {
		reportFile := filepath.Base(r.relPath)
		webhookCell := "-"
		if r.webhook != "" {
			webhookCell = "`" + r.webhook + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | [open](%s) |\n",
			r.version,
			fmtDateShort(r.rancherPublished),
			r.status(),
			webhookCell,
			fmtDateShort(r.webhookPublished),
			fmtDateShort(r.generated),
			reportFile)
	}
	b.WriteString("\n")
	return b.String()
}

func updateDashboard(readmePath string, byLine map[string][]reportMeta) error {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}
	content := string(data)
	const startMarker = "<!-- AUTO:DASHBOARD:START -->"
	const endMarker = "<!-- AUTO:DASHBOARD:END -->"
	i := strings.Index(content, startMarker)
	j := strings.Index(content, endMarker)
	if i == -1 || j == -1 || j < i {
		return fmt.Errorf("markers %s / %s not found in %s", startMarker, endMarker, readmePath)
	}
	dashboard := renderDashboard(byLine)
	newContent := content[:i+len(startMarker)] + "\n\n" + dashboard + "\n" + content[j:]
	return os.WriteFile(readmePath, []byte(newContent), 0o644)
}

func renderDashboard(byLine map[string][]reportMeta) string {
	lines := make([]string, 0, len(byLine))
	for l := range byLine {
		lines = append(lines, l)
	}
	sort.Slice(lines, func(i, j int) bool { return versionLess(lines[j], lines[i]) })

	var all []reportMeta
	for _, rs := range byLine {
		all = append(all, rs...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].generated.Equal(all[j].generated) {
			return all[i].generated.After(all[j].generated)
		}
		return versionLess(all[j].version, all[i].version)
	})

	var b strings.Builder
	b.WriteString("## Latest per release line\n\n")
	if len(lines) == 0 {
		b.WriteString("_No reports yet._\n")
	} else {
		b.WriteString("| Line | Latest alpha | Released | Status | Webhook | Webhook released | Checked | Report |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		for _, l := range lines {
			rs := byLine[l]
			if len(rs) == 0 {
				continue
			}
			latest := rs[0]
			webhookCell := "-"
			if latest.webhook != "" {
				webhookCell = "`" + latest.webhook + "`"
			}
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s | %s | %s | [open](reports/%s) |\n",
				l,
				latest.version,
				fmtDateShort(latest.rancherPublished),
				latest.status(),
				webhookCell,
				fmtDateShort(latest.webhookPublished),
				fmtDateShort(latest.generated),
				latest.relPath)
		}
		b.WriteString("\n")
	}

	if len(all) > 0 {
		b.WriteString("## Recent runs\n\n")
		n := 10
		if len(all) < n {
			n = len(all)
		}
		for _, r := range all[:n] {
			when := fmtDateShort(r.rancherPublished)
			if when == "-" {
				when = fmtDateShort(r.generated)
			}
			fmt.Fprintf(&b, "- %s · [`%s`](reports/%s) · %s\n", when, r.version, r.relPath, r.status())
		}
		b.WriteString("\n")
	}

	return b.String()
}

func versionLess(a, b string) bool {
	ap := parseVersion(a)
	bp := parseVersion(b)
	for i := 0; i < 4; i++ {
		if ap.nums[i] != bp.nums[i] {
			return ap.nums[i] < bp.nums[i]
		}
	}
	// An empty suffix is a released version and sorts after its prereleases.
	if ap.suffix == "" && bp.suffix != "" {
		return false
	}
	if ap.suffix != "" && bp.suffix == "" {
		return true
	}
	return ap.suffix < bp.suffix
}

type parsedVersion struct {
	nums   [4]int // major, minor, patch, prereleaseNum
	suffix string
}

var versionRE = regexp.MustCompile(`^v(\d+)\.(\d+)(?:\.(\d+))?(?:-([a-zA-Z]+)(\d+))?`)

func parseVersion(s string) parsedVersion {
	m := versionRE.FindStringSubmatch(s)
	var p parsedVersion
	if len(m) == 0 {
		return p
	}
	p.nums[0], _ = strconv.Atoi(m[1])
	p.nums[1], _ = strconv.Atoi(m[2])
	if m[3] != "" {
		p.nums[2], _ = strconv.Atoi(m[3])
	}
	if m[5] != "" {
		p.nums[3], _ = strconv.Atoi(m[5])
		p.suffix = "-" + m[4] + m[5]
	}
	return p
}
