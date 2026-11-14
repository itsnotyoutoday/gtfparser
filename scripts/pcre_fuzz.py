#!/usr/bin/env python3
"""Random differential test: generate patterns/subjects from a small grammar,
run them through PCRE2 (scripts/pcre_ref.c) and the Go engine, report diffs.

usage: pcre_fuzz.py [-n COUNT] [--seed N] [--pcre2 DIR] [--keep FILE]
Not part of `go test`; use it after changing the engine.
"""
import argparse, os, random, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
GTF = os.path.dirname(HERE)

ATOMS = ['a', 'b', 'c', 'A', 'B', '.', r'\w', r'\W', r'\d', r'\s', '[ab]', '[^a]', '[a-c]', r'[\w-]', 'é', '日',
         r'\b', r'\B', '^', '$', r'\A', r'\z', r'\Z', r'\h', r'\R', r'\N', '[[:alpha:]]', r'\p{L}', r'\P{L}',
         r'\x61', '(?i:a)', 'ab', ' ', '-', '1', '2', r'\G', r'\X', r'\Qa.\E', '[^\\W\\d]', '[a-cÉ-é]', r'[\p{Lu}\d]',
         r'\p{Greek}', r'\v', r'\H', '[[:^alpha:]]', '[[:upper:]]', r'[\s\S]', 'É', 'ǆ', 'k', 'ß', r'\x{e9}', '(?x: a b )']
QUANT = ['', '', '', '*', '+', '?', '{2}', '{1,2}', '{0,1}', '*?', '+?', '??', '{1,}?', '*+', '++', '?+']
FLAGS = ['(?i)', '(?s)', '(?m)', '(?-i)', '(?x)', '(?i:', '(?-i:']
INVALID = [b'\xff', b'\xc3', b'\xe6\x97', b'\x80', b'\xf0\x9f\x98']
SUBJ = list('aabbccAB 1-\n') + ['é', '日', 'ab', 'abc', '\t', 'É', 'ǅ', '\u212a', 'ß', 'α', 'e\u0301', '\r\n', '\u2028', '_']


def gen(rng, depth=0):
    n = rng.randint(1, 4)
    seq = []
    for _ in range(n):
        r = rng.random()
        if r < 0.55 or depth > 2:
            a = rng.choice(ATOMS)
            seq.append(a + rng.choice(QUANT) if a not in ('^', '$', r'\b', r'\B', r'\A', r'\z', r'\Z') else a)
        elif r < 0.68:
            seq.append('(' + gen(rng, depth + 1) + ')' + rng.choice(QUANT))
        elif r < 0.75:
            seq.append('(?:' + gen(rng, depth + 1) + ')' + rng.choice(QUANT))
        elif r < 0.79:
            seq.append('(?>' + gen(rng, depth + 1) + ')' + rng.choice(QUANT))
        elif r < 0.84:
            seq.append(rng.choice(['(?=', '(?!', '(?<=', '(?<!']) + gen_lb(rng) + ')')
        elif r < 0.88:
            seq.append('(' + gen(rng, depth + 1) + '|' + gen(rng, depth + 1) + ')' + rng.choice(QUANT))
        elif r < 0.91:
            seq.append(rng.choice([r'\1', r'\2', r'\g{-1}', r'\1', r'\k<n>']))
        elif r < 0.94:
            seq.append('(?<n>' + gen(rng, depth + 1) + ')')
        elif r < 0.96:
            seq.append(r'\K')
        elif r < 0.975:
            seq.append('(?|(a)|(b))' + rng.choice(QUANT))
        elif r < 0.98:
            seq.append('(?(1)' + gen(rng, depth + 1) + '|' + gen(rng, depth + 1) + ')')
        else:
            f = rng.choice(FLAGS)
            seq.append(f + (gen(rng, depth + 1) + ')' if f.endswith(':') else ''))
    s = ''.join(seq)
    if rng.random() < 0.2:
        s += '|' + gen(rng, depth + 1)
    return s


MUT = list('()[]{}|*+?\\^$.-:<>=!#\'PkgQEKGRXNhHvVpPbBAzZ0123456789abcAB, i') + ['(?', '(?<', '\\x', '\\p{', '[:', ':]', '(?i)', '{1,2}', '\\1', '(?P<n>', '\\k<n>']


def mutate(rng, pat):
    for _ in range(rng.choice([1, 1, 2, 3])):
        i = rng.randint(0, len(pat))
        k = rng.random()
        if k < 0.4:
            pat = pat[:i] + rng.choice(MUT) + pat[i:]
        elif k < 0.7 and pat:
            i = min(i, len(pat) - 1)
            pat = pat[:i] + rng.choice(MUT) + pat[i + 1:]
        elif pat:
            i = min(i, len(pat) - 1)
            pat = pat[:i] + pat[i + 1:]
    return pat


def gen_lb(rng):
    return ''.join(rng.choice(['a', 'b', 'é', '.', r'\w', '[ab]', 'ab', r'\d', 'c']) + rng.choice(['', '', '', '{2}']) for _ in range(rng.randint(1, 2)))


def esc_subject(b):
    """bytes -> corpus escape syntax (every non-printable or non-ASCII byte as \\xHH)."""
    out = []
    for c in b:
        if c == 0x5c:
            out.append('\\\\')
        elif 0x20 <= c < 0x7f:
            out.append(chr(c))
        else:
            out.append('\\x%02x' % c)
    return ''.join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('-n', type=int, default=20000)
    ap.add_argument('--seed', type=int, default=1)
    ap.add_argument('--pcre2', default='/opt/homebrew/opt/pcre2')
    ap.add_argument('--keep')
    ap.add_argument('--show', type=int, default=40, help='number of diffs to print')
    ap.add_argument('--mutate', action='store_true', help='mutate corpus.txt patterns character-wise instead of using the grammar')
    ap.add_argument('--errors', action='store_true', help='also compare compile error offsets and messages')
    a = ap.parse_args()
    rng = random.Random(a.seed)
    d = tempfile.mkdtemp()
    ref, go = os.path.join(d, 'pcre_ref'), os.path.join(d, 'pcrerun')
    subprocess.check_call(['cc', '-O2', '-I' + a.pcre2 + '/include', '-L' + a.pcre2 + '/lib', '-lpcre2-8', '-o', ref, os.path.join(HERE, 'pcre_ref.c')])
    subprocess.check_call(['go', 'build', '-o', go, './pcre/internal/pcrerun/cmd/pcrerun'], cwd=GTF, env=dict(os.environ, GOWORK='off'))
    cases = []
    corpus = []
    if a.mutate:
        for line in open(os.path.join(GTF, 'pcre/testdata/corpus.txt'), encoding='utf-8').read().split('\n'):
            if line and not line.startswith('#'):
                corpus.append(line.split('\t', 1))
    while len(cases) < a.n:
        if a.mutate:
            pat, subj0 = rng.choice(corpus)
            pat = mutate(rng, pat)
        else:
            pat = gen(rng)
        if '\t' in pat or '\n' in pat:
            continue
        for _ in range(3):
            subj = b''.join(rng.choice(SUBJ).encode() if rng.random() > 0.08 else rng.choice(INVALID) for _ in range(rng.randint(0, 8)))
            if a.mutate and rng.random() < 0.5:
                subj = subj0.replace('\\n', '\n').replace('\\t', '\t')
                if '\\' in subj:
                    subj = 'ab'
                subj = subj.encode()

            bounds = [i for i in range(len(subj) + 1) if i == len(subj) or (subj[i] & 0xc0) != 0x80]
            start = rng.choice([0, 0, 0] + bounds)
            cases.append((pat, subj, start, rng.choice([0, 0, 0, 1]), rng.choice([0, 1])))
    path = a.keep or os.path.join(d, 'cases.txt')
    with open(path, 'w', encoding='utf-8') as f:
        for pat, subj, start, nb, ci in cases:
            f.write('%s\t%s\t%d\t%d\t%d\n' % (pat, esc_subject(subj), start, nb, ci))
    inp = []
    for pat, subj, start, nb, ci in cases:
        inp.append('\t'.join([pat, subj.hex(), str(start), str(nb), str(ci)]))
    w = subprocess.run([ref], input='\n'.join(inp) + '\n', capture_output=True, text=True, timeout=600).stdout.split('\n')[:-1]
    g = subprocess.run([go, '-ref', path], capture_output=True, text=True, timeout=600).stdout.split('\n')[:-1]
    if len(w) != len(cases) or len(g) != len(cases):
        sys.exit('line count mismatch: %d ref, %d go, %d cases' % (len(w), len(g), len(cases)))
    bad = 0
    stats = {'error': 0, 'nomatch': 0, 'match': 0}
    for c, x, y in zip(cases, w, g):
        stats['error' if x.startswith('ERR') and not x.startswith('ERROR') else 'nomatch' if x in ('0',) or x.startswith('ERROR') else 'match'] += 1
        if x.startswith('ERROR'):
            x = '0'
        if x.startswith('ERR') and y.startswith('ERR') and not a.errors:
            continue
        if x != y:
            bad += 1
            if bad <= a.show:
                print('DIFF pattern=%r subject=%r start=%d notbol=%d caseless=%d\n   pcre2: %s\n   go   : %s' % (c[0], c[1], c[2], c[3], c[4], x, y))
    print('%d/%d agree (pcre2: %d matches, %d no match, %d compile errors)' % (len(cases) - bad, len(cases), stats['match'], stats['nomatch'], stats['error']))
    sys.exit(1 if bad else 0)


main()
