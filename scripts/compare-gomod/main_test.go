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
		{"v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head", "prime-head"},
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

func TestDashboardKeepsLatestBuildPerStream(t *testing.T) {
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
	body := renderDashboard(map[string][]reportMeta{"v2.14": {head, alpha}})
	if strings.Count(body, "| v2.14 |") != 2 {
		t.Fatalf("dashboard should contain one row per stream:\n%s", body)
	}
	for _, want := range []string{"`alpha`", "`prime-head`", "Latest per release line and stream"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
	wantShortLink := "[`v2.14.5-4d2902-head`](reports/v2.14/" + fullHead + ".md)"
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
	if !strings.Contains(body, "six-character SHA") {
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
	body := renderDashboard(map[string][]reportMeta{"v2.14": {report}})
	latestSection, _, _ := strings.Cut(body, "## Recent runs")

	wantHeader := "| Line | Stream | Latest build | Status | Webhook | Checked | Report |"
	if !strings.Contains(latestSection, wantHeader) {
		t.Fatalf("dashboard header missing %q:\n%s", wantHeader, latestSection)
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

func TestDashboardBuildLabelOnlyShortensFullPrimeHeadSHA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		stream  string
		want    string
	}{
		{"v2.14.5-4d29026545cefa8ce9d8950d746c4fda80217942-head", "prime-head", "v2.14.5-4d2902-head"},
		{"v2.14.5-alpha1", "alpha", "v2.14.5-alpha1"},
		{"v2.14.5-4d29026-head", "prime-head", "v2.14.5-4d29026-head"},
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
