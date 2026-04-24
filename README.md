# Webhook Ranch Hand

Tracks `github.com/rancher/*` version drift between [`rancher/rancher`](https://github.com/rancher/rancher) and [`rancher/webhook`](https://github.com/rancher/webhook) on every new Rancher alpha.

The scheduled action runs three times a day (02, 10, 18 UTC), discovers the newest `-alpha` for each active `v2.X` release line in the last 30 days, and writes a report for any alpha it hasn't already processed. Reports live under [`reports/`](reports/).

<!-- AUTO:DASHBOARD:START -->

## Latest per release line

| Line | Latest alpha | Released | Status | Webhook | Webhook released | Checked | Report |
| --- | --- | --- | --- | --- | --- | --- | --- |
| v2.15 | `v2.15.0-alpha3` | 2026-04-14 | ⚠️ 8 mismatches | `v0.10.0` | 2026-03-18 | 2026-04-24 | [open](reports/v2.15/v2.15.0-alpha3.md) |
| v2.14 | `v2.14.1-alpha7` | 2026-04-24 | ✅ Clean | `v0.10.1-rc.5` | 2026-04-23 | 2026-04-24 | [open](reports/v2.14/v2.14.1-alpha7.md) |
| v2.13 | `v2.13.5-alpha5` | 2026-04-22 | ⚠️ 6 mismatches | `v0.9.3` | 2026-02-18 | 2026-04-24 | [open](reports/v2.13/v2.13.5-alpha5.md) |
| v2.12 | `v2.12.9-alpha5` | 2026-04-22 | ⚠️ 5 mismatches | `v0.8.5` | 2026-01-23 | 2026-04-24 | [open](reports/v2.12/v2.12.9-alpha5.md) |
| v2.11 | `v2.11.13-alpha4` | 2026-04-23 | ⚠️ 4 mismatches | `v0.7.8` | 2025-11-20 | 2026-04-24 | [open](reports/v2.11/v2.11.13-alpha4.md) |

## Recent runs

- 2026-04-24 · [`v2.14.1-alpha7`](reports/v2.14/v2.14.1-alpha7.md) · ✅ Clean
- 2026-04-14 · [`v2.15.0-alpha3`](reports/v2.15/v2.15.0-alpha3.md) · ⚠️ 8 mismatches
- 2026-04-23 · [`v2.11.13-alpha4`](reports/v2.11/v2.11.13-alpha4.md) · ⚠️ 4 mismatches
- 2026-04-22 · [`v2.12.9-alpha5`](reports/v2.12/v2.12.9-alpha5.md) · ⚠️ 5 mismatches
- 2026-04-22 · [`v2.13.5-alpha5`](reports/v2.13/v2.13.5-alpha5.md) · ⚠️ 6 mismatches
- 2026-04-23 · [`v2.14.1-alpha6`](reports/v2.14/v2.14.1-alpha6.md) · ✅ Clean


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
