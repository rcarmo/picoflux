# picoflux engineering rules

Follow /workspace/AGENTS.md for coordination, Git identity, and push hygiene.

## Mandatory test profiling

Never run tests without CPU and heap/allocation profiling and post-run analysis.
This applies to focused tests, race tests, benchmarks, fuzzing, integration tests,
failed runs, and delegated work. Use the profiling-aware Makefile targets.

Retain binaries, logs, CPU profiles, heap profiles, and cumulative CPU,
alloc_space, and alloc_objects reports. Review hotspots after every run and
compare like workloads to identify allocation reductions and performance wins.
Generated reports alone do not replace engineering analysis. Missing profiles
following compilation failures or abrupt termination must be reported explicitly.
Do not claim the profiling gate passed when evidence is absent.

Preserve SQLite migrations, pure-Go builds, picoflux branding, and tag-only
release workflows when adopting upstream Miniflux changes.

## Project cache and temporary-file policy

Canonical project name: **picoflux** (repository checkout may be named nanoflux).
All disposable output belongs under `/workspace/tmp/picoflux/`:
`cache/<tool>/`, `build/`, and `runs/<purpose>/<run-id>/`.
Make exports TMPDIR/TMP/TEMP, GOCACHE/GOMODCACHE/GOBIN, Bun/npm,
Python/uv and Playwright paths. Direct commands and delegates must use these
same variables. The profiling helper configures isolated test scratch itself;
t.TempDir inherits TMPDIR. Do not use bare /tmp or home caches.

Retain CPU/heap profiles, binaries, logs, and receipts in `.profiles/`, separate
from scratch. Never delete retained evidence or another project's root in clean.
Do not relocate active jobs, installed toolchains, source, or durable data.
CI may set PROJECT_TMP_ROOT to `${RUNNER_TEMP}/picoflux` as its explicit
project-owned mapping; use the identical cache/build/runs hierarchy there.

### Portable root resolution (supersedes workspace-only defaults)

The vendored `scripts/project-tmp.sh` resolves the canonical root once, before
exporting child TMPDIR/TMP/TEMP. An explicit PROJECT_TMP_ROOT must be absolute,
usable, project-owned, non-symlink and end in `picoflux`; invalid overrides fail.
Without an override: writable `/workspace/tmp/picoflux`, then
`${RUNNER_TEMP}/picoflux`, original `${TMPDIR}/picoflux`, then platform
`/tmp/picoflux`. Generic fallbacks are supported on non-CI hosts too.
All choices use the same cache/build/runs hierarchy; never append the project
name recursively after TMPDIR has been redirected. No workspace helper is
required by the repository. Clean only owned disposable output, never retained
`.profiles` evidence, active-job files, installed tools or another project.
