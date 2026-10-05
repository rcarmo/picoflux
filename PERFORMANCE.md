# Performance review — October 2026

Every test now runs through scripts/test-profile.sh. Profiles, retained test
binaries, logs, cumulative CPU and allocation reports live in .profiles/.
Run focused workloads with TEST_PACKAGES and benchmark flags; do not compare
race-detector timings with production or non-race timings.

## Measured improvement

900x700 RGBA e-ink conversion, same machine/workload, two-second profiled runs:

| Measure | Before | After |
|---|---:|---:|
| ns/op | 28,154,062 | 2,733,356 |
| bytes/op | 23,469,662 | 630,866 |
| allocations/op | 10 | 2 |

The old code ran CatmullRom scaling even when dimensions were unchanged.
The new path reuses the source RGBA buffer read-only and writes grayscale pixels
directly. Smaller non-RGBA and downscaled images retain the established scaler.
Reference tests compare the original luma/contrast formula, including non-zero
origins. Exact comparisons are appropriate here because the integer conversion
contract is unchanged, not because of a generated-image hash.

## Full-suite analysis

The focused race suite and full non-race suite captured CPU/heap profiles.
Storage test CPU is dominated by bcrypt fixture creation (~85% in the non-race
run): preserve security cost rather than optimizing away password hashing.
Template allocations include per-render work and parsing/setup; realistic
render benchmarks are needed before changing concurrent rendering semantics.
Sanitizer profiles include deep-nesting fixtures; short fetcher/sanitizer CPU
profiles are too sparse for speed conclusions. Allocation profiling itself is
visible overhead, so reports must not be mistaken for production throughput.

SQLite migration v5 removes entries_user_status_changed_idx, whose lookup prefix
is already covered by entries_user_status_changed_published_idx. This reduces
index maintenance/storage without removing a lookup capability. The PostgreSQL
bytea enclosure index change is deliberately not applied: SQLite already indexes
URLs directly and has no PostgreSQL btree tuple-size constraint.

## Template rendering follow-up

Immutable per-view/language templates are now cached after locale bindings are
installed, avoiding request-time cloning without sharing mutable function maps.
The cache is cleared when templates are reparsed. Profiled race tests pass.

Parallel 100-row render workload, three three-second runs, CPU capture plus
512 KiB heap sampling on both baseline and candidate:

- Baseline: 31.7–32.9 us/op, 29.4 KB/op, 728 allocations/op.
- Cached: 27.0–32.5 us/op, 17.3 KB/op, 623 allocations/op.

The clear result is ~41% fewer bytes and 105 fewer allocations per request;
latency ranges overlap, so no robust throughput speedup is claimed. Full
allocation sampling heavily distorts execution timings and was not used for
latency acceptance. Remaining profiles are dominated by html/template execution,
not clone setup. Replacing the safe template engine is not justified by this
bounded benchmark.

Full suite rerun with CPU/heap capture after changes, with retained cumulative
CPU/alloc_space/alloc_objects tables. Short unit tests can have empty CPU samples;
the representative rendering benchmarks provide usable CPU evidence. Test
fixtures and profiler overhead remain distinct from application hotspots.
