#!/usr/bin/env python3
"""Compare run time of testdata/bench/*.tf in real tf and gtf (startup time subtracted)."""
import os, sys, time, glob
src = open(os.path.join(os.path.dirname(__file__), 'difftest.py')).read().split("ap = argparse")[0]
exec(src)
tf, lib, gtf = os.environ['TF_REF'], os.environ['TF_LIB'], os.environ.get('GTF', '/tmp/gtfrun')
scale = int(os.environ.get('SCALE', '1'))
def timed(fn, s, n=3):
    best = 1e9
    for _ in range(n):
        t = time.time(); fn(s); best = min(best, time.time() - t)
    return best
base_tf = timed(lambda s: run_tf(tf, lib, s, timeout=300), '/echo x')
base_g = timed(lambda s: run_gtf(gtf, lib, s), '/echo x')
print(f'startup: tf {base_tf:.2f}s gtf {base_g:.2f}s')
print(f"{'case':10} {'tf':>8} {'gtf':>8} {'gtf/tf':>7}")
for f in sorted(glob.glob(os.path.join(os.path.dirname(__file__), '..', 'testdata', 'bench', '*.tf'))):
    s = open(f).read()
    if scale != 1:
        import re
        s = re.sub(r'(?<![\w.])(\d{4,})(?![\w.])', lambda m: str(int(m.group(1)) * scale), s)
    a = timed(lambda x: run_tf(tf, lib, x, timeout=300), s, 1) - base_tf
    b = timed(lambda x: run_gtf(gtf, lib, x), s, 1) - base_g
    print(f'{os.path.basename(f):10} {a:7.2f}s {b:7.2f}s {b / max(a, 0.001):6.1f}x')
