package main

import (
	"strings"
	"testing"
	"time"
)

func TestExtractLine(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"v2.14.5-alpha1": "v2.14",
		"v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head": "v2.14",
		"v2.14-4d29026545cefa8ce9d8950d746c4fda80217942-head":   "v2.14",
		"v2.14-head":    "v2.14",
		"head":          "",
		"not-a-version": "",
	}
	for version, want := range tests {
		version, want := version, want
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			if got := extractLine(version); got != want {
				t.Fatalf("extractLine(%q) = %q, want %q", version, got, want)
			}
		})
	}
}

func TestNormalizedBuildStream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		want    string
	}{
		{"v2.14.5-alpha1", "alpha"},
		{"v2.15.1-29c07aa-head", "prime-head"},
		{"v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head", "prime-head"},
		{"v2.15.1-29c07a-head", "other"},
		{"v2.15.1-29c07aa8-head", "other"},
		{"v2.15.1-" + strings.Repeat("a", 39) + "-head", "other"},
		{"v2.14-4d29026545cefa8ce9d8950d746c4fda80217942-head", "community-head"},
		{"v2.14.5", "other"},
	}
	for _, tt := range tests {
		if got := normalizedBuildStream("", tt.version); got != tt.want {
			t.Errorf("normalizedBuildStream(%q) = %q, want %q", tt.version, got, tt.want)
		}
	}
}

func TestRenderReportUsesOSSRefAndKeepsImageRevision(t *testing.T) {
	t.Parallel()

	const (
		version       = "v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head"
		ossRevision   = "4d29026545cefa8ce9d8950d746c4fda80217942"
		imageRevision = "05cf5e948c7a6195200aadd9fe8a5beb619a6a84"
	)
	body := renderReport(reportInput{
		version:          version,
		rancherDate:      time.Date(2026, 8, 19, 13, 56, 44, 0, time.UTC),
		rancherDateSrc:   "image",
		rancherImage:     "stgregistry.suse.com/rancher/rancher:" + version,
		rancherRevision:  ossRevision,
		imageRevision:    imageRevision,
		rancherSourceRef: ossRevision,
		buildStream:      "prime-head",
		buildFlavor:      "prime",
		now:              time.Date(2026, 8, 19, 16, 0, 0, 0, time.UTC),
	})

	wants := []string{
		"rancher_revision: " + ossRevision,
		"rancher_image_revision: " + imageRevision,
		"rancher_source_ref: " + ossRevision,
		"build_stream: prime-head",
		"build_flavor: prime",
		"https://github.com/rancher/rancher/blob/" + ossRevision + "/go.mod",
		"OSS revision `4d29026545ce`",
		"image revision `05cf5e948c7a`",
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("rendered report missing %q", want)
		}
	}
	if strings.Contains(body, "rancher/rancher/blob/"+version) {
		t.Fatal("rendered report linked the image tag as a Git ref")
	}
}

func TestDashboardSplitsLatestBuildsByStream(t *testing.T) {
	t.Parallel()

	alpha := reportMeta{
		version: "v2.14.5-alpha1", line: "v2.14", buildStream: "alpha",
		rancherDate: time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC), relPath: "v2.14/v2.14.5-alpha1.md",
	}
	const fullHead = "v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head"
	head := reportMeta{
		version: fullHead, line: "v2.14", buildStream: "prime-head",
		rancherDate: time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC), relPath: "v2.14/" + fullHead + ".md",
	}
	olderHead := reportMeta{
		version: "v2.14.5-1111111-head", line: "v2.14", buildStream: "prime-head",
		rancherDate: time.Date(2026, 8, 18, 14, 0, 0, 0, time.UTC), relPath: "v2.14/v2.14.5-1111111-head.md",
	}
	head16 := reportMeta{
		version: "v2.16.0-29c07aa-head", line: "v2.16", buildStream: "prime-head",
		rancherDate: time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC), relPath: "v2.16/v2.16.0-29c07aa-head.md",
	}
	body := renderDashboard(map[string][]reportMeta{
		"v2.14": {olderHead, alpha, head}, // Deliberately unsorted to exercise renderer sorting.
		"v2.16": {head16},
	})
	if !strings.Contains(body, "## Latest builds") {
		t.Fatalf("dashboard is missing latest-builds heading:\n%s", body)
	}
	_, afterPrime, ok := strings.Cut(body, "### Prime heads\n")
	if !ok {
		t.Fatalf("dashboard is missing Prime heads section:\n%s", body)
	}
	primeSection, afterPrime, ok := strings.Cut(afterPrime, "### Alphas\n")
	if !ok {
		t.Fatalf("dashboard is missing Alphas section:\n%s", body)
	}
	alphaSection, _, _ := strings.Cut(afterPrime, "## Recent runs")

	if strings.Contains(primeSection, "alpha1") || !strings.Contains(primeSection, fullHead) || !strings.Contains(primeSection, head16.relPath) {
		t.Fatalf("Prime heads table is not isolated to Prime builds:\n%s", primeSection)
	}
	if strings.Contains(primeSection, olderHead.relPath) {
		t.Fatalf("Prime heads table includes an older build from the same release line:\n%s", primeSection)
	}
	if strings.Contains(alphaSection, "-head") || !strings.Contains(alphaSection, alpha.relPath) {
		t.Fatalf("Alphas table is not isolated to alpha builds:\n%s", alphaSection)
	}
	if strings.Index(primeSection, "| v2.16 |") > strings.Index(primeSection, "| v2.14 |") {
		t.Fatalf("Prime release lines are not newest-first:\n%s", primeSection)
	}
	wantShortLink := "[`v2.14.5-4d29026-head`](reports/v2.14/" + fullHead + ".md)"
	if got := strings.Count(body, wantShortLink); got != 2 {
		t.Errorf("dashboard contains shortened Prime head link %d times, want 2 (latest and recent):\n%s", got, body)
	}
	wantAlphaLink := "[`v2.14.5-alpha1`](reports/v2.14/v2.14.5-alpha1.md)"
	if got := strings.Count(body, wantAlphaLink); got != 2 {
		t.Errorf("dashboard contains alpha link %d times, want 2 (latest and recent):\n%s", got, body)
	}
	if strings.Contains(body, "[`"+fullHead+"`]") || strings.Contains(body, "`"+fullHead+"` |") {
		t.Fatalf("dashboard displays the full Prime head tag:\n%s", body)
	}
	if !strings.Contains(body, "seven-character SHA") {
		t.Fatalf("dashboard is missing the SHA abbreviation note:\n%s", body)
	}
}

func TestDashboardUsesCompactColumns(t *testing.T) {
	t.Parallel()

	report := reportMeta{
		version:          "v2.14.5-alpha1",
		line:             "v2.14",
		buildStream:      "alpha",
		rancherDate:      time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC),
		webhookPublished: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		generated:        time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		relPath:          "v2.14/v2.14.5-alpha1.md",
	}
	head := reportMeta{
		version: "v2.14.5-29c07aa-head", line: "v2.14", buildStream: "prime-head",
		generated: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), relPath: "v2.14/v2.14.5-29c07aa-head.md",
	}
	body := renderDashboard(map[string][]reportMeta{"v2.14": {report, head}})
	latestSection, _, _ := strings.Cut(body, "## Recent runs")

	wantHeader := "| Line | Latest build | Status | Webhook | Checked | Report |"
	if got := strings.Count(latestSection, wantHeader); got != 2 {
		t.Fatalf("dashboard contains compact header %d times, want 2:\n%s", got, latestSection)
	}
	if strings.Contains(latestSection, "| Line | Stream |") {
		t.Fatalf("stream column is redundant in split latest tables:\n%s", latestSection)
	}
	for _, unwanted := range []string{"Rancher date", "Webhook date", "Source", "2026-08-17", "2026-08-18"} {
		if strings.Contains(latestSection, unwanted) {
			t.Errorf("latest dashboard section contains removed value %q:\n%s", unwanted, latestSection)
		}
	}
	if !strings.Contains(latestSection, "2026-08-19") {
		t.Errorf("latest dashboard section is missing the checked date:\n%s", latestSection)
	}
}

func TestDashboardLatestTableEmptyStates(t *testing.T) {
	t.Parallel()

	alpha := reportMeta{
		version: "v2.14.5-alpha1", line: "v2.14", buildStream: "alpha",
		generated: time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC), relPath: "v2.14/v2.14.5-alpha1.md",
	}
	head := reportMeta{
		version: "v2.14.5-29c07aa-head", line: "v2.14", buildStream: "prime-head",
		generated: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), relPath: "v2.14/v2.14.5-29c07aa-head.md",
	}
	tests := []struct {
		name           string
		byLine         map[string][]reportMeta
		wantPrimeEmpty bool
		wantAlphaEmpty bool
		wantRecent     bool
	}{
		{name: "no reports", byLine: nil, wantPrimeEmpty: true, wantAlphaEmpty: true},
		{name: "alpha only", byLine: map[string][]reportMeta{"v2.14": {alpha}}, wantPrimeEmpty: true, wantRecent: true},
		{name: "Prime only", byLine: map[string][]reportMeta{"v2.14": {head}}, wantAlphaEmpty: true, wantRecent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := renderDashboard(tt.byLine)
			if got := strings.Contains(body, "_No Prime head reports yet._"); got != tt.wantPrimeEmpty {
				t.Errorf("Prime empty state = %v, want %v:\n%s", got, tt.wantPrimeEmpty, body)
			}
			if got := strings.Contains(body, "_No alpha reports yet._"); got != tt.wantAlphaEmpty {
				t.Errorf("alpha empty state = %v, want %v:\n%s", got, tt.wantAlphaEmpty, body)
			}
			if got := strings.Contains(body, "## Recent runs"); got != tt.wantRecent {
				t.Errorf("Recent runs presence = %v, want %v:\n%s", got, tt.wantRecent, body)
			}
		})
	}
}

func TestDashboardPreservesOtherStreams(t *testing.T) {
	t.Parallel()

	rc := reportMeta{
		version: "v2.14.5-rc1", line: "v2.14", buildStream: "other",
		generated: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), relPath: "v2.14/v2.14.5-rc1.md",
	}
	body := renderDashboard(map[string][]reportMeta{"v2.14": {rc}})
	_, otherSection, ok := strings.Cut(body, "### Other streams\n")
	if !ok || !strings.Contains(otherSection, "| v2.14 | `other` |") || !strings.Contains(otherSection, rc.relPath) {
		t.Fatalf("dashboard dropped latest non-alpha/non-Prime stream:\n%s", body)
	}
}

func TestDashboardBuildLabelOnlyShortensFullPrimeHeadSHA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		stream  string
		want    string
	}{
		{"v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head", "prime-head", "v2.14.5-4d29026-head"},
		{"v2.14.5-alpha1", "alpha", "v2.14.5-alpha1"},
		{"v2.14.5-4d29026-head", "prime-head", "v2.14.5-4d29026-head"},
		{"v2.14.5-4d290265-head", "prime-head", "v2.14.5-4d290265-head"},
		{"v2.14.5-" + strings.Repeat("a", 39) + "-head", "prime-head", "v2.14.5-" + strings.Repeat("a", 39) + "-head"},
		{"v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head", "other", "v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head"},
	}
	for _, tt := range tests {
		if got := dashboardBuildLabel(tt.version, tt.stream); got != tt.want {
			t.Errorf("dashboardBuildLabel(%q, %q) = %q, want %q", tt.version, tt.stream, got, tt.want)
		}
	}
}

func TestVersionLessUsesNumericAlphaSuffix(t *testing.T) {
	t.Parallel()

	if !versionLess("v2.15.0-alpha9", "v2.15.0-alpha10") {
		t.Fatal("alpha9 should sort before alpha10")
	}
	if versionLess("v2.15.0", "v2.15.0-alpha10") {
		t.Fatal("a release should sort after its prerelease")
	}
}
