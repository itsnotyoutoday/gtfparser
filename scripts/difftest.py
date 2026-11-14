#!/usr/bin/env python3
"""Differential test: run TF case files through real tf and through gtf, diff the echoed output.

usage: difftest.py [--tf PATH] [--lib DIR] [--gtf PATH] case.tf ...
A case file is a TF script; its /echo output is compared. Real tf is driven in
a pty with /echo redirected into a file (a macro named echo shadows the builtin).
"""
import argparse, os, pty, re, select, subprocess, sys, tempfile, time
SCREEN = []  # raw tty output of the last real-tf run (its warnings go here)

def run_tf(tf, lib, script, timeout=20):
    d = tempfile.mkdtemp()
    out, src = os.path.join(d, 'out'), os.path.join(d, 'case.tf')
    with open(src, 'w') as f:
        f.write('/test _o := tfopen("%s", "w")\n' % out)
        f.write('/def -i echo = /test tfwrite(_o, {*})\n/def -i E = /test tfwrite(_o, {1})\n')
        f.write(script)
        f.write('\n/test tfclose(_o)\n/quit\n')
    SCREEN.clear()
    m, s = pty.openpty()
    import fcntl, struct, termios
    fcntl.ioctl(s, termios.TIOCSWINSZ, struct.pack('HHHH', 50, 4000, 0, 0))  # wide: no line wrapping
    p = subprocess.Popen([tf, '-n', '-f' + src, '-L' + lib], stdin=s, stdout=s, stderr=s,
                         env={'TERM': 'dumb', 'PATH': os.environ['PATH'], 'HOME': d, 'LANG': os.environ.get('LANG', ''), 'LC_ALL': os.environ.get('LC_ALL', '')}, close_fds=True)
    t = time.time()
    while p.poll() is None and time.time() - t < timeout:
        r, _, _ = select.select([m], [], [], 0.2)
        if r:
            try: SCREEN.append(os.read(m, 65536))
            except OSError: break
    if p.poll() is None: p.kill()
    return open(out).read() if os.path.exists(out) else '<no output>'

def run_gtf(gtf, lib, script):
    d = tempfile.mkdtemp()
    out, src = os.path.join(d, 'out'), os.path.join(d, 'case.tf')
    with open(src, 'w') as f:
        f.write('/test _o := tfopen("%s", "w")\n' % out)
        f.write('/def -i echo = /test tfwrite(_o, {*})\n/def -i E = /test tfwrite(_o, {1})\n')
        f.write(script)
        f.write('\n/test tfclose(_o)\n')
    subprocess.run([gtf, '-f', src, '-L', lib], capture_output=True, text=True, timeout=20)
    return open(out).read() if os.path.exists(out) else '<no output>'

ap = argparse.ArgumentParser()
ap.add_argument('--tf', default=os.environ.get('TF_REF'))
ap.add_argument('--lib', default=os.environ.get('TF_LIB'))
ap.add_argument('--gtf', default='/tmp/gtfrun')
ap.add_argument('--show', action='store_true', help="print real tf's warnings")
ap.add_argument('cases', nargs='+')
a = ap.parse_args()
bad = 0
for c in a.cases:
    script = open(c).read()
    want, got = run_tf(a.tf, a.lib, script), run_gtf(a.gtf, a.lib, script)
    if a.show:
        t = re.sub(r'\x1b\[[0-9;?]*[A-Za-z]|\x1b[()][A-Z0-9]|\r', '', b''.join(SCREEN).decode(errors='replace'))
        for l in re.findall(r'^% .*(?:\n    .*)*', t, re.M):
            if 'Visual mode' not in l and 'Redefined macro echo' not in l and 'Loading' not in l and 'LC_' not in l and 'mail file' not in l: print('  tf>', ' '.join(l.split()))
    if want == got:
        print('ok  ', c)
    else:
        bad += 1
        print('DIFF', c)
        import difflib
        for l in difflib.unified_diff(want.splitlines(), got.splitlines(), 'tf', 'gtf', lineterm='', n=0): print('   ', l)
sys.exit(1 if bad else 0)
