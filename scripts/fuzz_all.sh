#!/bin/sh
# Build gtf, run the expression fuzzer, the macro fuzzer and the cases2 library corpus
# against real tf, and print a summary.
#
# usage: scripts/fuzz_all.sh [-s SEED] [-n COUNT] [-m MACRO_COUNT] [-q]
#   -s SEED         random seed for both fuzzers (default 1)
#   -n COUNT        number of random expressions (default 1500)
#   -m MACRO_COUNT  number of random macro programs (default 120)
#   -q              quiet: only the summary (details go to $OUT)
# Environment: GTFSRC (engine checkout to build from), OUT (log dir, default /tmp/gtf_fuzz_out), GTF (binary, default /tmp/gtf_fuzz)
SEED=1; N=1500; M=120; Q=0
while getopts s:n:m:q o; do
  case $o in s) SEED=$OPTARG;; n) N=$OPTARG;; m) M=$OPTARG;; q) Q=1;; *) exit 2;; esac
done
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
# TF_REF/TF_LIB: set by the caller, or via  TFREF_DIR=<built tinyfugue tree> . scripts/env.sh
[ -z "$TF_REF" ] && [ -f "$HERE/env.sh" ] && . "$HERE/env.sh"
export GOWORK=off
OUT=${OUT:-/tmp/gtf_fuzz_out}
export GTF=${GTF:-/tmp/gtf_fuzz}
mkdir -p "$OUT"
# GTFSRC: engine checkout to build the runner from (cmd/gtfrun); otherwise $GTF must already exist.
if [ -n "$GTFSRC" ]; then
  (cd "$GTFSRC" && go build -o "$GTF" ./cmd/gtfrun) || { echo "build failed"; exit 1; }
fi
[ -x "$GTF" ] || { echo "no runner at $GTF (set GTFSRC=<engine checkout> or build it)"; exit 1; }
cd "$ROOT" || exit 1
echo "== gtf built: $GTF   tf: $TF_REF"

echo "== expression fuzzer (seed=$SEED count=$N)"
python3 scripts/fuzz_expr.py --seed "$SEED" --count "$N" --gtf "$GTF" > "$OUT/expr.txt" 2>&1; E=$?
[ $Q = 0 ] && cat "$OUT/expr.txt"
echo "== macro fuzzer (seed=$SEED count=$M)"
python3 scripts/fuzz_macro.py --seed "$SEED" --count "$M" --gtf "$GTF" > "$OUT/macro.txt" 2>&1; Mx=$?
[ $Q = 0 ] && cat "$OUT/macro.txt"
echo "== library corpus (testdata/cases2)"
python3 scripts/fuzz_corpus.py --gtf "$GTF" --quiet > "$OUT/corpus.txt" 2>&1; Cx=$?
[ $Q = 0 ] && cat "$OUT/corpus.txt"

echo
echo "================ SUMMARY (logs in $OUT) ================"
grep -E '^(expressions run|diverging categories|tf hangs)' "$OUT/expr.txt" | sed 's/^/expr : /'
grep -E '^(programs run|diverging categories|hangs)' "$OUT/macro.txt" | sed 's/^/macro: /'
tail -1 "$OUT/corpus.txt" | sed 's/^/corpus: /'
[ $E = 0 ] && [ $Mx = 0 ] && [ $Cx = 0 ] && { echo "ALL MATCH"; exit 0; }
echo "DIVERGENCES FOUND"; exit 1
