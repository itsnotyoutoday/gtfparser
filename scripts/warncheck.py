#!/usr/bin/env python3
"""Compare gtfparser's warnings/errors with real tf's, case by case.

usage: warncheck.py [options] [KEY-or-prefix ...]

Cases live in testdata/warnings/*.tf; a case starts at a line `;;; <catalog-key> [opts]`
and runs until the next header. Each case is run as its own script (`tf -n -f case.tf`,
and `gtfrun -f case.tf`), line numbers are relative to the case. `@DIR@` in a case is
replaced by a scratch directory. Catalog: testdata/warnings/catalog.tsv.

Real tf is driven in a wide pty (like scripts/difftest.py); its screen is stripped of
ANSI, wrapped lines are joined, and the `% ...` lines are kept. Every line is split into
CONTEXT (the "file, line N: cmd: " location prefix) and BODY (the message, including a
"Warning: " marker). Bodies and contexts are compared separately:

  OK                 same context, same body
  body               body text differs
  context            context prefix differs
  body+context       both differ
  missing            real tf warned, gtfparser did not (or fewer lines)
  extra              gtfparser warned, real tf did not (or more lines)
  INVALID            real tf produced nothing matching the catalog template (fix the case)

Exit status is non-zero when any mismatch is not covered by testdata/warnings/known.tsv
(columns: key, class, reason; key `*` matches every case; class `*` every class).

Environment (see scripts/env.sh): TF_REF (real tf binary), TF_LIB (its lib/tf).
"""
import argparse, concurrent.futures as cf, fcntl, glob, json, os, pty, re, select
import shutil, struct, subprocess, sys, tempfile, termios, time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WDIR = os.path.join(ROOT, 'testdata', 'warnings')

# ---------------------------------------------------------------- catalog / cases

CAT_COLS = ['key', 'kind', 'file', 'line', 'func', 'template', 'context', 'trigger', 'status', 'case', 'note']

def unesc(s):
    return s.replace('\\t', '\t').replace('\\n', '\n').replace('\\\\', '\\')

def load_catalog(path=None):
    path = path or os.path.join(WDIR, 'catalog.tsv')
    cat = {}
    with open(path) as f:
        rows = [l.rstrip('\n') for l in f if l.strip()]
    hdr = rows[0].split('\t')
    assert hdr == CAT_COLS, 'bad catalog header: %r' % hdr
    for r in rows[1:]:
        p = r.split('\t')
        p += [''] * (len(CAT_COLS) - len(p))
        e = dict(zip(CAT_COLS, p))
        e['template'] = unesc(e['template'])
        cat[e['key']] = e
    return cat

def load_cases(paths=None):
    cases = []
    for path in paths or sorted(glob.glob(os.path.join(WDIR, '*.tf'))):
        cur = None
        for l in open(path).read().split('\n'):
            if l.startswith(';;; '):
                parts = l[4:].split()
                cur = {'key': parts[0], 'opts': parts[1:], 'lines': [], 'file': os.path.basename(path)}
                cases.append(cur)
            elif cur is not None:
                cur['lines'].append(l)
        for c in cases:
            while c['lines'] and not c['lines'][-1].strip(): c['lines'].pop()
    return cases

_FMT = re.compile(r'%[-+ #0]*(?:\d+|\*)?(?:\.(?:\d+|\*))?[lh]*([a-zA-Z%])')

def template_regex(tpl):
    """Regex (unanchored pieces joined) matching what a printf-style tf template can print."""
    out, i = '', 0
    for m in _FMT.finditer(tpl):
        out += re.escape(tpl[i:m.start()])
        c = m.group(1)
        out += '%' if c == '%' else ('' if c == 'A' else '.*?')
        i = m.end()
    out += re.escape(tpl[i:])
    return out

def entry_regex(e):
    r = template_regex(e['template'])
    if e['kind'] == 'W': r = 'Warning: ' + r
    return re.compile(r, re.S)

# ---------------------------------------------------------------- running real tf

NOISE = re.compile(r'^% (.*(Internal error at expr\.c:702, |: cmd: "[^"]*"$|internal error: reduce: bad op 0x0000)|features: no such function|/load: \S*local\.tf: no such file|LC_\w+ category set|Warning: Can\'t figure out name of mail file|Loading commands from \S*(stdlib|case)\.tf\.$|Visual mode|'
                   r'.*Redefined macro)')
ANSI = re.compile(r'\x1b\[[0-9;?]*[A-Za-z]|\x1b[()][A-Z0-9]|\x1b[=>]|\r')

def screen_lines(raw):
    t = re.sub(r'\x1b\[[0-9;?]*[A-Za-z]|\x1b[()][A-Z0-9]|\x1b[=>]', '', raw).replace('\r\n', '\n')
    t = t.replace('% Visual mode is not supported on this terminal.', '')
    out = []
    for l in t.split('\n'):
        segs = [x for x in l.split('\r') if x.strip()]   # a bare \r overwrites the line
        l = segs[-1].rstrip() if segs else ''
        if l.startswith('% '):
            out.append(l)
        elif out and l.startswith('    ') and l.strip():   # continuation of a wrapped message
            out[-1] += ' ' + l.strip()
    return out

def undouble(ls):
    """tf repaints its scrollback (at exit, and when input is typed): the screen so far is
    printed again. Drop every such verbatim copy of the lines that precede it."""
    ls = list(ls)
    while ls and ls[0] in ls[1:]:
        k = ls.index(ls[0], 1)
        if ls[k:2 * k] != ls[:k]: break
        ls = ls[:k] + ls[2 * k:]
    return ls

def run_tf(tf, lib, src, home, timeout, typed=None):
    """Run tf on a pty. With `typed` (list of lines) no script is loaded: the lines are typed at
    the prompt once the banner is up, which gives the "typed command" diagnostic context."""
    m, s = pty.openpty()
    fcntl.ioctl(s, termios.TIOCSWINSZ, struct.pack('HHHH', 50, 4000, 0, 0))
    env = {'TERM': 'dumb', 'PATH': os.environ['PATH'], 'HOME': home,
           'LANG': os.environ.get('LANG', ''), 'LC_ALL': os.environ.get('LC_ALL', '')}
    argv = [tf, '-n'] + ([] if typed is not None else ['-f' + src]) + ['-L' + lib]
    p = subprocess.Popen(argv, stdin=s, stdout=s, stderr=s, env=env, close_fds=True)
    os.close(s)
    buf = b''; t0 = time.time(); crashed = hung = False; sent = typed is None
    while p.poll() is None and time.time() - t0 < timeout:
        r, _, _ = select.select([m], [], [], 0.1)
        if r:
            try: buf += os.read(m, 65536)
            except OSError: break
            if b'Abnormal termination' in buf:
                crashed = True
                break
        if not sent and b"to quit tf" in buf and time.time() - t0 > 0.5:
            for l in typed + ['/quit']:
                os.write(m, (l + '\n').encode())
                time.sleep(0.15)
            sent = True
    if p.poll() is None:
        p.kill()
        hung = not crashed
    p.wait()
    os.close(m)
    return buf.decode(errors='replace'), crashed, hung

def run_gtf(gtf, lib, src, timeout, typed=False):
    try:
        r = subprocess.run([gtf, '-f', src, '-L', lib] + (['-i'] if typed else []), capture_output=True, text=True, timeout=timeout,
                           stdin=subprocess.DEVNULL)
    except subprocess.TimeoutExpired:
        return [], True
    ls = [l.rstrip() for l in r.stderr.split('\n') if l.startswith('% ')]
    if len(ls) > 50: ls = ls[:50] + ['%% ...(%d more lines)' % (len(ls) - 50)]
    return ls, False

# ---------------------------------------------------------------- splitting context / body

LOC = re.compile(r'^(?:(?:<FILE>|\S+\.tf), lines? \d+(?:-\d+)?: )')
CMD = re.compile(r'^([^\s:]+(?: -\w)?): (?!$)')

def split_line(line, e, ktype):
    """line without the leading '% ' -> (context, body).

    The context grammar is [<file>, line N: ][<command>[ -<opt>]: ]. The command part is
    optional, so "a: b: msg" is ambiguous; the catalog template decides: the longest context
    whose remainder still matches the template wins, else the shortest."""
    ctx = ''
    rest = line
    m = LOC.match(rest)
    if m: ctx, rest = m.group(0), rest[m.end():]
    if ktype in ('E', 'W', 'I'):
        rx = entry_regex(e) if e else None
        mm = CMD.match(rest) if not rest.startswith('Warning:') else None
        cuts = ([mm.end()] if mm else []) + [0]
        if rx:
            for c in cuts:
                if rx.fullmatch(rest[c:]):
                    return ctx + rest[:c], rest[c:]
        if mm:
            return ctx + rest[:mm.end()], rest[mm.end():]
    return ctx, rest

def parse_lines(ls, e, dirs):
    out = []
    for l in ls:
        for d, tag in dirs: l = l.replace(d, tag)
        body = l[2:]
        kind = e['kind'] if e else 'E'
        if kind in ('O', 'T', 'H'):
            out.append(('', body))
        else:
            out.append(split_line(body, e, kind))
    return out

# ---------------------------------------------------------------- one case

def run_case(c, cat, a):
    e = cat.get(c['key'])
    res = {'key': c['key'], 'file': c['file'], 'status': '', 'classes': [], 'tf': [], 'gtf': [], 'notes': []}
    d = os.path.realpath(tempfile.mkdtemp(prefix='warn_'))
    try:
        sd = os.path.join(d, 'sub'); os.mkdir(sd)
        body = '\n'.join(c['lines']).replace('@DIR@', sd) + '\n/quit\n'
        src = os.path.join(d, 'case.tf')
        with open(src, 'w') as f: f.write(body)
        raw, crashed, hung = run_tf(a.tf, a.lib, src, d, a.timeout, typed=c['lines'] if 'typed' in c['opts'] else None)
        tfl = [l for l in undouble(screen_lines(raw)) if not NOISE.match(l)]
        gl, ghung = run_gtf(a.gtf, a.lib, src, a.timeout, typed='typed' in c['opts'])
        gl = [l for l in gl if not NOISE.match(l)]
        dirs = [(d, '<DIR>'), (a.lib, '<LIB>')]
        # case.tf is shown as <FILE>
        tfp = [(x, y) for x, y in parse_lines(tfl, e, dirs)]
        gp = [(x, y) for x, y in parse_lines(gl, e, dirs)]
        tfp = [(x.replace('<DIR>/case.tf', '<FILE>'), y) for x, y in tfp]
        gp = [(x.replace('<DIR>/case.tf', '<FILE>'), y) for x, y in gp]
        if crashed: res['notes'].append('tf crashed')
        if hung: res['notes'].append('tf hung')
        if ghung: res['notes'].append('gtf hung')
        rx = entry_regex(e) if e else None
        if not e: valid = False
        elif e['kind'] in ('E', 'W', 'I'): valid = any(rx.fullmatch(b) for _, b in tfp)
        else: valid = any(rx.fullmatch(b) for _, b in tfp) or any(rx.fullmatch(x + b) for x, b in tfp)
        if a.probe: valid = bool(tfp)
        res['tf'] = [x + y for x, y in tfp]
        res['gtf'] = [x + y for x, y in gp]
        res['tf_split'] = tfp; res['gtf_split'] = gp
        classes = set()
        for i in range(max(len(tfp), len(gp))):
            if i >= len(gp): classes.add('missing'); continue
            if i >= len(tfp): classes.add('extra'); continue
            (c1, b1), (c2, b2) = tfp[i], gp[i]
            if b1 != b2: classes.add('body')
            if c1 != c2 and not a.ignore_context: classes.add('context')
        res['classes'] = sorted(classes)
        if not valid:
            res['status'] = 'INVALID'
        elif not classes:
            res['status'] = 'OK'
        else:
            res['status'] = '+'.join(x for x in ('body', 'context', 'missing', 'extra') if x in classes)
        res['valid'] = valid
    finally:
        shutil.rmtree(d, ignore_errors=True)
    return res

# ---------------------------------------------------------------- reporting

def load_causes():
    out = []
    path = os.path.join(WDIR, 'causes.tsv')
    if os.path.exists(path):
        for l in open(path):
            l = l.rstrip('\n')
            if not l.strip() or l.startswith('#'): continue
            p = l.split('\t') + ['', '']
            out.append((re.compile(p[0]), p[1], p[2]))
    return out

def cause_of(causes, key):
    for rx, cause, prio in causes:
        if rx.search(key): return cause, prio
    return 'other', '3'

def load_known(path):
    k = {}
    if os.path.exists(path):
        for l in open(path):
            l = l.rstrip('\n')
            if not l.strip() or l.startswith('#'): continue
            p = l.split('\t') + ['', '']
            k.setdefault(p[0], {})[p[1]] = p[2]
    return k

def is_known(known, key, cls):
    for kk in (key, '*'):
        for cc in (cls, '*'):
            if cc in known.get(kk, {}): return True
    return False

def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument('--tf', default=os.environ.get('TF_REF'))
    ap.add_argument('--lib', default=os.environ.get('TF_LIB'))
    ap.add_argument('--gtf', default='/tmp/warn_gtfrun', help='gtfrun binary (go build -o /tmp/warn_gtfrun ./cmd/gtfrun)')
    ap.add_argument('--ignore-context', action='store_true', help='compare message bodies only')
    ap.add_argument('--json'), ap.add_argument('--tsv')
    ap.add_argument('--known', default=os.path.join(WDIR, 'known.tsv'))
    ap.add_argument('--update-known', action='store_true', help='rewrite known.tsv from the current mismatches')
    ap.add_argument('--no-gate', action='store_true', help='always exit 0')
    ap.add_argument('--show-ok', action='store_true', help='also list OK cases in the table')
    ap.add_argument('--timeout', type=float, default=8)
    ap.add_argument('-j', type=int, default=8)
    ap.add_argument('--probe', metavar='FILE', help='run the cases of FILE without a catalog (authoring aid)')
    ap.add_argument('--list', action='store_true', help='list cases and exit')
    ap.add_argument('keys', nargs='*', help='run only cases whose key starts with one of these')
    a = ap.parse_args()
    if not a.tf or not a.lib: sys.exit('set TF_REF and TF_LIB (source scripts/env.sh)')
    a.lib = os.path.realpath(a.lib)
    cat = {} if a.probe else load_catalog()
    cases = load_cases([a.probe] if a.probe else None)
    seen = set()
    for c in cases:
        if c['key'] in seen: sys.exit('duplicate case key %s' % c['key'])
        seen.add(c['key'])
        if not a.probe and c['key'] not in cat: sys.exit('case %s has no catalog entry' % c['key'])
    if a.keys: cases = [c for c in cases if any(c['key'].startswith(k) for k in a.keys)]
    if a.list:
        for c in cases: print(c['key'], c['file'])
        return
    with cf.ThreadPoolExecutor(a.j) as ex:
        results = list(ex.map(lambda c: run_case(c, cat, a), cases))
    known = load_known(a.known)
    bad = []
    counts = {}
    for r in results:
        counts[r['status']] = counts.get(r['status'], 0) + 1
        cl = list(r['classes']) + (['invalid'] if r['status'] == 'INVALID' else [])
        r['unknown'] = [x for x in cl if not is_known(known, r['key'], x)]
        if r['unknown']: bad.append(r)
    # table
    for r in results:
        if r['status'] == 'OK' and not a.show_ok: continue
        note = ' [%s]' % ', '.join(r['notes']) if r['notes'] else ''
        flag = '' if not r['unknown'] else '  <-- NEW'
        print('%-14s %s%s%s' % (r['status'], r['key'], note, flag))
        print('    tf : %s' % (' | '.join(r['tf']) or '(nothing)'))
        print('    gtf: %s' % (' | '.join(r['gtf']) or '(nothing)'))
    cls_count = {k: sum(1 for r in results if k in r['classes']) for k in ('body', 'context', 'missing', 'extra')}
    print('\nsummary: %d cases: %s' % (len(results), ', '.join('%s=%d' % kv for kv in sorted(counts.items()))))
    print('mismatch classes (cases): ' + ', '.join('%s=%d' % kv for kv in cls_count.items()))
    crashes = [r['key'] for r in results if 'tf crashed' in r['notes']]
    if crashes: print('real tf crashed (message still compared) in: %s' % ' '.join(crashes))
    if a.json:
        with open(a.json, 'w') as f: json.dump(results, f, indent=1)
    if a.tsv:
        with open(a.tsv, 'w') as f:
            f.write('key\tstatus\tclasses\tnotes\ttf\tgtf\n')
            for r in results:
                f.write('%s\t%s\t%s\t%s\t%s\t%s\n' % (r['key'], r['status'], ','.join(r['classes']), ','.join(r['notes']),
                        ' || '.join(r['tf']).replace('\t', ' '), ' || '.join(r['gtf']).replace('\t', ' ')))
    if a.update_known:
        causes = load_causes()
        with open(a.known, 'w') as f:
            f.write('# key\tclass\treason   (regenerated by warncheck.py --update-known; reasons come from causes.tsv)\n')
            for r in results:
                cl = list(r['classes']) + (['invalid'] if r['status'] == 'INVALID' else [])
                for x in cl:
                    why = 'context prefix format (no file/line location, command-name style differs)' if x == 'context' else cause_of(causes, r['key'])[0]
                    f.write('%s\t%s\t%s\n' % (r['key'], x, why))
        print('wrote', a.known)
        return
    if bad:
        print('\n%d case(s) with mismatches not in %s:' % (len(bad), os.path.relpath(a.known)))
        for r in bad: print('  %s: %s' % (r['key'], ','.join(r['unknown'])))
        if not a.no_gate: sys.exit(1)
    else:
        print('no mismatches outside the allowlist')

if __name__ == '__main__':
    main()
