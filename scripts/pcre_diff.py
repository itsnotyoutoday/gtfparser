#!/usr/bin/env python3
"""Differential test of the pcre package against real TinyFugue (PCRE2).

usage: pcre_diff.py [--tf PATH] [--lib DIR] [--update] [--show-ok] [corpus]

corpus (default pcre/testdata/corpus.txt) has one "pattern<TAB>subject" per
line; see pcre/internal/pcrerun for the subject escapes.  All pairs go through
ONE real tf run: each becomes `regmatch(pattern, subject)` and the result
(n, PL, PR, P0..P9) is written to a file with tfwrite.  The same corpus is
run through the Go engine (CompileTF) and the records are compared.

--update rewrites pcre/testdata/expected.txt from real tf's answers (the
file `go test ./pcre` checks the engine against).

The tf binary must be a WIDECHAR build (ICU), because TinyFugue only compiles
patterns with PCRE2_UTF|PCRE2_UCP then; a non-widechar tf matches bytes and
will disagree on all Unicode cases.  scripts/pcre_build_tfwide.sh makes one
from the reference tree that env.sh points at.  tf runs with LANG/LC_ALL set to
en_US.UTF-8 (its locale-derived PCRE2 character tables matter: for example
[[:ascii:]] covers U+0000-U+00FF in a UTF-8 locale).
"""
import argparse, os, pty, re, select, subprocess, sys, tempfile, time

GTF = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PRE = 3  # script lines run_tf writes before the case body


def tf_string(s):
    """TF expression for a literal string; control characters via char()."""
    parts, cur = [], ''
    for ch in s:
        if ord(ch) < 32 or ord(ch) == 127:
            if cur:
                parts.append('"%s"' % cur)
                cur = ''
            parts.append('char(%d)' % ord(ch))
        elif ch == '\\':
            cur += '\\\\'
        elif ch == '"':
            cur += '\\"'
        else:
            cur += ch
    if cur or not parts:
        parts.append('"%s"' % cur)
    return parts[0] if len(parts) == 1 else 'strcat(%s)' % ', '.join(parts)


def unescape(s):
    out, i = bytearray(), 0
    m = {'n': '\n', 't': '\t', 'r': '\r', 'f': '\f', 'v': '\v', 'e': '\x1b', '\\': '\\'}
    while i < len(s):
        if s[i] != '\\':
            out += s[i].encode('utf-8'); i += 1; continue
        c = s[i + 1]
        if c == 'x':
            out.append(int(s[i + 2:i + 4], 16)); i += 4
        elif c == 'u':
            out += chr(int(s[i + 2:i + 6], 16)).encode('utf-8'); i += 6
        else:
            out += m[c].encode(); i += 2
    return out.decode('utf-8')


def esc(s):
    return s.replace('\\', '\\\\').replace('\n', '\\n').replace('\t', '\\t').replace('\r', '\\r')


def read_corpus(path):
    pairs = []
    for line in open(path, encoding='utf-8').read().split('\n'):
        if not line or line.startswith('#'):
            continue
        pat, subj = line.split('\t', 1)
        pairs.append((pat, unescape(subj)))
    return pairs


def run_tf(tf, lib, script, timeout=180):
    d = tempfile.mkdtemp()
    out, src = os.path.join(d, 'out'), os.path.join(d, 'case.tf')
    with open(src, 'w', encoding='utf-8') as f:
        f.write('/test _o := tfopen("%s", "w")\n' % out)
        f.write('/def -i echo = /test tfwrite(_o, {*})\n/def -i E = /test tfwrite(_o, {1})\n')
        f.write(script)
        f.write('\n/test tfclose(_o)\n/quit\n')
    screen = []
    m, s = pty.openpty()
    p = subprocess.Popen([tf, '-n', '-f' + src, '-L' + lib], stdin=s, stdout=s, stderr=s,
                         env={'TERM': 'dumb', 'PATH': os.environ['PATH'], 'HOME': d, 'LANG': 'en_US.UTF-8', 'LC_ALL': 'en_US.UTF-8'}, close_fds=True)
    t = time.time()
    while p.poll() is None and time.time() - t < timeout:
        r, _, _ = select.select([m], [], [], 0.2)
        if r:
            try:
                screen.append(os.read(m, 65536))
            except OSError:
                break
    if p.poll() is None:
        p.kill()
    text = open(out, encoding='utf-8', errors='replace', newline='').read() if os.path.exists(out) else ''
    return text, b''.join(screen).decode(errors='replace')


def tf_records(tf, lib, pairs):
    lines = []
    for i, (pat, subj) in enumerate(pairs):
        lines.append('/test n := regmatch(%s, %s)' % (tf_string(pat), tf_string(subj)))
        lines.append('/test tfwrite(_o, strcat("@@", %d, " ", n))' % i)
        lines.append('/test tfwrite(_o, strcat("@L=", {PL}))')
        lines.append('/test tfwrite(_o, strcat("@R=", {PR}))')
        for k in range(10):
            lines.append('/test tfwrite(_o, strcat("@%d=", {P%d}))' % (k, k))
    # line number (in the final file) of each pair's regmatch line
    lineno = {PRE + 1 + 14 * i: i for i in range(len(pairs))}
    text, screen = run_tf(tf, lib, '\n'.join(lines))
    # compile errors -> scraped from the tty
    flat = re.sub(r'\x1b\[[0-9;?]*[A-Za-z]|\x1b[()][A-Z0-9]|\r', '', screen)
    flat = re.sub(r'\s+', ' ', flat)
    errs = {}
    for mm in re.finditer(r'line (\d+): regmatch: regexp error: character (\d+): (.*?)(?= % |$)', flat):
        ln = int(mm.group(1))
        if ln in lineno:
            errs[lineno[ln]] = (int(mm.group(2)), mm.group(3).strip())
    res = {}
    cur = None
    for line in text.split('\n'):
        m = re.match(r'@@(\d+) (\S*)$', line)
        if m:
            cur = {'n': m.group(2), 'f': {}, 'last': None}
            res[int(m.group(1))] = cur
            continue
        m = re.match(r'@([LR0-9])=(.*)$', line)
        if m and cur is not None:
            cur['last'] = m.group(1)
            cur['f'][cur['last']] = m.group(2)
            continue
        if cur is not None and cur['last'] is not None:
            cur['f'][cur['last']] += '\n' + line
    recs = []
    for i in range(len(pairs)):
        if i in errs:
            recs.append('ERR\t%d\t%s' % errs[i]); continue
        r = res.get(i)
        if r is None:
            recs.append('MISSING'); continue
        try:
            n = int(r['n'])
        except ValueError:
            recs.append('BADN:' + r['n']); continue
        if n <= 0:
            recs.append('0'); continue
        f = [str(n), esc(r['f'].get('L', '')), esc(r['f'].get('R', ''))]
        for k in range(min(n, 10)):
            f.append(esc(r['f'].get(str(k), '')))
        recs.append('\t'.join(f))
    return recs, errs


def go_records(corpus):
    exe = os.path.join(tempfile.mkdtemp(), 'pcrerun')
    env = dict(os.environ, GOWORK='off')
    subprocess.check_call(['go', 'build', '-o', exe, './pcre/internal/pcrerun/cmd/pcrerun'], cwd=GTF, env=env)
    out = subprocess.run([exe, corpus], capture_output=True, text=True, check=True).stdout
    return out.split('\n')[:-1]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--tf', default=os.environ.get('TF_REF'))
    ap.add_argument('--lib', default=os.environ.get('TF_LIB'))
    ap.add_argument('--update', action='store_true', help='write pcre/testdata/expected.txt from real tf')
    ap.add_argument('--show-ok', action='store_true')
    ap.add_argument('--errors', action='store_true', help='print real tf compile error messages')
    ap.add_argument('corpus', nargs='?', default=os.path.join(GTF, 'pcre/testdata/corpus.txt'))
    a = ap.parse_args()
    if not a.tf or not a.lib:
        sys.exit('set TF_REF/TF_LIB (. scripts/env.sh)')
    pairs = read_corpus(a.corpus)
    want, errs = tf_records(a.tf, a.lib, pairs)
    got = go_records(a.corpus)
    bad = 0
    for i, ((pat, subj), w, g) in enumerate(zip(pairs, want, got)):
        if a.errors and i in errs:
            print('tf error #%d %r: char %d: %s' % (i, pat, errs[i][0], errs[i][1]))
        if w == g:
            if a.show_ok:
                print('ok   #%d %r %r' % (i, pat, subj))
            continue
        bad += 1
        print('DIFF #%d pattern=%r subject=%r\n   tf : %r\n   go : %r' % (i, pat, subj, w, g))
    print('%d/%d pairs agree with real tf' % (len(pairs) - bad, len(pairs)))
    if a.update:
        if any(w.startswith(('MISSING', 'BADN')) for w in want):
            sys.exit('refusing to write expected.txt: tf run incomplete')
        path = os.path.join(os.path.dirname(a.corpus), 'expected.txt')
        open(path, 'w', encoding='utf-8').write('\n'.join(want) + '\n')
        print('wrote', path)
    sys.exit(1 if bad else 0)


main()
