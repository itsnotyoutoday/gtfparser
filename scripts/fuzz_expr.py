#!/usr/bin/env python3
"""Grammar-based random expression fuzzer: real tf vs gtf.

usage: . scripts/env.sh; fuzz_expr.py [--seed N] [--count N] [--batch N] [--no-shrink] [--gtf PATH]
Each expression is run as `/test E(expr)` with a reset of the variables first; a marker
line before each makes "no output" (an error) detectable.  Divergent expressions are
shrunk (subtree replacement, batched) and grouped by category.
"""
import argparse, os, random, sys, collections, zlib
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fuzz_common as C

RESET = '/test x:=5, y:=-3, s:="hello", t:="Hello World", f:=1.5, z:=0, e:="", n:="12", d:=1:30, w:="a b  c"'
VARS = ['x', 'y', 's', 't', 'f', 'z', 'e', 'n', 'd', 'w', 'unsetv']

# precedence (higher binds tighter)
BIN = {'*': 11, '/': 11, '+': 10, '-': 10,
       '<': 9, '>': 9, '<=': 9, '>=': 9,
       '==': 8, '!=': 8, '=~': 8, '!~': 8, '=/': 8, '!/': 8,
       '&': 7, '|': 6}
UN_PREC, TERN_PREC, ASSIGN_PREC, COMMA_PREC = 12, 5, 4, 3
ASSIGN_OPS = [':=', '=', '+=', '-=', '*=', '/=']

# name -> list of arg-kind strings (s=string, i=int, n=number, p=pattern, a=any); variadic marked with '*'
FUNCS = {
    'strlen': 's', 'substr': 'sii?', 'strcat': 'a*', 'strstr': 'ss', 'strchr': 'ss', 'strrchr': 'ss',
    'strcmp': 'ss', 'strncmp': 'ssi', 'tolower': 's', 'toupper': 's', 'strrep': 'si', 'replace': 'sss',
    'abs': 'n', 'sqrt': 'n', 'pow': 'nn', 'mod': 'nn', 'trunc': 'n', 'ascii': 's', 'char': 'i',
    'pad': 'a*', 'rand': 'FIXED', 'regmatch': 'ps', 'ftime': 'FTIME', 'sin': 'n', 'cos': 'n', 'tan': 'n',
    'asin': 'n', 'acos': 'n', 'atan': 'n', 'exp': 'n', 'ln': 'n', 'log10': 'n', 'min': 'nn', 'max': 'nn',
    'strip_attr': 's', 'decode_attr': 's', 'encode_attr': 's', 'strlen2': 'DROP', 'isin': 'DROP',
    'tfopen': 'DROP', 'systype': 'DROP', 'nosuchfn': 'a',
    'eval': 'DROP', 'getenv': 'DROP', 'is_open': 'DROP',
}
FUNCS = {k: v for k, v in FUNCS.items() if v != 'DROP'}
FUNCS['nosuchfn'] = 'a'
FNAMES = [k for k in FUNCS if k != 'nosuchfn']

INTS = ['0', '1', '2', '3', '5', '7', '10', '12', '13', '100', '255', '256', '2147483647', '2147483648',
        '4294967296', '9223372036854775807', '9223372036854775808', '99999999999999999999', '007', '0x1f', '1000000']
DECS = ['1.5', '0.1', '3.14159', '.5', '5.', '1.1234567', '0.0', '2.5', '100.25', '1234567.5', '0.000001', '10.0', '1.0']
FLOATS = ['1e3', '1.5e-2', '1e20', '1E5', '2e0', '1e-7', '1.5e300', '1e308', '5e-324']
DTIMES = ['0:01:02', '1:30', '0:00:00.5', '12:34:56.789', '0:00:01', '1:00:00', '0:0:0']
STRS = ['abc', 'hello', 'Hello World', '', 'a b  c', '12', '1.5', '  7', '0x1f', '1e3', '-3', 'foo*', '*', 'a?c',
        'a.c', '^a', 'c$', '(b+)', 'l+', '[a-z]+', 'a|b', 'x', 'ABC', 'abcabc', '%', '$', '{', '}', '%%', '\\\\', '\\"',
        'a\\tb', '\\101', '\\x41', 'a\\\\b', "it's", 'say "hi"', 'ü', '日本語', '12abc', 'abc12', '+5', '--', '1:30', ' ']


def lit_str(rng):
    s = rng.choice(STRS)
    q = rng.choice('""""\'\'`')
    # escape unsafe quote chars
    body = s.replace('\\\\', '\x00')
    if q == '"': body = body.replace('"', '\\"') if '\\"' not in body else body
    elif q == "'": body = body.replace("'", "\\'")
    else: body = body.replace('`', '\\`')
    body = body.replace('\x00', '\\\\')
    if q == '"' and s.endswith('\\') and not s.endswith('\\\\'): body += '\\'
    return q + body + q


def atom(rng):
    r = rng.random()
    if r < .22: return ('lit', rng.choice(INTS))
    if r < .32: return ('lit', rng.choice(DECS))
    if r < .38: return ('lit', rng.choice(FLOATS))
    if r < .42: return ('lit', rng.choice(DTIMES))
    if r < .62: return ('lit', lit_str(rng))
    return ('var', rng.choice(VARS))


def gen(rng, depth, want=None):
    if depth <= 0 or rng.random() < .12:
        return atom(rng)
    r = rng.random()
    d = depth - 1
    if r < .38:
        op = rng.choice(list(BIN))
        return ('bin', op, gen(rng, d), gen(rng, d))
    if r < .46:
        return ('un', rng.choice(['-', '+', '!', '-', '!']), gen(rng, d))
    if r < .52:
        return ('tern', gen(rng, d), gen(rng, d), gen(rng, d))
    if r < .55:
        return ('elvis', gen(rng, d), gen(rng, d))
    if r < .59:
        return ('assign', rng.choice(['x', 'y', 's', 'f', 'z', 'n', 'u2']), rng.choice(ASSIGN_OPS), gen(rng, d))
    if r < .63:
        return ('incdec', rng.choice(['x', 'y', 'f', 'z', 's', 'n']), rng.choice(['++', '--']), rng.random() < .5)
    if r < .66:
        return ('comma', gen(rng, d), gen(rng, d))
    if r < .68:
        return ('paren', gen(rng, d))
    if r < .72:
        return ('brace', rng.choice(['x', 's', '1', '*', '#', 'f', 'L']))
    return gen_call(rng, d)


def gen_call(rng, d):
    f = rng.choice(FNAMES) if rng.random() < .97 else 'nosuchfn'
    spec = FUNCS[f]
    args = []
    if spec == 'FIXED':
        args = [('lit', rng.choice(['1', '1', '1', '0']))] if rng.random() < .8 else [('lit', '1'), ('lit', '1')]
    elif spec == 'FTIME':
        args = [('lit', '"%Y-%m-%d %H:%M:%S"'), ('lit', rng.choice(['0', '86400', '1000000000', '1700000000.5']))]
        if rng.random() < .3: args = [('lit', rng.choice(['"%H"', '"%a %b %d"', '"%j"', '"%%"']))] + args[1:]
    elif spec.endswith('*'):
        args = [gen(rng, d) for _ in range(rng.randint(0, 4))]
    else:
        opt = spec.endswith('?')
        n = len(spec) - (1 if opt else 0)
        k = n + (1 if opt and rng.random() < .5 else 0)
        if opt: k = n + rng.randint(0, 1)
        k = k if k - (1 if opt else 0) >= 0 else k
        for ch in spec.rstrip('?')[:k]:
            args.append(gen_arg(rng, ch, d))
        if rng.random() < .03 and args: args.pop()
        elif rng.random() < .03: args.append(atom(rng))
    return ('call', f, args)


def gen_arg(rng, ch, d):
    if rng.random() < .35:
        return gen(rng, d)
    if ch == 'i':
        return ('lit', rng.choice(['0', '1', '2', '3', '-1', '5', '100'])) if rng.random() < .8 else atom(rng)
    if ch == 'n':
        return atom(rng) if rng.random() < .5 else ('lit', rng.choice(INTS + DECS))
    if ch == 'p':
        return ('lit', '"' + rng.choice(['a', 'l+', '(b+)c', '^h', '[a-z]+', '(.)(.)', 'o$', 'x*', '(', '\\\\d+', '.']) + '"')
    return atom(rng)


def prec(n):
    t = n[0]
    if t == 'bin': return BIN[n[1]]
    if t == 'un': return UN_PREC
    if t in ('tern', 'elvis'): return TERN_PREC
    if t == 'assign': return ASSIGN_PREC
    if t == 'comma': return COMMA_PREC
    if t == 'incdec': return UN_PREC
    return 99


def render(n, rng=None):
    t = n[0]
    sp = ' '
    if t in ('lit', 'var'): return n[1]
    if t == 'brace': return '{' + n[1] + '}'
    if t == 'paren': return '(' + render(n[1]) + ')'
    if t == 'call': return n[1] + '(' + ', '.join(render(a) for a in n[2]) + ')'
    if t == 'incdec':
        return (n[2] + n[1]) if n[3] else (n[1] + n[2])
    if t == 'un':
        c = n[2]
        s = render(c)
        if prec(c) < UN_PREC: s = '(' + s + ')'
        if s[:1] in '+-' and n[1] in '+-': s = ' ' + s
        return n[1] + s
    if t == 'bin':
        p = BIN[n[1]]
        a, b = render(n[2]), render(n[3])
        if prec(n[2]) < p: a = '(' + a + ')'
        if prec(n[3]) <= p: b = '(' + b + ')'
        # tight spacing only when unambiguous
        tight = (zlib.crc32((n[1] + a + b).encode()) % 5 == 0) and a[-1:].isalnum() and b[:1].isalnum() and n[1] not in ('=~', '!~', '=/', '!/')
        return a + n[1] + b if tight else a + ' ' + n[1] + ' ' + b
    if t == 'tern':
        c, a, b = render(n[1]), render(n[2]), render(n[3])
        if prec(n[1]) <= TERN_PREC: c = '(' + c + ')'
        if prec(n[2]) < TERN_PREC: a = '(' + a + ')'
        if prec(n[3]) < TERN_PREC: b = '(' + b + ')'
        return c + ' ? ' + a + ' : ' + b
    if t == 'elvis':
        c, b = render(n[1]), render(n[2])
        if prec(n[1]) <= TERN_PREC: c = '(' + c + ')'
        if prec(n[2]) < TERN_PREC: b = '(' + b + ')'
        return c + ' ?: ' + b
    if t == 'assign':
        v = render(n[3])
        if prec(n[3]) < ASSIGN_PREC: v = '(' + v + ')'
        return n[1] + ' ' + n[2] + ' ' + v
    if t == 'comma':
        a, b = render(n[1]), render(n[2])
        if prec(n[1]) < COMMA_PREC: a = '(' + a + ')'
        if prec(n[2]) <= COMMA_PREC: b = '(' + b + ')'
        return a + ', ' + b
    raise ValueError(n)


def children(n):
    t = n[0]
    if t == 'bin': return [n[2], n[3]]
    if t == 'un': return [n[2]]
    if t == 'tern': return [n[1], n[2], n[3]]
    if t in ('elvis', 'comma'): return [n[1], n[2]]
    if t == 'assign': return [n[3]]
    if t == 'paren': return [n[1]]
    if t == 'call': return list(n[2])
    return []


def shrinks(n):
    """all one-step reductions of tree n"""
    out = []
    for c in children(n):
        out.append(c)
    for a in [('lit', '1'), ('lit', '0'), ('lit', '""'), ('lit', '"a"')]:
        if n != a and n[0] not in ('lit',): out.append(a)
    t = n[0]
    if t == 'bin':
        for i, c in ((2, n[2]), (3, n[3])):
            for s in shrinks(c): out.append(n[:i] + (s,) + n[i + 1:])
    elif t == 'un':
        for s in shrinks(n[2]): out.append((t, n[1], s))
    elif t == 'tern':
        for i in (1, 2, 3):
            for s in shrinks(n[i]): out.append(n[:i] + (s,) + n[i + 1:])
    elif t in ('elvis', 'comma'):
        for i in (1, 2):
            for s in shrinks(n[i]): out.append(n[:i] + (s,) + n[i + 1:])
    elif t == 'assign':
        for s in shrinks(n[3]): out.append(n[:3] + (s,))
    elif t == 'paren':
        for s in shrinks(n[1]): out.append((t, s))
    elif t == 'call':
        for i, c in enumerate(n[2]):
            for s in shrinks(c):
                out.append((t, n[1], n[2][:i] + [s] + n[2][i + 1:]))
            out.append((t, n[1], n[2][:i] + n[2][i + 1:]))
    elif t == 'lit':
        v = n[1]
        for r in ('1', '0', '""', '"a"'):
            if v != r and len(r) < len(v): out.append(('lit', r))
    # dedupe
    seen, res = set(), []
    for o in out:
        r = render(o)
        if r not in seen: seen.add(r); res.append(o)
    return res


def size(n): return len(render(n))


def item(expr):
    return [RESET, '/test E((%s))' % expr] if False else [RESET, '/test E(%s)' % expr]


def category(tree, ka):
    """coarse root-cause bucket: kind + sorted operator/function set"""
    feats = set()
    def walk(n):
        t = n[0]
        if t == 'bin': feats.add(n[1])
        elif t == 'call': feats.add(n[1] + '()')
        elif t in ('un',): feats.add('u' + n[1])
        elif t in ('tern', 'elvis', 'assign', 'comma', 'incdec', 'brace'): feats.add(t + (n[2] if t == 'assign' else ''))
        elif t == 'lit':
            v = n[1]
            feats.add('lit:' + ('str' if v[:1] in '"\'`' else 'dtime' if ':' in v else 'float' if 'e' in v.lower() and not v.startswith('0x') else 'dec' if '.' in v else 'int'))
        elif t == 'var': feats.add('var')
        for c in children(n): walk(c)
    walk(tree)
    feats = {f for f in feats if f not in ('lit:int',)}
    return ka + ' ' + ','.join(sorted(feats))


def mutate_text(rng, s):
    """syntax-noise: occasionally break the text to test error parity"""
    if not s: return s
    r = rng.random()
    i = rng.randrange(len(s))
    if r < .3: return s[:i] + s[i + 1:]
    if r < .6: return s[:i] + rng.choice(['(', ')', ',', '+', '?', ':', '"', '{', '}', '=', ' ', '!', '*', '&']) + s[i:]
    return s[:i] + rng.choice([')', '(', '"', ',']) + s[i + 1:]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--seed', type=int, default=1)
    ap.add_argument('--count', type=int, default=600)
    ap.add_argument('--batch', type=int, default=200)
    ap.add_argument('--depth', type=int, default=4)
    ap.add_argument('--noise', type=float, default=0.05)
    ap.add_argument('--no-shrink', action='store_true')
    ap.add_argument('--gtf')
    ap.add_argument('--timeout', type=int, default=30)
    ap.add_argument('--max-report', type=int, default=60)
    a = ap.parse_args()
    if a.gtf: C.GTF = a.gtf
    rng = random.Random(a.seed)
    trees = []
    while len(trees) < a.count:
        t = gen(rng, rng.randint(1, a.depth))
        trees.append(t)
    texts = []
    for t in trees:
        s = render(t)
        if rng.random() < a.noise: s = mutate_text(rng, s)
        texts.append(s)
    div = []  # (tree, text, ta, gb)
    ran = 0
    for b in range(0, len(texts), a.batch):
        chunk = texts[b:b + a.batch]
        items = [item(x) for x in chunk]
        d, ra, rb = C.compare(items, a.timeout)
        ran += len(chunk)
        for i, ta, gb in d:
            div.append((trees[b + i], chunk[i], ta, gb))
    print('expressions run: %d   diverging: %d' % (ran, len(div)))
    # group by category
    groups = collections.OrderedDict()
    for tree, text, ta, gb in div:
        k = category(tree, C.kind(ta, gb))
        groups.setdefault(k, []).append((tree, text, ta, gb))
    print('categories (pre-shrink): %d' % len(groups))
    # shrink one representative (smallest few) per category
    reps = []
    for k, lst in groups.items():
        lst.sort(key=lambda r: len(r[1]))
        reps.append((k, lst[0], len(lst)))
    if not a.no_shrink:
        cur = [r[1][0] for r in reps]
        curtext = [r[1][1] for r in reps]
        curres = [(r[1][2], r[1][3]) for r in reps]
        for rnd in range(8):
            cand = []  # (rep index, tree)
            for i, t in enumerate(cur):
                if t is None: continue
                ss = [s for s in shrinks(t) if size(s) < size(t)]
                ss.sort(key=size)
                for s in ss[:30]: cand.append((i, s))
            if not cand: break
            items = [item(render(s)) for _, s in cand]
            d, ra, rb = C.compare(items, a.timeout)
            dset = {i for i, _, _ in d}
            changed = False
            best = {}
            for ci, (ri, s) in enumerate(cand):
                if ci in dset:
                    # keep same divergence kind
                    if C.kind(ra[ci], rb[ci]) != C.kind(*curres[ri]): continue
                    if ri not in best or size(s) < size(best[ri][0]): best[ri] = (s, ci)
            for ri, (s, ci) in best.items():
                cur[ri] = s; curtext[ri] = render(s); curres[ri] = (ra[ci], rb[ci]); changed = True
            if not changed: break
        for i in range(len(reps)):
            k, r, n = reps[i]
            reps[i] = (k, (cur[i], curtext[i], curres[i][0], curres[i][1]), n)
    # dedupe reps by final text
    seen = set()
    n_cat = 0
    print()
    for k, (tree, text, ta, gb), n in sorted(reps, key=lambda r: len(r[1][1])):
        if text in seen: continue
        seen.add(text)
        n_cat += 1
        if n_cat > a.max_report: continue
        print('[%s] (x%d)' % (k, n))
        print('   expr: %s' % text)
        print('   tf  : %s' % C.show(ta))
        print('   gtf : %s' % C.show(gb))
    print('\ndiverging categories: %d' % n_cat)
    print('tf hangs/crashes: %s' % {k: v for k, v in C.STATS.items() if v})
    sys.exit(1 if div else 0)


if __name__ == '__main__':
    main()
