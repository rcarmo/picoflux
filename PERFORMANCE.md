# Performance review — October 2026

Pre-release profiling runs through scripts/test-profile.sh. Raw profiles, matching
binaries and reports use the project-owned runs/profiles hierarchy and are deleted
immediately after analysis. Retain concise conclusions here, not raw evidence.
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

Full suite rerun with CPU/heap capture after changes, with analysed cumulative
CPU/alloc_space/alloc_objects tables. Short unit tests can have empty CPU samples;
the representative rendering benchmarks provide usable CPU evidence. Test
fixtures and profiler overhead remain distinct from application hotspots.

## Dependency refresh — October 6, 2026

Updated all direct Go modules and their selected transitive dependencies, retaining
modernc SQLite, plus CI actions (immutable release pins), ESLint flat configuration,
Alpine 3.24 and Go 1.27.1. The old local Go 1.26.3 had ten reachable standard-library
vulnerability findings; scanning with 1.27.1 reports none reachable. One module-only
advisory remains outside called packages/symbols; this is not a claim of no advisory
anywhere in the module graph.

Equivalent three-run CPU/heap-profiled benchmarks on the same host:
- Before refresh: render 13.11–13.62 us/op, 17.33 KB, 623 allocations;
  e-ink 2.145–2.153 ms/op, 631 KB, 2 allocations.
- Updated dependencies, old toolchain: render 12.95–13.44 us/op;
  e-ink 2.088–2.113 ms/op; allocation counts unchanged.
- Final Go 1.27.1: render 12.23–13.51 us/op; e-ink 2.170–2.227 ms/op;
  allocation counts unchanged. No material dependency-driven speedup claimed.

CPU profiles identified per-pixel offset arithmetic as a candidate. Hoisting it
passed non-zero-origin and subimage/stride reference tests but comparative timing
regressed (2.60–2.90 ms/op); discarded it, retaining the additional regression test.
Remaining allocations are chiefly HTML execution and the required grayscale output
buffer. Full profiled tests, focused race tests, vet/build and five static Linux
cross-builds passed. Alpine build verifies the binary, certificates and timezone
assets. Disposable run artifacts removed after analysis.
