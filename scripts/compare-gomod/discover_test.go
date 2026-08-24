package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseDiscoverTag(t *testing.T) {
	t.Parallel()
	fullSHA := strings.Repeat("a1", 20)
	tests := []struct {
		name       string
		tag        string
		wantOK     bool
		wantStream string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantAlpha  int
		wantSHA    string
	}{
		{
			name: "alpha", tag: "v2.14.5-alpha12", wantOK: true,
			wantStream: discoverStreamAlpha, wantMajor: 2, wantMinor: 14, wantPatch: 5, wantAlpha: 12,
		},
		{
			name: "full uppercase prime SHA", tag: "v2.14.5-" + strings.ToUpper(fullSHA) + "-head", wantOK: true,
			wantStream: discoverStreamPrimeHead, wantMajor: 2, wantMinor: 14, wantPatch: 5, wantSHA: strings.ToUpper(fullSHA),
		},
		{
			name: "seven character prime SHA", tag: "v2.14.5-deadbee-head", wantOK: true,
			wantStream: discoverStreamPrimeHead, wantMajor: 2, wantMinor: 14, wantPatch: 5, wantSHA: "deadbee",
		},
		{
			name: "seven character uppercase prime SHA", tag: "v2.14.5-DEADBEE-head", wantOK: true,
			wantStream: discoverStreamPrimeHead, wantMajor: 2, wantMinor: 14, wantPatch: 5, wantSHA: "DEADBEE",
		},
		{name: "literal patch placeholder", tag: "v2.14.z-" + fullSHA + "-head"},
		{name: "6 character SHA", tag: "v2.14.5-deadbe-head"},
		{name: "8 character SHA", tag: "v2.14.5-deadbeef-head"},
		{name: "39 character SHA", tag: "v2.14.5-" + strings.Repeat("a", 39) + "-head"},
		{name: "41 character SHA", tag: "v2.14.5-" + strings.Repeat("a", 41) + "-head"},
		{name: "non-hex short SHA", tag: "v2.14.5-deadbeg-head"},
		{name: "non-hex SHA", tag: "v2.14.5-" + strings.Repeat("g", 40) + "-head"},
		{name: "old minor SHA head", tag: "v2.14-" + fullSHA + "-head"},
		{name: "rolling head", tag: "v2.14-head"},
		{name: "malformed alpha", tag: "v2.14.5-alpha"},
		{name: "suffix after alpha", tag: "v2.14.5-alpha2-amd64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseDiscoverTag(tt.tag)
			if ok != tt.wantOK {
				t.Fatalf("parseDiscoverTag(%q) ok = %v, want %v", tt.tag, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.stream != tt.wantStream || got.major != tt.wantMajor || got.minor != tt.wantMinor ||
				got.patch != tt.wantPatch || got.alpha != tt.wantAlpha || got.sha != tt.wantSHA {
				t.Fatalf("parseDiscoverTag(%q) = %#v", tt.tag, got)
			}
		})
	}
}

func TestParseDiscoverImageConfig(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("b", 40)
	tag, ok := parseDiscoverTag("v2.14.5-" + strings.ToUpper(sha) + "-head")
	if !ok {
		t.Fatal("test tag did not parse")
	}
	created := time.Date(2026, 8, 19, 12, 34, 56, 789000000, time.FixedZone("fixture", -4*60*60))
	payload := discoverConfigFixture(created, "private-revision", sha,
		"CATTLE_RANCHER_WEBHOOK_VERSION=109.0.0+up0.10.0-rc.1",
		"RANCHER_VERSION_TYPE=PrImE",
	)

	got, err := parseDiscoverImageConfig(payload, tag, "registry.example/rancher/rancher", 3)
	if err != nil {
		t.Fatalf("parseDiscoverImageConfig() error = %v", err)
	}
	if got.ImageRevision != "private-revision" {
		t.Errorf("ImageRevision = %q", got.ImageRevision)
	}
	if got.SourceRevision != sha {
		t.Errorf("SourceRevision = %q, want OSS revision %q", got.SourceRevision, sha)
	}
	if got.WebhookBuild != "109.0.0+up0.10.0-rc.1" {
		t.Errorf("WebhookBuild = %q", got.WebhookBuild)
	}
	if got.Flavor != discoverFlavorPrime {
		t.Errorf("Flavor = %q", got.Flavor)
	}
	if got.Created.Location() != time.UTC || !got.Created.Equal(created) {
		t.Errorf("Created = %s, want UTC form of %s", got.Created, created)
	}

	fields := strings.Split(got.TSV(), "\t")
	if len(fields) != 9 {
		t.Fatalf("TSV has %d fields, want 9: %q", len(fields), got.TSV())
	}
	wantFields := []string{
		tag.name,
		created.UTC().Format(time.RFC3339),
		"image",
		"registry.example/rancher/rancher",
		"private-revision",
		sha,
		"109.0.0+up0.10.0-rc.1",
		"prime-head",
		"prime",
	}
	if strings.Join(fields, "|") != strings.Join(wantFields, "|") {
		t.Fatalf("TSV fields = %#v, want %#v", fields, wantFields)
	}
}

func TestParseDiscoverImageConfigRejectsPrimeRevisionMismatch(t *testing.T) {
	t.Parallel()
	fullTagSHA := strings.Repeat("c", 40)
	shortTagSHA := "c0ffee0"
	shortRevision := shortTagSHA + strings.Repeat("d", 33)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	for _, tt := range []struct {
		name          string
		tagSHA        string
		imageRevision string
		ossRevision   string
		env           []string
		wantError     string
	}{
		{name: "missing OSS revision", tagSHA: fullTagSHA, imageRevision: fullTagSHA, env: []string{"RANCHER_VERSION_TYPE=prime"}, wantError: "has no org.opencontainers.image.oss.revision"},
		{name: "full OSS revision mismatch", tagSHA: fullTagSHA, imageRevision: fullTagSHA, ossRevision: strings.Repeat("e", 40), env: []string{"RANCHER_VERSION_TYPE=prime"}, wantError: "does not match OSS source revision"},
		{name: "short OSS revision mismatch", tagSHA: shortTagSHA, ossRevision: "badcafe" + strings.Repeat("d", 33), env: []string{"RANCHER_VERSION_TYPE=prime"}, wantError: "does not match OSS source revision"},
		{name: "short label is not a full revision", tagSHA: shortTagSHA, ossRevision: shortTagSHA, env: []string{"RANCHER_VERSION_TYPE=prime"}, wantError: "does not match OSS source revision"},
		{name: "39 character label is not a full revision", tagSHA: shortTagSHA, ossRevision: shortRevision[:39], env: []string{"RANCHER_VERSION_TYPE=prime"}, wantError: "does not match OSS source revision"},
		{name: "non-hex full label", tagSHA: shortTagSHA, ossRevision: shortTagSHA + strings.Repeat("g", 33), env: []string{"RANCHER_VERSION_TYPE=prime"}, wantError: "does not match OSS source revision"},
		{name: "community metadata", tagSHA: fullTagSHA, imageRevision: "private", ossRevision: fullTagSHA, wantError: "not classified as Prime"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tag, ok := parseDiscoverTag("v2.15.1-" + tt.tagSHA + "-head")
			if !ok {
				t.Fatal("test tag did not parse")
			}
			_, err := parseDiscoverImageConfig(
				discoverConfigFixture(now, tt.imageRevision, tt.ossRevision, tt.env...),
				tag, "example.test/rancher", 0,
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestParseDiscoverImageConfigAcceptsShortPrimeRevisionPrefix(t *testing.T) {
	t.Parallel()
	const tagSHA = "29C07AA"
	fullRevision := strings.ToLower(tagSHA) + strings.Repeat("b", 33)
	tag, ok := parseDiscoverTag("v2.15.1-" + tagSHA + "-head")
	if !ok {
		t.Fatal("test tag did not parse")
	}
	payload := discoverConfigFixture(
		time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
		"private-revision",
		fullRevision,
		"RANCHER_VERSION_TYPE=prime",
	)

	got, err := parseDiscoverImageConfig(payload, tag, "example.test/rancher", 0)
	if err != nil {
		t.Fatalf("parseDiscoverImageConfig() error = %v", err)
	}
	if got.SourceRevision != fullRevision {
		t.Errorf("SourceRevision = %q, want full OSS revision %q", got.SourceRevision, fullRevision)
	}
	if got.Tag != "v2.15.1-"+tagSHA+"-head" {
		t.Errorf("Tag = %q", got.Tag)
	}
}

func TestParseDiscoverImageConfigPrimeFlavorFromSourceLabel(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("7", 40)
	tag, ok := parseDiscoverTag("v2.14.5-" + sha + "-head")
	if !ok {
		t.Fatal("test tag did not parse")
	}
	payload := discoverConfigFixtureWithSource(
		time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
		"private-revision",
		sha,
		"https://github.com/rancher-prime/rancher.git",
	)
	got, err := parseDiscoverImageConfig(payload, tag, "example.test/rancher", 0)
	if err != nil {
		t.Fatalf("parseDiscoverImageConfig() error = %v", err)
	}
	if got.Flavor != discoverFlavorPrime || got.SourceRevision != sha {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestDiscoverCandidatesSelectionAndSourcePrecedence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	sha14OldPatch := strings.Repeat("a", 40)
	sha14Selected := strings.Repeat("b", 40)
	sha14Mismatch := strings.Repeat("c", 40)
	sha13Fallback := strings.Repeat("d", 40)

	primary := "primary.example/rancher/rancher"
	mirror := "mirror.example/rancher/rancher"
	broken := "broken.example/rancher/rancher"
	runner := newDiscoverFixtureRunner()
	runner.listings[primary] = discoverFixtureResponse{data: discoverTagsFixture(
		"latest", // ignored
		"v2.14.5-alpha2",
		"v2.14.5-alpha10",
		"v2.14.5-alpha1",
		"v2.13.8-alpha1",
		"v2.14.9-"+sha14OldPatch+"-head",
		"v2.14.10-"+sha14Selected+"-head",
		"v2.14.11-"+sha14Mismatch+"-head", // invalid higher patch must not hide patch 10
		"v2.14.z-"+sha14Selected+"-head",  // ignored
		"v2.13.6-"+sha13Fallback+"-head",
	)}
	runner.listings[mirror] = discoverFixtureResponse{data: discoverTagsFixture(
		"v2.14.5-alpha10", // duplicate; primary wins
		"v2.13.8-alpha2",
		"v2.14.10-"+sha14Selected+"-head", // duplicate; primary wins
		"v2.13.6-"+sha13Fallback+"-head",  // primary fails, so this is used
	)}
	runner.listings[broken] = discoverFixtureResponse{err: errors.New("registry unavailable")}

	runner.configs[discoverFixtureKey(primary, "v2.14.5-alpha10")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-2*time.Hour), "image-alpha-revision", "oss-alpha-revision",
		"CATTLE_RANCHER_WEBHOOK_VERSION=109.0.0+up0.10.0-rc.2",
	)}
	runner.configs[discoverFixtureKey(mirror, "v2.14.5-alpha10")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-time.Hour), "mirror-must-not-win", "", "RANCHER_VERSION_TYPE=prime",
	)}
	runner.configs[discoverFixtureKey(primary, "v2.14.5-alpha2")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-4*time.Hour), "alpha2", "",
	)}
	// alpha1 is deliberately newer but must not be inspected because the alpha
	// tags-per-line limit is two after numeric tag ordering.
	runner.configs[discoverFixtureKey(primary, "v2.14.5-alpha1")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-30*time.Minute), "alpha1", "",
	)}
	runner.configs[discoverFixtureKey(primary, "v2.13.8-alpha1")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-40*24*time.Hour), "old-alpha", "",
	)}
	runner.configs[discoverFixtureKey(mirror, "v2.13.8-alpha2")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-48*time.Hour), "alpha13", "", "RANCHER_VERSION_TYPE=community",
	)}

	// Patch 9 is inspected for provenance but cannot win once patch 10 is
	// validated as the highest real Prime patch for v2.14.
	runner.configs[discoverFixtureKey(primary, "v2.14.9-"+sha14OldPatch+"-head")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-10*time.Minute), "", sha14OldPatch, "RANCHER_VERSION_TYPE=prime",
	)}
	runner.configs[discoverFixtureKey(primary, "v2.14.10-"+sha14Selected+"-head")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-3*time.Hour), "private-prime-revision", sha14Selected,
		"CATTLE_RANCHER_WEBHOOK_VERSION=109.0.0+up0.10.0-rc.3",
		"RANCHER_VERSION_TYPE=prime",
	)}
	runner.configs[discoverFixtureKey(mirror, "v2.14.10-"+sha14Selected+"-head")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-time.Hour), "", sha14Selected, "RANCHER_VERSION_TYPE=prime",
	)}
	runner.configs[discoverFixtureKey(primary, "v2.14.11-"+sha14Mismatch+"-head")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-30*time.Minute), "", strings.Repeat("f", 40), "RANCHER_VERSION_TYPE=prime",
	)}
	runner.configs[discoverFixtureKey(primary, "v2.13.6-"+sha13Fallback+"-head")] = discoverFixtureResponse{err: errors.New("manifest missing")}
	runner.configs[discoverFixtureKey(mirror, "v2.13.6-"+sha13Fallback+"-head")] = discoverFixtureResponse{data: discoverConfigFixture(
		now.Add(-6*time.Hour), "private-13-revision", sha13Fallback, "RANCHER_VERSION_TYPE=PRIME",
	)}

	var warnings bytes.Buffer
	got, err := discoverCandidates(ctx, runner, discoverOptions{
		Images:      []string{primary, mirror, broken},
		MaxAge:      30 * 24 * time.Hour,
		TagsPerLine: 2,
		Now:         now,
		Workers:     3,
	}, &warnings)
	if err != nil {
		t.Fatalf("discoverCandidates() error = %v", err)
	}

	wantTags := []string{
		"v2.14.5-alpha10",
		"v2.14.10-" + sha14Selected + "-head",
		"v2.13.8-alpha2",
		"v2.13.6-" + sha13Fallback + "-head",
	}
	if len(got) != len(wantTags) {
		t.Fatalf("discoverCandidates() returned %d candidates, want %d\n%#v\nwarnings:\n%s", len(got), len(wantTags), got, warnings.String())
	}
	for i, want := range wantTags {
		if got[i].Tag != want {
			t.Errorf("candidate[%d].Tag = %q, want %q", i, got[i].Tag, want)
		}
	}
	if got[0].ImageSource != primary || got[0].SourceRevision != "oss-alpha-revision" || got[0].Flavor != "community" {
		t.Errorf("alpha precedence/metadata = %#v", got[0])
	}
	if got[1].ImageSource != primary || got[1].ImageRevision != "private-prime-revision" || got[1].SourceRevision != sha14Selected || got[1].Flavor != "prime" {
		t.Errorf("Prime metadata = %#v", got[1])
	}
	if got[3].ImageSource != mirror {
		t.Errorf("fallback ImageSource = %q, want %q", got[3].ImageSource, mirror)
	}

	inspectCalls := runner.inspectCallSnapshot()
	for _, forbidden := range []string{
		discoverFixtureKey(primary, "v2.14.5-alpha1"),
		discoverFixtureKey(mirror, "v2.14.5-alpha10"),
		discoverFixtureKey(mirror, "v2.14.10-"+sha14Selected+"-head"),
	} {
		if discoverContains(inspectCalls, forbidden) {
			t.Errorf("unexpected inspection of %s; calls = %#v", forbidden, inspectCalls)
		}
	}
	if !discoverContains(inspectCalls, discoverFixtureKey(mirror, "v2.13.6-"+sha13Fallback+"-head")) {
		t.Errorf("missing fallback inspection; calls = %#v", inspectCalls)
	}
	if !discoverContains(inspectCalls, discoverFixtureKey(primary, "v2.14.9-"+sha14OldPatch+"-head")) {
		t.Errorf("valid lower Prime patch was not inspected; calls = %#v", inspectCalls)
	}
	for _, wantWarning := range []string{
		"could not list tags for " + broken,
		"tag SHA " + sha14Mismatch + " does not match OSS source revision",
		"could not inspect " + primary + ":v2.13.6-" + sha13Fallback + "-head",
	} {
		if !strings.Contains(warnings.String(), wantWarning) {
			t.Errorf("warnings do not contain %q:\n%s", wantWarning, warnings.String())
		}
	}
	if runner.maxConcurrentInspections() > 3 {
		t.Errorf("maximum concurrent inspections = %d, want <= 3", runner.maxConcurrentInspections())
	}
}

func TestDiscoverCandidatesAgeBoundaryAndTimestampTie(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	image := "example.test/rancher"
	runner := newDiscoverFixtureRunner()
	runner.listings[image] = discoverFixtureResponse{data: discoverTagsFixture(
		"v2.12.9-alpha1",
		"v2.12.9-alpha2",
		"v2.11.8-alpha1",
	)}
	runner.configs[discoverFixtureKey(image, "v2.12.9-alpha1")] = discoverFixtureResponse{data: discoverConfigFixture(now.Add(-30*24*time.Hour), "one", "")}
	runner.configs[discoverFixtureKey(image, "v2.12.9-alpha2")] = discoverFixtureResponse{data: discoverConfigFixture(now.Add(-30*24*time.Hour), "two", "")}
	runner.configs[discoverFixtureKey(image, "v2.11.8-alpha1")] = discoverFixtureResponse{data: discoverConfigFixture(now.Add(-30*24*time.Hour-time.Nanosecond), "old", "")}

	got, err := discoverCandidates(context.Background(), runner, discoverOptions{
		Images: []string{image}, MaxAge: 30 * 24 * time.Hour, TagsPerLine: 3, Now: now,
		Streams: map[string]bool{discoverStreamAlpha: true},
	}, nil)
	if err != nil {
		t.Fatalf("discoverCandidates() error = %v", err)
	}
	if len(got) != 1 || got[0].Tag != "v2.12.9-alpha2" {
		t.Fatalf("candidates = %#v, want alpha2 at the inclusive age boundary", got)
	}
}

func TestRunDiscoverCLI(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Add(-time.Hour)
	sha := strings.Repeat("9", 40)
	first := "first.example/rancher"
	second := "second.example/rancher"
	runner := newDiscoverFixtureRunner()
	runner.listings[first] = discoverFixtureResponse{err: errors.New("denied")}
	runner.listings[second] = discoverFixtureResponse{data: discoverTagsFixture(
		"v2.16.0-alpha3",
		"v2.16.0-"+sha+"-head",
	)}
	runner.configs[discoverFixtureKey(second, "v2.16.0-alpha3")] = discoverFixtureResponse{data: discoverConfigFixture(
		now, "alpha-revision", "", "CATTLE_RANCHER_WEBHOOK_VERSION=110.0.0+up0.12.0",
	)}

	var stdout, stderr bytes.Buffer
	code := runDiscoverCLI(context.Background(), []string{
		"-max-age-days", "7",
		"-tags-per-line", "4",
		"-streams", "alpha",
		"-image", first,
		"-image", second,
	}, &stdout, &stderr, runner)
	if code != 0 {
		t.Fatalf("runDiscoverCLI() code = %d, stderr = %s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout lines = %#v", lines)
	}
	fields := strings.Split(lines[0], "\t")
	if len(fields) != 9 {
		t.Fatalf("output fields = %#v, want 9 fields", fields)
	}
	if fields[0] != "v2.16.0-alpha3" || fields[2] != "image" || fields[3] != second ||
		fields[4] != "alpha-revision" || fields[5] != "alpha-revision" || fields[7] != "alpha" || fields[8] != "community" {
		t.Errorf("output fields = %#v", fields)
	}
	if !strings.Contains(stderr.String(), "discover: warning: could not list tags for "+first) {
		t.Errorf("stderr = %q", stderr.String())
	}
	if discoverContains(runner.inspectCallSnapshot(), discoverFixtureKey(second, "v2.16.0-"+sha+"-head")) {
		t.Error("disabled prime-head stream was inspected")
	}
}

func TestRunDiscoverCLIRuntimeFailures(t *testing.T) {
	t.Parallel()

	t.Run("all registries fail", func(t *testing.T) {
		t.Parallel()
		runner := newDiscoverFixtureRunner()
		runner.listings["broken.example/rancher"] = discoverFixtureResponse{err: errors.New("denied")}
		var stdout, stderr bytes.Buffer
		code := runDiscoverCLI(context.Background(), []string{
			"-streams", "alpha", "-image", "broken.example/rancher",
		}, &stdout, &stderr, runner)
		if code != 1 || !strings.Contains(stderr.String(), "could not list tags from any configured image source") {
			t.Fatalf("code = %d, stderr = %q", code, stderr.String())
		}
	})

	t.Run("matched stream cannot be inspected", func(t *testing.T) {
		t.Parallel()
		const image = "broken.example/rancher"
		runner := newDiscoverFixtureRunner()
		runner.listings[image] = discoverFixtureResponse{data: discoverTagsFixture("v2.16.0-alpha1")}
		runner.configs[discoverFixtureKey(image, "v2.16.0-alpha1")] = discoverFixtureResponse{err: errors.New("manifest unavailable")}
		var stdout, stderr bytes.Buffer
		code := runDiscoverCLI(context.Background(), []string{
			"-streams", "alpha", "-image", image,
		}, &stdout, &stderr, runner)
		if code != 1 || !strings.Contains(stderr.String(), "could not inspect a valid candidate for stream(s): alpha") {
			t.Fatalf("code = %d, stderr = %q", code, stderr.String())
		}
	})

	t.Run("legitimate no match", func(t *testing.T) {
		t.Parallel()
		const image = "empty.example/rancher"
		runner := newDiscoverFixtureRunner()
		runner.listings[image] = discoverFixtureResponse{data: discoverTagsFixture("v2.16-head", "latest")}
		var stdout, stderr bytes.Buffer
		code := runDiscoverCLI(context.Background(), []string{
			"-streams", "prime-head", "-image", image,
		}, &stdout, &stderr, runner)
		if code != 0 || stdout.Len() != 0 {
			t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("only stale matches", func(t *testing.T) {
		t.Parallel()
		const image = "stale.example/rancher"
		runner := newDiscoverFixtureRunner()
		runner.listings[image] = discoverFixtureResponse{data: discoverTagsFixture("v2.16.0-alpha1")}
		runner.configs[discoverFixtureKey(image, "v2.16.0-alpha1")] = discoverFixtureResponse{data: discoverConfigFixture(
			time.Now().UTC().Add(-60*24*time.Hour), "old-revision", "",
		)}
		var stdout, stderr bytes.Buffer
		code := runDiscoverCLI(context.Background(), []string{
			"-max-age-days", "30", "-streams", "alpha", "-image", image,
		}, &stdout, &stderr, runner)
		if code != 0 || stdout.Len() != 0 {
			t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
		}
	})
}

func TestRunDiscoverCLIUsageErrorsOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "help", args: []string{"-h"}, code: 0},
		{name: "zero age", args: []string{"-max-age-days", "0"}, code: 2},
		{name: "zero alpha limit", args: []string{"-tags-per-line", "0"}, code: 2},
		{name: "unknown stream", args: []string{"-streams", "alpha,head"}, code: 2},
		{name: "empty stream", args: []string{"-streams", "alpha,"}, code: 2},
		{name: "positional argument", args: []string{"surprise"}, code: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := runDiscoverCLI(context.Background(), tt.args, &stdout, &stderr, newDiscoverFixtureRunner())
			if code != tt.code {
				t.Fatalf("code = %d, want %d; stderr = %s", code, tt.code, stderr.String())
			}
			if tt.name == "help" {
				for _, text := range []string{"source_revision", "webhook_build", "prime-head"} {
					if !strings.Contains(stderr.String(), text) {
						t.Errorf("help does not document %q:\n%s", text, stderr.String())
					}
				}
			}
		})
	}
}

type discoverFixtureResponse struct {
	data []byte
	err  error
}

type discoverFixtureRunner struct {
	listings map[string]discoverFixtureResponse
	configs  map[string]discoverFixtureResponse
	delay    time.Duration

	mu             sync.Mutex
	listCalls      []string
	inspectCalls   []string
	activeInspect  int
	maximumInspect int
}

func newDiscoverFixtureRunner() *discoverFixtureRunner {
	return &discoverFixtureRunner{
		listings: make(map[string]discoverFixtureResponse),
		configs:  make(map[string]discoverFixtureResponse),
	}
}

func (r *discoverFixtureRunner) ListTags(_ context.Context, image string) ([]byte, error) {
	r.mu.Lock()
	r.listCalls = append(r.listCalls, image)
	response, found := r.listings[image]
	r.mu.Unlock()
	if !found {
		return nil, fmt.Errorf("no listing fixture for %s", image)
	}
	return append([]byte(nil), response.data...), response.err
}

func (r *discoverFixtureRunner) InspectConfig(ctx context.Context, image, tag string) ([]byte, error) {
	key := discoverFixtureKey(image, tag)
	r.mu.Lock()
	r.inspectCalls = append(r.inspectCalls, key)
	r.activeInspect++
	if r.activeInspect > r.maximumInspect {
		r.maximumInspect = r.activeInspect
	}
	response, found := r.configs[key]
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.activeInspect--
		r.mu.Unlock()
	}()
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if !found {
		return nil, fmt.Errorf("no config fixture for %s", key)
	}
	return append([]byte(nil), response.data...), response.err
}

func (r *discoverFixtureRunner) inspectCallSnapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.inspectCalls...)
}

func (r *discoverFixtureRunner) maxConcurrentInspections() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maximumInspect
}

func discoverFixtureKey(image, tag string) string {
	return image + "@" + tag
}

func discoverTagsFixture(tags ...string) []byte {
	payload, err := json.Marshal(map[string]any{"Repository": "rancher/rancher", "Tags": tags})
	if err != nil {
		panic(err)
	}
	return payload
}

func discoverConfigFixture(created time.Time, revision, ossRevision string, env ...string) []byte {
	return discoverConfigFixtureWithSource(created, revision, ossRevision, "", env...)
}

func discoverConfigFixtureWithSource(created time.Time, revision, ossRevision, source string, env ...string) []byte {
	labels := map[string]string{}
	if revision != "" {
		labels["org.opencontainers.image.revision"] = revision
	}
	if ossRevision != "" {
		labels["org.opencontainers.image.oss.revision"] = ossRevision
	}
	if source != "" {
		labels["org.opencontainers.image.source"] = source
	}
	payload, err := json.Marshal(map[string]any{
		"created": created.Format(time.RFC3339Nano),
		"config": map[string]any{
			"Labels": labels,
			"Env":    env,
		},
	})
	if err != nil {
		panic(err)
	}
	return payload
}

func discoverContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
