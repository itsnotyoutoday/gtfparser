#!/usr/bin/env python3
"""Shared helpers for fuzz_expr.py / fuzz_macro.py: batched runs through real tf and gtf.

A batch is a list of "items" (each a list of TF script lines).  Before each item
the script emits a marker line @@<i>@@ via E, so the captured output can be split
per item (an item that errors in tf simply produces no lines between markers).
"""
import os, pty, re, select, subprocess, sys, tempfile, time, fcntl, struct, termios

TF = os.environ.get('TF_REF')
LIB = os.environ.get('TF_LIB')
GTF = os.environ.get('GTF', '/tmp/gtf_fuzz')
PRELUDE = ('/test _o := tfopen("%s", "w")\n'
           '/def -i echo = /test tfwrite(_o, {*})\n/def -i E = /test tfwrite(_o, {1})\n')
STATS = {'tf_timeouts': 0, 'tf_crashes': 0, 'gtf_timeouts': 0}
SCREENS = []


def _env(d):
    return {'TERM': 'dumb', 'PATH': os.environ['PATH'], 'HOME': d, 'TZ': 'UTC',
            'LANG': os.environ.get('LANG', ''), 'LC_ALL': os.environ.get('LC_ALL', '')}


def run_tf(script, timeout=20):
    d = tempfile.mkdtemp()
    out, src = os.path.join(d, 'out'), os.path.join(d, 'case.tf')
    with open(src, 'w') as f:
        f.write(PRELUDE % out)
        f.write(script)
        f.write('\n/test tfclose(_o)\n/quit\n')
    m, s = pty.openpty()
    fcntl.ioctl(s, termios.TIOCSWINSZ, struct.pack('HHHH', 50, 4000, 0, 0))
    p = subprocess.Popen([TF, '-n', '-f' + src, '-L' + LIB], stdin=s, stdout=s, stderr=s,
                         env=_env(d), close_fds=True)
    buf = []
    t = time.time()
    while p.poll() is None and time.time() - t < timeout:
        r, _, _ = select.select([m], [], [], 0.2)
        if r:
            try: buf.append(os.read(m, 65536))
            except OSError: break
    hung = p.poll() is None
    if hung:
        p.kill(); p.wait()
    os.close(m); os.close(s)
    screen = re.sub(r'\x1b\[[0-9;?]*[A-Za-z]|\x1b[()][A-Z0-9]|\r', '', b''.join(buf).decode(errors='replace'))
    res = open(out, encoding='utf-8', errors='replace').read() if os.path.exists(out) else ''
    crashed = (not hung) and p.returncode not in (0, None) and p.returncode < 0
    return res, screen, ('hang' if hung else 'crash' if crashed else None)


def run_gtf(script, timeout=20):
    d = tempfile.mkdtemp()
    out, src = os.path.join(d, 'out'), os.path.join(d, 'case.tf')
    with open(src, 'w') as f:
        f.write(PRELUDE % out)
        f.write(script)
        f.write('\n/test tfclose(_o)\n')
    st = None
    try:
        r = subprocess.run([GTF, '-f', src, '-L', LIB], capture_output=True, text=True, timeout=timeout,
                           env=_env(d), stdin=subprocess.DEVNULL)
        if r.returncode < 0 or 'panic' in r.stderr[:2000]: st = 'crash'
        err = r.stderr + r.stdout
    except subprocess.TimeoutExpired as e:
        st, err = 'hang', ''
    res = open(out, encoding='utf-8', errors='replace').read() if os.path.exists(out) else ''
    return res, err, st


def build_script(items):
    L = []
    for i, it in enumerate(items):
        L.append('/test E("@@%d@@")' % i)
        L.extend(it)
    L.append('/test E("@@END@@")')
    return '\n'.join(L)


def split_out(out, n):
    """-> (list of n lists of lines, complete?)"""
    segs = [[] for _ in range(n)]
    cur = None
    complete = False
    for ln in out.split('\n'):
        m = re.fullmatch(r'@@(\d+|END)@@', ln)
        if m:
            if m.group(1) == 'END': complete = True; cur = None; continue
            cur = int(m.group(1)); continue
        if cur is not None and cur < n:
            segs[cur].append(ln)
    # trailing empty from final newline
    for s in segs:
        while s and s[-1] == '' and False: s.pop()
    return segs, complete


def run_items(items, which, timeout=20):
    """Run items through 'tf' or 'gtf'. Hangs/crashes are bisected so that good items
    still produce results; the offending single item gets result 'HANG'/'CRASH'."""
    fn = run_tf if which == 'tf' else run_gtf
    n = len(items)
    if n == 0: return []
    out, extra, st = fn(build_script(items), timeout)
    segs, complete = split_out(out, n)
    if complete and st is None:
        return [tuple(s) for s in segs]
    # incomplete: bisect
    if n == 1:
        STATS[which + ('_timeouts' if st == 'hang' else '_crashes')] = STATS.get(which + ('_timeouts' if st == 'hang' else '_crashes'), 0) + 1
        return [('<%s>' % (st or 'INCOMPLETE').upper(),)]
    h = n // 2
    return run_items(items[:h], which, timeout) + run_items(items[h:], which, timeout)


def _par(items, which, timeout, chunk=6, workers=10):
    from concurrent.futures import ThreadPoolExecutor
    chunks = [items[i:i + chunk] for i in range(0, len(items), chunk)]
    with ThreadPoolExecutor(workers) as ex:
        res = list(ex.map(lambda c: run_items(c, which, timeout), chunks))
    return [r for c in res for r in c]


def compare(items, timeout=20):
    """-> (list of (index, tf_result, gtf_result) for diverging items, tf results, gtf results)"""
    a = _par(items, 'tf', timeout)
    b = _par(items, 'gtf', timeout)
    return [(i, a[i], b[i]) for i in range(len(items)) if a[i] != b[i]], a, b


def show(res):
    if len(res) == 0: return '<error/no output>'
    return ' | '.join(repr(x) for x in res)


def kind(ta, gb):
    if len(ta) == 0 and len(gb) > 0: return 'tf-err/gtf-value'
    if len(ta) > 0 and len(gb) == 0: return 'tf-value/gtf-err'
    return 'value-diff'
