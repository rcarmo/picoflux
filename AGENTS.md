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

## Portable project caches and scratch

Canonical project name: **picoflux**, independent of checkout directory name.
The vendored `scripts/project-tmp.sh` resolves the root before changing child
TMPDIR. Make snapshots and exports PROJECT_ORIGINAL_TMPDIR; the helper does
likewise when invoked directly. Children receive the resolved PROJECT_TMP_ROOT
so resolution cannot recursively append project names.

- Absolute PROJECT_TMP_BASE selects `<base>/picoflux`.
- Absolute PROJECT_TMP_ROOT is supported for compatibility and must end in
  `picoflux`. When both overrides are supplied they must agree.
- Invalid, unowned, unwritable, symlink or conflicting overrides fail, never
  silently fall back.
- CI: usable RUNNER_TEMP, then original inherited TMPDIR, then platform temp,
  always appending `picoflux`; CI never prefers an existing workspace mount.
- Local: writable `/workspace/tmp`, otherwise platform temp (POSIX `/tmp`),
  always appending `picoflux`.

Layout: `cache/<tool>/`, `build/`, `tests/`, `logs/`, and
`runs/<purpose>/<run-id>/`. Make exports TMPDIR/TMP/TEMP and Go, Bun/npm,
Python/uv and Playwright cache variables through this root. Direct commands and
delegates must use the same configuration. Profiling helpers create isolated
per-run scratch; t.TempDir inherits TMPDIR. No host helper is required.

Retained CPU/heap profiles, binaries, logs and receipts remain in `.profiles/`,
separate from disposable scratch. Clean only owned disposable files; never
remove evidence, another project's root, active-job files, installed toolchains,
source or durable data. Do not relocate files used by active jobs.
