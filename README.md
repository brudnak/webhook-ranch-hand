# Webhook Ranch Hand

Tracks `github.com/rancher/*` version drift between [`rancher/rancher`](https://github.com/rancher/rancher) and [`rancher/webhook`](https://github.com/rancher/webhook) on every new Rancher alpha.

The scheduled action runs three times a day (02, 10, 18 UTC), discovers the newest Rancher `-alpha` images for each active `v2.X` release line in the last 30 days, and writes a report for any alpha it hasn't already processed. Reports live under [`reports/`](reports/).

<!-- AUTO:DASHBOARD:START -->

## Latest per release line

| Line | Latest alpha | Rancher date | Source | Status | Webhook | Webhook date | Checked | Report |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| v2.15 | `v2.15.0-alpha4` | 2026-05-08 | Image built | ⚠️ 7 mismatches | `v0.11.0-rc.2` | 2026-04-03 | 2026-05-22 | [open](reports/v2.15/v2.15.0-alpha4.md) |
| v2.14 | `v2.14.2-alpha6` | 2026-05-21 | Image built | ⚠️ 1 mismatch | `v0.10.5-rc.4` | 2026-05-19 | 2026-05-22 | [open](reports/v2.14/v2.14.2-alpha6.md) |
| v2.13 | `v2.13.6-alpha5` | 2026-05-20 | Image built | ⚠️ 1 mismatch | `v0.9.5-rc.5` | 2026-05-19 | 2026-05-22 | [open](reports/v2.13/v2.13.6-alpha5.md) |
| v2.12 | `v2.12.10-alpha4` | 2026-05-21 | Image built | ⚠️ 5 mismatches | `v0.8.6` | 2026-04-29 | 2026-05-22 | [open](reports/v2.12/v2.12.10-alpha4.md) |
| v2.11 | `v2.11.14-alpha1` | 2026-05-20 | Image built | ⚠️ 5 mismatches | `v0.7.9` | 2026-04-29 | 2026-05-22 | [open](reports/v2.11/v2.11.14-alpha1.md) |
| v2.10 | `v2.10.12-alpha1` | 2026-05-20 | Image built | ⚠️ 1 mismatch | `v0.6.12` | 2026-01-27 | 2026-05-22 | [open](reports/v2.10/v2.10.12-alpha1.md) |

## Recent runs

- 2026-05-21 · [`v2.12.10-alpha4`](reports/v2.12/v2.12.10-alpha4.md) · ⚠️ 5 mismatches
- 2026-05-21 · [`v2.14.2-alpha6`](reports/v2.14/v2.14.2-alpha6.md) · ⚠️ 1 mismatch
- 2026-05-20 · [`v2.13.6-alpha5`](reports/v2.13/v2.13.6-alpha5.md) · ⚠️ 1 mismatch
- 2026-05-20 · [`v2.11.14-alpha1`](reports/v2.11/v2.11.14-alpha1.md) · ⚠️ 5 mismatches
- 2026-05-20 · [`v2.10.12-alpha1`](reports/v2.10/v2.10.12-alpha1.md) · ⚠️ 1 mismatch
- 2026-05-20 · [`v2.14.2-alpha5`](reports/v2.14/v2.14.2-alpha5.md) · ⚠️ 1 mismatch
- 2026-05-20 · [`v2.14.2-alpha4`](reports/v2.14/v2.14.2-alpha4.md) · ⚠️ 1 mismatch
- 2026-05-18 · [`v2.12.10-alpha3`](reports/v2.12/v2.12.10-alpha3.md) · ⚠️ 5 mismatches
- 2026-05-18 · [`v2.13.6-alpha4`](reports/v2.13/v2.13.6-alpha4.md) · ⚠️ 6 mismatches
- 2026-05-15 · [`v2.13.6-alpha3`](reports/v2.13/v2.13.6-alpha3.md) · ⚠️ 6 mismatches


<!-- AUTO:DASHBOARD:END -->

## Manual runs queue

Need a one-off check (e.g. an RC, or an older alpha)? Add it as a bullet between the markers below — one version per line, with a leading `v`. The next scheduled run will process it and delete the line on success. Failed entries are left in place so they retry automatically.

<!-- MANUAL-QUEUE:START -->

<!-- example: -->
<!-- - v2.14.0-rc.1    -->
<!-- - v2.11.13-alpha1 -->

<!-- MANUAL-QUEUE:END -->

## How it works

1. **Discover.** Rancher image registries are searched for tags matching `^v\d+\.\d+\.\d+-alpha\d+$`. Image `created` time provides the 30-day window, and the newest alpha per release line is selected.
2. **Process.** For each candidate, the action downloads `rancher/rancher@<tag>` and resolves the webhook pin from `build.yaml`, then runs [`scripts/compare-gomod`](scripts/compare-gomod) against both `go.mod` files. Image metadata is used for discovery only; `build.yaml` remains the source of truth for the webhook pin.
3. **Classify.** `replace` directives are applied so the comparison is against *effective* versions, not raw `require` pins. `pkg/apis` and `pkg/client` drift is expected (rancher replaces them locally) and is filtered out of the mismatch count.
4. **Index.** After every run, the dashboard table above and the per-line index pages in [`reports/`](reports/) are regenerated from the on-disk reports. Nothing else in this README is touched.

## Archive

Historical reports (pre-2026-04, plaintext format) live in [`archieve/`](archieve/). They are not regenerated — treat them as a read-only record.
