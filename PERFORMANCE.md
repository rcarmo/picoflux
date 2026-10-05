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
