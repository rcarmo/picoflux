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
