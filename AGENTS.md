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
