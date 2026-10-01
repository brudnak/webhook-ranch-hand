# Webhook Ranch Hand

Tracks `github.com/rancher/*` version drift between [`rancher/rancher`](https://github.com/rancher/rancher) and [`rancher/webhook`](https://github.com/rancher/webhook) across Rancher alpha and Prime head builds.

The scheduled action runs three times a day (02, 10, 18 UTC). For each active `v2.X` release line it discovers the newest immutable alpha and Prime head images from the last 30 days, then writes a report for every build it has not already processed. Reports live under [`reports/`](reports/).

<!-- AUTO:DASHBOARD:START -->

## Latest per release line and stream

_Prime head labels use a seven-character SHA on this page; open a report for the full tag and revision details._

| Line | Stream | Latest build | Status | Webhook | Checked | Report |
| --- | --- | --- | --- | --- | --- | --- |
| v2.16 | `prime-head` | [`v2.16.0-940e5ad-head`](reports/v2.16/v2.16.0-940e5ad-head.md) | ⚠️ 8 mismatches | `v0.12.1-rc.6` | 2026-10-01 | [open](reports/v2.16/v2.16.0-940e5ad-head.md) |
| v2.15 | `prime-head` | [`v2.15.3-19c9298-head`](reports/v2.15/v2.15.3-19c9298-head.md) | ⚠️ 3 mismatches | `v0.11.3` | 2026-09-30 | [open](reports/v2.15/v2.15.3-19c9298-head.md) |
| v2.15 | `alpha` | [`v2.15.1-alpha1`](reports/v2.15/v2.15.1-alpha1.md) | ⚠️ 7 mismatches | `v0.11.1-rc.1` | 2026-08-19 | [open](reports/v2.15/v2.15.1-alpha1.md) |
| v2.14 | `prime-head` | [`v2.14.7-f42e1ac-head`](reports/v2.14/v2.14.7-f42e1ac-head.md) | ⚠️ 1 mismatch | `v0.10.12` | 2026-09-30 | [open](reports/v2.14/v2.14.7-f42e1ac-head.md) |
| v2.14 | `alpha` | [`v2.14.5-alpha1`](reports/v2.14/v2.14.5-alpha1.md) | ✅ Clean | `v0.10.10-rc.2` | 2026-08-19 | [open](reports/v2.14/v2.14.5-alpha1.md) |
| v2.13 | `prime-head` | [`v2.13.11-4b6da74-head`](reports/v2.13/v2.13.11-4b6da74-head.md) | ⚠️ 1 mismatch | `v0.9.10` | 2026-09-30 | [open](reports/v2.13/v2.13.11-4b6da74-head.md) |
| v2.13 | `alpha` | [`v2.13.9-alpha1`](reports/v2.13/v2.13.9-alpha1.md) | ✅ Clean | `v0.9.8-rc.2` | 2026-08-19 | [open](reports/v2.13/v2.13.9-alpha1.md) |
| v2.12 | `prime-head` | [`v2.12.15-02e41b6-head`](reports/v2.12/v2.12.15-02e41b6-head.md) | ⚠️ 1 mismatch | `v0.8.11` | 2026-09-30 | [open](reports/v2.12/v2.12.15-02e41b6-head.md) |
| v2.12 | `alpha` | [`v2.12.13-alpha1`](reports/v2.12/v2.12.13-alpha1.md) | ⚠️ 4 mismatches | `v0.8.9` | 2026-08-19 | [open](reports/v2.12/v2.12.13-alpha1.md) |
| v2.11 | `prime-head` | [`v2.11.19-17b3e2d-head`](reports/v2.11/v2.11.19-17b3e2d-head.md) | ⚠️ 1 mismatch | `v0.7.12` | 2026-09-30 | [open](reports/v2.11/v2.11.19-17b3e2d-head.md) |
| v2.11 | `alpha` | [`v2.11.17-alpha1`](reports/v2.11/v2.11.17-alpha1.md) | ⚠️ 5 mismatches | `v0.7.10` | 2026-08-19 | [open](reports/v2.11/v2.11.17-alpha1.md) |
| v2.10 | `alpha` | [`v2.10.12-alpha1`](reports/v2.10/v2.10.12-alpha1.md) | ⚠️ 1 mismatch | `v0.6.12` | 2026-05-22 | [open](reports/v2.10/v2.10.12-alpha1.md) |

## Recent runs

- 2026-10-01 · `prime-head` · [`v2.16.0-940e5ad-head`](reports/v2.16/v2.16.0-940e5ad-head.md) · ⚠️ 8 mismatches
- 2026-09-30 · `prime-head` · [`v2.16.0-bf639bd-head`](reports/v2.16/v2.16.0-bf639bd-head.md) · ⚠️ 9 mismatches
- 2026-09-30 · `prime-head` · [`v2.13.11-4b6da74-head`](reports/v2.13/v2.13.11-4b6da74-head.md) · ⚠️ 1 mismatch
- 2026-09-30 · `prime-head` · [`v2.14.7-f42e1ac-head`](reports/v2.14/v2.14.7-f42e1ac-head.md) · ⚠️ 1 mismatch
- 2026-09-30 · `prime-head` · [`v2.12.15-02e41b6-head`](reports/v2.12/v2.12.15-02e41b6-head.md) · ⚠️ 1 mismatch
- 2026-09-30 · `prime-head` · [`v2.15.3-19c9298-head`](reports/v2.15/v2.15.3-19c9298-head.md) · ⚠️ 3 mismatches
- 2026-09-30 · `prime-head` · [`v2.11.19-17b3e2d-head`](reports/v2.11/v2.11.19-17b3e2d-head.md) · ⚠️ 1 mismatch
- 2026-09-30 · `prime-head` · [`v2.16.0-e1cbe45-head`](reports/v2.16/v2.16.0-e1cbe45-head.md) · ⚠️ 9 mismatches
- 2026-09-29 · `prime-head` · [`v2.16.0-6cd75e1-head`](reports/v2.16/v2.16.0-6cd75e1-head.md) · ⚠️ 9 mismatches
- 2026-09-29 · `prime-head` · [`v2.16.0-b55846c-head`](reports/v2.16/v2.16.0-b55846c-head.md) · ⚠️ 6 mismatches


<!-- AUTO:DASHBOARD:END -->

## Manual runs queue

Need a one-off check (for example an RC, an older alpha, or an immutable SHA-qualified head)? Add it as a bullet between the markers below — one image tag per line, with a leading `v`. The next scheduled run will process it and delete the line on success. Failed entries are left in place so they retry automatically. Do not queue a moving alias such as `v2.14-head`.

<!-- MANUAL-QUEUE:START -->

<!-- example: -->
<!-- - v2.14.0-rc.1    -->
<!-- - v2.11.13-alpha1 -->
<!-- - v2.14.5-0123456-head -->

<!-- MANUAL-QUEUE:END -->

## How it works

1. **Discover.** Rancher image registries are searched for immutable alpha tags (`vX.Y.Z-alphaN`), while Prime head tags (`vX.Y.Z-{SHA}-head`) are read only from `stgregistry.suse.com`. The newest image per release line and stream is selected by image `created` time. For Prime heads, `Z` is the highest validated numeric patch currently published for that line. Moving aliases such as `head` and `vX.Y-head` are deliberately ignored so a report always identifies one build. A staging outage fails Prime-head discovery instead of silently falling back to a Community registry.
2. **Resolve source.** Community images identify the public Rancher commit with `org.opencontainers.image.revision`. Prime images have a separate wrapper revision, so Ranch Hand uses `org.opencontainers.image.oss.revision` for the public [`rancher/rancher`](https://github.com/rancher/rancher) source and records both revisions. The SHA embedded in a Prime head tag must match the corresponding prefix of that full OSS revision (legacy 40-character tags match it in full).
3. **Process.** For each candidate, the action downloads the resolved public Rancher commit and reads the webhook pin from its `build.yaml`, then runs [`scripts/compare-gomod`](scripts/compare-gomod) against both `go.mod` files. `build.yaml` remains the source of truth because Prime image environments may omit the webhook pin.
4. **Classify.** `replace` directives are applied so the comparison is against *effective* versions, not raw `require` pins. `pkg/apis` and `pkg/client` drift is expected (rancher replaces them locally) and is filtered out of the mismatch count.
5. **Index.** After every run, the dashboard table above and the per-line index pages in [`reports/`](reports/) are regenerated from the on-disk reports. Alpha and Prime head streams remain separate. Nothing else in this README is touched.

## Archive

Historical reports (pre-2026-04, plaintext format) live in [`archieve/`](archieve/). They are not regenerated — treat them as a read-only record.
