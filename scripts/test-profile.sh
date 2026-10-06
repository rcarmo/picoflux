#!/usr/bin/env bash
set -uo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
source "$root/scripts/project-tmp.sh"
PROJECT_TMP_ROOT=$(project_tmp_resolve picoflux) || exit 1
project_tmp_init "$PROJECT_TMP_ROOT" || exit 1
export PROJECT_TMP_ROOT
export TMPDIR="$PROJECT_TMP_ROOT/runs/tests/$(date -u +%Y%m%dT%H%M%S)-$$"
export TMP="$TMPDIR" TEMP="$TMPDIR"
export GOCACHE="$PROJECT_TMP_ROOT/cache/go-build"
export GOMODCACHE="$PROJECT_TMP_ROOT/cache/go-mod"
export GOBIN="$PROJECT_TMP_ROOT/build/bin"
mkdir -p "$TMPDIR" "$GOCACHE" "$GOMODCACHE" "$GOBIN"
# This process owns its unique scratch directory; never remove another run.
trap 'rm -rf -- "$TMPDIR"' EXIT
run="${PROFILE_RUN_DIR:-$PROJECT_TMP_ROOT/runs/profiles/$(date -u +%Y%m%dT%H%M%S)-$$}"
mkdir -p "$run"
echo "Profiles: $run"
{ git rev-parse HEAD; go version; printf "packages=%s flags=%s heap_sampling=%s\n" "${TEST_PACKAGES:-./...}" "$*" "${MEMPROFILE_RATE:-1}"; } > "$run/metadata.txt"
packages=${TEST_PACKAGES:-./...}
rc=0
for pkg in $(go list $packages); do
  dir="$run/${pkg//\//_}"
  mkdir -p "$dir"
  if go test -count=1 -cpuprofile="$dir/cpu.pprof" -memprofile="$dir/heap.pprof" -memprofilerate=${MEMPROFILE_RATE:-1} -o "$dir/test.bin" "$@" "$pkg" >"$dir/test.log" 2>&1; then
    result=0
  else
    result=$?
  fi
  cat "$dir/test.log"
  if ((result != 0)); then rc=1; fi
  if [[ -f "$dir/cpu.pprof" && -f "$dir/heap.pprof" ]]; then
    for mode in cpu alloc_space alloc_objects; do
      profile="$dir/heap.pprof"
      args=(-"$mode")
      if [[ "$mode" == cpu ]]; then profile="$dir/cpu.pprof"; args=(); fi
      go tool pprof -top -cum "${args[@]}" "$dir/test.bin" "$profile" >"$dir/$mode.txt" 2>&1 || rc=1
    done
  elif grep -q '\[no test files\]' "$dir/test.log"; then
    echo "$pkg: no tests; no profiles expected"
  else
    echo "MISSING PROFILES: $pkg" >&2
    rc=1
  fi
done
exit "$rc"
