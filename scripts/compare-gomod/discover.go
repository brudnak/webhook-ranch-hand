package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	discoverStreamAlpha     = "alpha"
	discoverStreamPrimeHead = "prime-head"

	discoverFlavorCommunity = "community"
	discoverFlavorPrime     = "prime"

	discoverDefaultMaxAgeDays  = 30
	discoverDefaultTagsPerLine = 10
	discoverDefaultWorkers     = 8
)

var discoverDefaultImages = []string{
	"stgregistry.suse.com/rancher/rancher",
	"docker.io/rancher/rancher",
	"registry.rancher.com/rancher/rancher",
	"registry.suse.com/rancher/rancher",
}

var (
	discoverAlphaTagRE = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)-alpha([0-9]+)$`)
	// Prime head tags name an exact source commit. Require the complete SHA so
	// the tag can be checked for equality with the effective source revision.
	discoverPrimeHeadTagRE = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)-([0-9a-fA-F]{40})-head$`)
)

// skopeoRunner is deliberately small so discovery can be tested without a
// registry or a skopeo binary.
type skopeoRunner interface {
	ListTags(context.Context, string) ([]byte, error)
	InspectConfig(context.Context, string, string) ([]byte, error)
}

type commandSkopeoRunner struct {
	path string
}

func (r commandSkopeoRunner) ListTags(ctx context.Context, image string) ([]byte, error) {
	return r.run(ctx, "list-tags", discoverDockerReference(image))
}

func (r commandSkopeoRunner) InspectConfig(ctx context.Context, image, tag string) ([]byte, error) {
	return r.run(ctx,
		"inspect",
		"--override-os", "linux",
		"--override-arch", "amd64",
		"--config",
		discoverDockerReference(image+":"+tag),
	)
}

func (r commandSkopeoRunner) run(ctx context.Context, args ...string) ([]byte, error) {
	path := r.path
	if path == "" {
		path = "skopeo"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if detail := strings.TrimSpace(stderr.String()); detail != "" {
		return nil, fmt.Errorf("%w: %s", err, detail)
	}
	return nil, err
}

func discoverDockerReference(reference string) string {
	if strings.Contains(reference, "://") {
		return reference
	}
	return "docker://" + reference
}

type discoverOptions struct {
	Images      []string
	MaxAge      time.Duration
	TagsPerLine int
	Streams     map[string]bool
	Now         time.Time
	Workers     int
}

type discoverCandidate struct {
	Tag            string
	Created        time.Time
	SourceKind     string
	ImageSource    string
	ImageRevision  string
	SourceRevision string
	WebhookBuild   string
	Stream         string
	Flavor         string

	tag         discoverTag
	sourceIndex int
}

// TSV returns the workflow interface. Its stable field order is:
//
//	tag, created (RFC3339), source_kind, image_source, image_revision,
//	source_revision, webhook_build, stream, flavor
func (c discoverCandidate) TSV() string {
	fields := []string{
		c.Tag,
		c.Created.UTC().Format(time.RFC3339),
		c.SourceKind,
		c.ImageSource,
		c.ImageRevision,
		c.SourceRevision,
		c.WebhookBuild,
		c.Stream,
		c.Flavor,
	}
	for i := range fields {
		fields[i] = discoverTSVField(fields[i])
	}
	return strings.Join(fields, "\t")
}

func discoverTSVField(s string) string {
	r := strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
	return r.Replace(s)
}

type discoverTag struct {
	name   string
	stream string
	major  int
	minor  int
	patch  int
	alpha  int
	sha    string
}

func (t discoverTag) lineKey() string {
	return strconv.Itoa(t.major) + "." + strconv.Itoa(t.minor)
}

func parseDiscoverTag(tag string) (discoverTag, bool) {
	if match := discoverAlphaTagRE.FindStringSubmatch(tag); match != nil {
		values, ok := discoverNumericTagParts(match[1], match[2], match[3], match[4])
		if !ok {
			return discoverTag{}, false
		}
		return discoverTag{
			name: tag, stream: discoverStreamAlpha,
			major: values[0], minor: values[1], patch: values[2], alpha: values[3],
		}, true
	}
	if match := discoverPrimeHeadTagRE.FindStringSubmatch(tag); match != nil {
		values, ok := discoverNumericTagParts(match[1], match[2], match[3])
		if !ok {
			return discoverTag{}, false
		}
		return discoverTag{
			name: tag, stream: discoverStreamPrimeHead,
			major: values[0], minor: values[1], patch: values[2], sha: match[4],
		}, true
	}
	return discoverTag{}, false
}

func discoverNumericTagParts(parts ...string) ([]int, bool) {
	values := make([]int, len(parts))
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		values[i] = value
	}
	return values, true
}

type discoverTarget struct {
	tag     discoverTag
	sources []int
}

type discoverInspectResult struct {
	candidate *discoverCandidate
	warnings  []string
	relevant  bool
}

// discoverCandidates isolates registry failures to the affected source or
// image. It returns every successfully selected stream/line candidate, writes
// deterministic warnings to warnOut, and errors only when no source can be
// listed or a matched stream has no inspectable candidate at all.
func discoverCandidates(ctx context.Context, runner skopeoRunner, opts discoverOptions, warnOut io.Writer) ([]discoverCandidate, error) {
	opts = normalizeDiscoverOptions(opts)

	targetByName := make(map[string]*discoverTarget)
	var targetsInFirstSeenOrder []*discoverTarget
	listedSources := 0
	for sourceIndex, image := range opts.Images {
		payload, err := runner.ListTags(ctx, image)
		if err != nil {
			discoverWarnf(warnOut, "could not list tags for %s: %v", image, err)
			continue
		}
		var listing struct {
			Tags []string `json:"Tags"`
		}
		if err := json.Unmarshal(payload, &listing); err != nil {
			discoverWarnf(warnOut, "could not parse tags for %s: %v", image, err)
			continue
		}
		listedSources++
		seenAtSource := make(map[string]bool)
		for _, name := range listing.Tags {
			if seenAtSource[name] {
				continue
			}
			seenAtSource[name] = true
			tag, ok := parseDiscoverTag(name)
			if !ok || !opts.Streams[tag.stream] {
				continue
			}
			target := targetByName[name]
			if target == nil {
				target = &discoverTarget{tag: tag}
				targetByName[name] = target
				targetsInFirstSeenOrder = append(targetsInFirstSeenOrder, target)
			}
			target.sources = append(target.sources, sourceIndex)
		}
	}

	// The first-seen slice makes map iteration irrelevant. Sorting gives stable
	// warning, inspection, and output tie-breaking order.
	targets := append([]*discoverTarget(nil), targetsInFirstSeenOrder...)
	sort.SliceStable(targets, func(i, j int) bool {
		return discoverTagBefore(targets[i].tag, targets[j].tag)
	})

	alphaCount := make(map[string]int)
	selected := make([]discoverTarget, 0, len(targets))
	for _, target := range targets {
		if target.tag.stream == discoverStreamAlpha {
			line := target.tag.lineKey()
			if alphaCount[line] >= opts.TagsPerLine {
				continue
			}
			alphaCount[line]++
		}
		selected = append(selected, *target)
	}

	results := inspectDiscoverTargets(ctx, runner, opts, selected)
	selectedByStream := make(map[string]int)
	validByStream := make(map[string]int)
	valid := make([]discoverCandidate, 0, len(results))
	for i, result := range results {
		if result.relevant {
			selectedByStream[selected[i].tag.stream]++
		}
		for _, warning := range result.warnings {
			discoverWarnf(warnOut, "%s", warning)
		}
		if result.candidate == nil {
			continue
		}
		validByStream[result.candidate.Stream]++
		valid = append(valid, *result.candidate)
	}
	if listedSources == 0 {
		return nil, errors.New("could not list tags from any configured image source")
	}
	var failedStreams []string
	for stream, count := range selectedByStream {
		if count > 0 && validByStream[stream] == 0 {
			failedStreams = append(failedStreams, stream)
		}
	}
	if len(failedStreams) > 0 {
		sort.Strings(failedStreams)
		return nil, fmt.Errorf("could not inspect a valid candidate for stream(s): %s", strings.Join(failedStreams, ", "))
	}

	// A tag only establishes the current Prime patch after its image metadata
	// proves that it is a Prime build with matching OSS provenance. This keeps a
	// malformed or stale higher-patch tag from hiding valid lower-patch builds.
	primePatch := make(map[string]int)
	for _, candidate := range valid {
		if candidate.Stream != discoverStreamPrimeHead {
			continue
		}
		line := candidate.tag.lineKey()
		patch, found := primePatch[line]
		if !found || candidate.tag.patch > patch {
			primePatch[line] = candidate.tag.patch
		}
	}

	latest := make(map[string]discoverCandidate)
	for _, candidate := range valid {
		if candidate.Stream == discoverStreamPrimeHead && candidate.tag.patch != primePatch[candidate.tag.lineKey()] {
			continue
		}
		key := candidate.tag.lineKey() + "/" + candidate.Stream
		current, found := latest[key]
		if !found || discoverCandidateIsNewer(candidate, current) {
			latest[key] = candidate
		}
	}

	candidates := make([]discoverCandidate, 0, len(latest))
	for _, candidate := range latest {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if discoverTagBefore(candidates[i].tag, candidates[j].tag) {
			return true
		}
		if discoverTagBefore(candidates[j].tag, candidates[i].tag) {
			return false
		}
		return candidates[i].Tag < candidates[j].Tag
	})
	return candidates, nil
}

func normalizeDiscoverOptions(opts discoverOptions) discoverOptions {
	if len(opts.Images) == 0 {
		opts.Images = append([]string(nil), discoverDefaultImages...)
	}
	if opts.MaxAge <= 0 {
		opts.MaxAge = discoverDefaultMaxAgeDays * 24 * time.Hour
	}
	if opts.TagsPerLine <= 0 {
		opts.TagsPerLine = discoverDefaultTagsPerLine
	}
	if len(opts.Streams) == 0 {
		opts.Streams = map[string]bool{
			discoverStreamAlpha:     true,
			discoverStreamPrimeHead: true,
		}
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
	if opts.Workers <= 0 {
		opts.Workers = discoverDefaultWorkers
	}
	return opts
}

func discoverTagBefore(a, b discoverTag) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	if a.stream != b.stream {
		return discoverStreamRank(a.stream) < discoverStreamRank(b.stream)
	}
	if a.patch != b.patch {
		return a.patch > b.patch
	}
	if a.stream == discoverStreamAlpha && a.alpha != b.alpha {
		return a.alpha > b.alpha
	}
	return a.name < b.name
}

func discoverStreamRank(stream string) int {
	if stream == discoverStreamAlpha {
		return 0
	}
	return 1
}

func inspectDiscoverTargets(ctx context.Context, runner skopeoRunner, opts discoverOptions, targets []discoverTarget) []discoverInspectResult {
	results := make([]discoverInspectResult, len(targets))
	if len(targets) == 0 {
		return results
	}
	workers := opts.Workers
	if workers > len(targets) {
		workers = len(targets)
	}
	jobs := make(chan int, len(targets))
	for i := range targets {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = inspectDiscoverTarget(ctx, runner, opts, targets[index])
			}
		}()
	}
	wg.Wait()
	return results
}

func inspectDiscoverTarget(ctx context.Context, runner skopeoRunner, opts discoverOptions, target discoverTarget) discoverInspectResult {
	var result discoverInspectResult
	cutoff := opts.Now.Add(-opts.MaxAge)
	for _, sourceIndex := range target.sources {
		image := opts.Images[sourceIndex]
		payload, err := runner.InspectConfig(ctx, image, target.tag.name)
		if err != nil {
			result.relevant = true
			result.warnings = append(result.warnings,
				fmt.Sprintf("could not inspect %s:%s: %v", image, target.tag.name, err))
			continue
		}
		created, err := parseDiscoverCreated(payload)
		if err != nil {
			result.relevant = true
			result.warnings = append(result.warnings,
				fmt.Sprintf("ignoring %s:%s: %v", image, target.tag.name, err))
			continue
		}
		if created.Before(cutoff) {
			continue
		}
		result.relevant = true
		candidate, err := parseDiscoverImageConfig(payload, target.tag, image, sourceIndex)
		if err != nil {
			result.warnings = append(result.warnings,
				fmt.Sprintf("ignoring %s:%s: %v", image, target.tag.name, err))
			continue
		}
		result.candidate = &candidate
		return result
	}
	return result
}

func parseDiscoverCreated(payload []byte) (time.Time, error) {
	var config struct {
		Created string `json:"created"`
	}
	if err := json.Unmarshal(payload, &config); err != nil {
		return time.Time{}, fmt.Errorf("invalid image config: %w", err)
	}
	createdText := strings.TrimSpace(config.Created)
	if createdText == "" {
		return time.Time{}, errors.New("image config has no created timestamp")
	}
	created, err := time.Parse(time.RFC3339Nano, createdText)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid created timestamp %q: %w", createdText, err)
	}
	return created.UTC(), nil
}

func parseDiscoverImageConfig(payload []byte, tag discoverTag, image string, sourceIndex int) (discoverCandidate, error) {
	var config struct {
		Created string `json:"created"`
		Config  struct {
			Labels map[string]string `json:"Labels"`
			Env    []string          `json:"Env"`
		} `json:"config"`
	}
	if err := json.Unmarshal(payload, &config); err != nil {
		return discoverCandidate{}, fmt.Errorf("invalid image config: %w", err)
	}
	created, err := parseDiscoverCreated(payload)
	if err != nil {
		return discoverCandidate{}, err
	}

	imageRevision := strings.TrimSpace(config.Config.Labels["org.opencontainers.image.revision"])
	ossRevision := strings.TrimSpace(config.Config.Labels["org.opencontainers.image.oss.revision"])
	imageSourceLabel := strings.TrimSpace(config.Config.Labels["org.opencontainers.image.source"])
	sourceRevision := imageRevision
	if ossRevision != "" {
		sourceRevision = ossRevision
	}

	versionType := discoverEnvValue(config.Config.Env, "RANCHER_VERSION_TYPE")
	flavor := discoverFlavorCommunity
	if strings.EqualFold(strings.TrimSpace(versionType), discoverFlavorPrime) ||
		strings.Contains(strings.ToLower(imageSourceLabel), "rancher-prime") {
		flavor = discoverFlavorPrime
	}
	if tag.stream == discoverStreamPrimeHead {
		if ossRevision == "" {
			return discoverCandidate{}, errors.New("Prime-head image has no org.opencontainers.image.oss.revision label")
		}
		if flavor != discoverFlavorPrime {
			return discoverCandidate{}, errors.New("Prime-head image is not classified as Prime")
		}
		if !strings.EqualFold(tag.sha, ossRevision) {
			return discoverCandidate{}, fmt.Errorf(
				"tag SHA %s does not match OSS source revision %q", tag.sha, ossRevision)
		}
	}
	return discoverCandidate{
		Tag:            tag.name,
		Created:        created.UTC(),
		SourceKind:     "image",
		ImageSource:    image,
		ImageRevision:  imageRevision,
		SourceRevision: sourceRevision,
		WebhookBuild:   discoverEnvValue(config.Config.Env, "CATTLE_RANCHER_WEBHOOK_VERSION"),
		Stream:         tag.stream,
		Flavor:         flavor,
		tag:            tag,
		sourceIndex:    sourceIndex,
	}, nil
}

func discoverEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func discoverCandidateIsNewer(candidate, current discoverCandidate) bool {
	if !candidate.Created.Equal(current.Created) {
		return candidate.Created.After(current.Created)
	}
	if discoverTagBefore(candidate.tag, current.tag) {
		return true
	}
	if discoverTagBefore(current.tag, candidate.tag) {
		return false
	}
	return candidate.sourceIndex < current.sourceIndex
}

func discoverWarnf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "discover: warning: "+format+"\n", args...)
}

type discoverImageFlags []string

func (f *discoverImageFlags) String() string {
	return strings.Join(*f, ",")
}

func (f *discoverImageFlags) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("image source cannot be empty")
	}
	*f = append(*f, value)
	return nil
}

func cmdDiscover(args []string) {
	code := runDiscoverCLI(context.Background(), args, os.Stdout, os.Stderr, commandSkopeoRunner{})
	if code != 0 {
		os.Exit(code)
	}
}

func runDiscoverCLI(ctx context.Context, args []string, stdout, stderr io.Writer, runner skopeoRunner) int {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var images discoverImageFlags
	maxAgeDays := fs.Int("max-age-days", discoverDefaultMaxAgeDays, "ignore images older than this many days")
	tagsPerLine := fs.Int("tags-per-line", discoverDefaultTagsPerLine, "maximum newest alpha tags to inspect per release line")
	streamsText := fs.String("streams", discoverStreamAlpha+","+discoverStreamPrimeHead, "comma-separated streams: alpha, prime-head")
	fs.Var(&images, "image", "OCI image repository to search (repeatable; source order is precedence)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s discover [options]\n\n", os.Args[0])
		fmt.Fprintln(stderr, "Writes headerless TSV in this stable field order:")
		fmt.Fprintln(stderr, "  tag, created, source_kind, image_source, image_revision, source_revision, webhook_build, stream, flavor")
		fmt.Fprintln(stderr, "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "discover: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 2
	}
	if *maxAgeDays <= 0 {
		fmt.Fprintln(stderr, "discover: -max-age-days must be greater than zero")
		return 2
	}
	maxDurationDays := int(time.Duration(1<<63-1) / (24 * time.Hour))
	if *maxAgeDays > maxDurationDays {
		fmt.Fprintln(stderr, "discover: -max-age-days is too large")
		return 2
	}
	if *tagsPerLine <= 0 {
		fmt.Fprintln(stderr, "discover: -tags-per-line must be greater than zero")
		return 2
	}
	streams, err := parseDiscoverStreams(*streamsText)
	if err != nil {
		fmt.Fprintf(stderr, "discover: %v\n", err)
		return 2
	}

	opts := discoverOptions{
		Images:      append([]string(nil), images...),
		MaxAge:      time.Duration(*maxAgeDays) * 24 * time.Hour,
		TagsPerLine: *tagsPerLine,
		Streams:     streams,
		Now:         time.Now().UTC(),
		Workers:     discoverDefaultWorkers,
	}
	candidates, err := discoverCandidates(ctx, runner, opts, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "discover: %v\n", err)
		return 1
	}
	for _, candidate := range candidates {
		// Individual registry failures are isolated. Discovery returns non-zero
		// only when every source, or an entire matched stream, is unreadable.
		_, _ = fmt.Fprintln(stdout, candidate.TSV())
	}
	return 0
}

func parseDiscoverStreams(value string) (map[string]bool, error) {
	streams := make(map[string]bool)
	for _, raw := range strings.Split(value, ",") {
		stream := strings.TrimSpace(raw)
		switch stream {
		case discoverStreamAlpha, discoverStreamPrimeHead:
			streams[stream] = true
		case "":
			return nil, errors.New("-streams cannot contain an empty stream")
		default:
			return nil, fmt.Errorf("unknown stream %q (want alpha or prime-head)", stream)
		}
	}
	if len(streams) == 0 {
		return nil, errors.New("-streams must select at least one stream")
	}
	return streams, nil
}
