# Webhook Ranch Hand

Tracks `github.com/rancher/*` version drift between [`rancher/rancher`](https://github.com/rancher/rancher) and [`rancher/webhook`](https://github.com/rancher/webhook) on every new Rancher alpha.

The scheduled action runs three times a day (02, 10, 18 UTC), discovers the newest `-alpha` for each active `v2.X` release line in the last 30 days, and writes a report for any alpha it hasn't already processed. Reports live under [`reports/`](reports/).

<!-- AUTO:DASHBOARD:START -->

_No reports yet — run the workflow or add a version to the manual queue below._

<!-- AUTO:DASHBOARD:END -->

## Manual runs queue

Need a one-off check (e.g. an RC, or an older alpha)? Add it as a bullet between the markers below — one version per line, with a leading `v`. The next scheduled run will process it and delete the line on success. Failed entries are left in place so they retry automatically.

<!-- MANUAL-QUEUE:START -->

<!-- example: -->
<!-- - v2.14.0-rc.1    -->
<!-- - v2.11.13-alpha1 -->

<!-- MANUAL-QUEUE:END -->

## How it works

1. **Discover.** `gh api /repos/rancher/rancher/releases` is filtered to prereleases matching `^v\d+\.\d+\.\d+-alpha\d+$`, within the last 30 days, and reduced to the newest alpha per release line.
2. **Process.** For each candidate, the action downloads `rancher/rancher@<tag>` and resolves the webhook pin from `build.yaml`, then runs [`scripts/compare-gomod`](scripts/compare-gomod) against both `go.mod` files.
3. **Classify.** `replace` directives are applied so the comparison is against *effective* versions, not raw `require` pins. `pkg/apis` and `pkg/client` drift is expected (rancher replaces them locally) and is filtered out of the mismatch count.
4. **Index.** After every run, the dashboard table above and the per-line index pages in [`reports/`](reports/) are regenerated from the on-disk reports. Nothing else in this README is touched.

## Archive

Historical reports (pre-2026-04, plaintext format) live in [`archieve/`](archieve/). They are not regenerated — treat them as a read-only record.
