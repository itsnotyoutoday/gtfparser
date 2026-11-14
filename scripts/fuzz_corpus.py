#!/usr/bin/env python3
"""Run case files (default testdata/cases2/*.tf) through real tf and gtf and diff the captured output.

Like difftest.py but tolerant of hangs/crashes in either implementation and of non-UTF-8 output.
usage: . scripts/env.sh; fuzz_corpus.py [--gtf PATH] [--quiet] [files...]
"""
import argparse, difflib, glob, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fuzz_common as C

ap = argparse.ArgumentParser()
ap.add_argument('--gtf')
ap.add_argument('--quiet', action='store_true')
ap.add_argument('--timeout', type=int, default=25)
ap.add_argument('files', nargs='*')
a = ap.parse_args()
if a.gtf: C.GTF = a.gtf
files = a.files or sorted(glob.glob(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'testdata', 'cases2', '*.tf')))
tot = bad = lines = 0
for f in files:
    s = open(f, encoding='utf-8').read()
    want, _, st1 = C.run_tf(s, a.timeout)
    got, _, st2 = C.run_gtf(s, a.timeout)
    tot += 1
    lines += len(want.splitlines())
    note = ''.join(' [tf %s]' % st1 if st1 else '') + ''.join(' [gtf %s]' % st2 if st2 else '')
    if want == got and not note:
        print('ok  ', os.path.basename(f), '(%d lines)' % len(want.splitlines()))
        continue
    bad += 1
    print('DIFF', os.path.basename(f), note)
    if not a.quiet:
        for l in difflib.unified_diff(want.splitlines(), got.splitlines(), 'tf', 'gtf', lineterm='', n=0):
            print('   ', l)
print('files: %d  diverging: %d  (tf output lines compared: %d)' % (tot, bad, lines))
sys.exit(1 if bad else 0)
