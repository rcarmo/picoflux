#!/usr/bin/env bash
set -uo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
run="$root/.profiles/$(date -u +%Y%m%dT%H%M%S)-$$"
mkdir -p "$run"
echo "Profiles: $run"
packages=${TEST_PACKAGES:-./...}
rc=0
for pkg in $(go list $packages); do
  dir="$run/${pkg//\//_}"
  mkdir -p "$dir"
  go test -count=1 -cpuprofile="$dir/cpu.pprof" -memprofile="$dir/heap.pprof" -memprofilerate=1 -o "$dir/test.bin" "$@" "$pkg" >"$dir/test.log" 2>&1
  result=$?
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
