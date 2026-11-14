#!/usr/bin/env python3
"""Random macro-program fuzzer: real tf vs gtf.

usage: . scripts/env.sh; fuzz_macro.py [--seed N] [--count N] [--batch N] [--no-shrink] [--gtf PATH]

Each item defines macros m1 (the entry point) and m2 (a callee that sees m1's locals through
dynamic scoping), sets some globals, then invokes m1 several ways: `/m1 args`, as a function
`m1(a, b)`, through $() in a wrapper (w1), after /shift etc.  All output (the macros' /echo
and the values) goes to the capture file.  Failing programs are shrunk statement by statement
and grouped by the set of constructs they use.
"""
import argparse, os, random, sys, collections
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fuzz_common as C

WORDS = ['foo', 'bar', 'a b', 'x', 'Q', '12', '3.5', '"q r"', "it's", '(', ')', '*', '#', ';', 'a;b', '-n', '=', '\\', 'ü', '[1]', '{', '}']
SUBST = ['%1', '%2', '%3', '%{1}', '%{2}', '%{3}', '%*', '%{*}', '%{#}', '%#', '%-1', '%{-1}', '%{-2}', '%{-3}', '%L', '%{L}',
         '%{L1}', '%{L2}', '%{-L1}', '%{-L}', '%{1-dflt}', '%{3-d f}', '%{nosuch-fb}', '%{v}', '%v', '%{w-W}', '%a', '%{a}', '%{i}',
         '%%', '%%1', '$$', '$${v}', '\\\\', '\\n', '\\t', '\\101', '\\$', '\\%', '$[1+2]', '$[{1}]', '$[strlen({*})]',
         '$[{#} > 1 ? "many" : "few"]', '$[a]', '$[v . 1]', '$[strcat(v, {1})]', '$(/m2 %1)', '$(/m2 %*)', '$(/first %*)', '$(/rest %*)',
         '$(/last %*)', '$(/nth 2 %*)', '$(/car %*)', '$(/length %*)', '${v}', '${w}', '%?', '%0', '%{0}', '%P1', '%{P1}', '%PL', '%PR', '%P0',
         '%{PL}', '%%{x}', '%{1}%{2}', '%1%2', '%1x', '%{1}x', '%1-%2']
CONDS = ['{1} == "a"', '{#} > 1', '{#}', '!{#}', 'v =~ "x"', 'v !~ ""', 'a', 'i < 3', 'strlen({*}) > 2', '{1} =/ "a*"', 'regmatch("a(.)", {*})',
         '{1} > 2', '{1} =~ "foo"', 'a == 1', 'a > v', '{2} !~ ""', '1', '0', '{-1} =~ ""', '{#} == 2 & {1} !~ ""', '{1} | {2}', 'strstr({*}, "b") >= 0']
ARGS = ['', 'a', 'a b c', '"q r" s', "'x y' z", '1 2 3', 'a  b', '-x foo', '%1 %2', '$[1+1] $(/echo hi)', 'foo;bar', '  lead trail  ', '3', '0', 'a "b c" d',
        '\\n \\\\ x', '{1} {*}', '100 200', 'foo bar baz qux', '(x)', 'a=b', '5 apples', 'x*y']
FNARGS = ['', '"a"', '"a b", 2', '1, 2, 3', '"x", "", "z"', 'v, a', '"%1", "$$"', '3', '"a", "b", "c", "d"', '"", ""']
EXPRS = ['{1}', '{#}', '{1} + 1', 'strcat({1}, {2})', 'v', 'a + 1', '{*}', '"s" . 1', 'strlen({*})', '{L}', '{-1}', 'a ? "y" : "n"', '0', '1', '$[1]', '{1} * 2', '{#} - 1']


def rt(rng, n=None):
    """text = list of tokens"""
    n = n if n is not None else rng.randint(1, 4)
    return [rng.choice(SUBST) if rng.random() < .55 else rng.choice(WORDS) for _ in range(n)]


def rtext(t): return ' '.join(t)


def stmt(rng, d, inloop=False, depth_calls=True):
    r = rng.random()
    if r < .22: return ('echo', rt(rng))
    if r < .30: return ('let', rng.choice('abi'), rt(rng, rng.randint(1, 2)))
    if r < .35: return ('set', rng.choice(['v', 'w', 'g']), rt(rng, rng.randint(1, 2)))
    if r < .40: return ('test', rng.choice(['a := {1}', '++a', 'v := strcat(v, {1})', 'i := {#}', 'a += 2', 'echo(strcat("t:", {*}))',
                                              'regmatch("(a)(.)", {*})', 'regmatch("(.)", {1}), echo({P0}, {P1})', 'a := regmatch("o+", {*}) ? {PL} : "none"', '--i']))
    if r < .43: return ('result', rng.choice(['return', 'result']), rng.choice(EXPRS + ['', 'v . "x"']))
    if r < .48: return ('shift', rng.choice(['', '', '1', '2', '0', '3']))
    if r < .53: return ('eval', rt(rng, rng.randint(1, 3)), rng.choice(['/echo', '/echo', '/let a=', '/shift', '/test']))
    if r < .62 and depth_calls: return ('call', rng.choice(['/m2', '/m2', '/first', '/rest', '/last']), rt(rng, rng.randint(0, 3)))
    if r < .66 and depth_calls: return ('callv', rng.choice(['a', 'b']), rng.choice(['m2', 'first', 'rest', 'last', 'car', 'cdr', 'length']), rt(rng, rng.randint(0, 2)))
    if r < .68: return ('qecho',)  # /echo %?
    if d <= 0: return ('echo', rt(rng))
    if r < .80:
        e = [('if', rng.choice(CONDS), [stmt(rng, d - 1, inloop) for _ in range(rng.randint(1, 2))])]
        for _ in range(rng.randint(0, 2)):
            e.append(('elseif', rng.choice(CONDS), [stmt(rng, d - 1, inloop) for _ in range(rng.randint(1, 2))]))
        if rng.random() < .6:
            e.append(('else', None, [stmt(rng, d - 1, inloop) for _ in range(rng.randint(1, 2))]))
        return ('ifchain', e)
    if r < .85:
        return ('iflist', rng.choice(['/@test {1} > 0', '/@test {#}', '/test a', '/@test v =~ "x"', '/m2 %1', '/@test regmatch("a", {*})']), [stmt(rng, d - 1, inloop) for _ in range(rng.randint(1, 2))],
                [stmt(rng, d - 1, inloop) for _ in range(rng.randint(0, 1))])
    if r < .92:
        return ('while', rng.choice(['i < 3', 'i < 2', '({#} > 0) & (i < 3)', 'shift(), {#}', 'a < 3']), [stmt(rng, d - 1, True) for _ in range(rng.randint(1, 2))], rng.random() < .3)
    if r < .96:
        return ('for', rng.choice(['k', 'i']), rng.choice(['1', '0']), rng.choice(['2', '3', '{#}']), rt(rng, 1))
    if inloop: return ('break', rng.choice(CONDS))
    return ('echo', rt(rng))


def render_stmts(ss):
    return '%;'.join(render_stmt(s) for s in ss)


def render_stmt(s):
    k = s[0]
    if k == 'echo': return '/echo ' + rtext(s[1])
    if k == 'let': return '/let %s=%s' % (s[1], rtext(s[2]))
    if k == 'set': return '/set %s=%s' % (s[1], rtext(s[2]))
    if k == 'test': return '/test ' + s[1]
    if k == 'result': return '/%s %s' % (s[1], s[2])
    if k == 'shift': return '/shift' + (' ' + s[1] if s[1] else '')
    if k == 'eval': return '/eval %s %s' % (s[2], rtext(s[1]).replace('%', '%%') if hash0(s) else rtext(s[1]))
    if k == 'call': return '%s %s' % (s[1], rtext(s[2]))
    if k == 'callv': return '/let %s=$(%s %s)%%;/echo %%%s' % (s[1], '/' + s[2], rtext(s[3]), s[1])
    if k == 'qecho': return '/echo [%?]'
    if k == 'ifchain':
        out = ''
        for e in s[1]:
            if e[0] == 'if': out += '/if (%s) %s' % (e[1], render_stmts(e[2]))
            elif e[0] == 'elseif': out += '%%;/elseif (%s) %s' % (e[1], render_stmts(e[2]))
            else: out += '%%;/else %s' % render_stmts(e[2])
        return out + '%;/endif'
    if k == 'iflist':
        out = '/if %s%%; /then %s' % (s[1], render_stmts(s[2]))
        if s[3]: out += '%%;/else %s' % render_stmts(s[3])
        return out + '%;/endif'
    if k == 'while':
        c = s[1]
        v = 'a' if c.startswith('a <') else 'i'
        pre = '' if 'shift' in c else '/let %s=0%%;' % v
        inc = '' if 'shift' in c else '%%;/test ++%s' % v
        return '%s/while (%s) %s%s%%;/done' % (pre, c, render_stmts(s[2]), inc)
    if k == 'for': return '/for %s %s %s %s' % (s[1], s[2], s[3], '/echo ' + rtext(s[4]) + ' %' + s[1])
    if k == 'break': return '/if (%s) /break%%;/endif' % s[1]
    raise ValueError(k)


def hash0(s): return len(rtext(s[1])) % 2 == 0


def gen_prog(rng):
    p = {}
    p['globals'] = [('v', rng.choice(['x', 'hello', '', 'a b', '3'])), ('w', rng.choice(['W', '', 'w w'])), ('g', rng.choice(['G', '7']))]
    p['m1'] = [stmt(rng, rng.randint(1, 3)) for _ in range(rng.randint(1, 5))]
    p['m2'] = [stmt(rng, 1, depth_calls=False) for _ in range(rng.randint(1, 3))]
    calls = []
    for _ in range(rng.randint(1, 3)):
        r = rng.random()
        if r < .5: calls.append(('cmd', rng.choice(ARGS)))
        elif r < .75: calls.append(('fn', rng.choice(FNARGS)))
        else: calls.append(('sub', rng.choice(ARGS)))
    p['calls'] = calls
    return p


def render_prog(p):
    L = ['/set %s=%s' % (k, v) for k, v in p['globals']]
    L.append('/def -i m1 = ' + render_stmts(p['m1']))
    L.append('/def -i m2 = ' + render_stmts(p['m2']))
    L.append('/def -i w1 = /echo <$(/m1 %*)>')
    for c in p['calls']:
        if c[0] == 'cmd': L.append('/m1 ' + c[1])
        elif c[0] == 'fn': L.append('/test E(m1(%s))' % c[1])
        else: L.append('/w1 ' + c[1])
    return L


# --- shrinking ---
def shrink_list(ss):
    out = []
    for i in range(len(ss)):
        out.append(ss[:i] + ss[i + 1:])
    for i, s in enumerate(ss):
        for v in shrink_stmt(s):
            out.append(ss[:i] + [v] + ss[i + 1:])
        # hoist block bodies
        for body in blocks_of(s):
            out.append(ss[:i] + body + ss[i + 1:])
    return out


def blocks_of(s):
    if s[0] == 'ifchain': return [e[2] for e in s[1]]
    if s[0] == 'iflist': return [s[2], s[3]]
    if s[0] == 'while': return [s[2]]
    return []


def shrink_stmt(s):
    out = []
    k = s[0]
    if k in ('echo',):
        for t in shrink_text(s[1]): out.append((k, t))
    elif k in ('let', 'set'):
        for t in shrink_text(s[2]): out.append((k, s[1], t))
    elif k == 'eval':
        for t in shrink_text(s[1]): out.append((k, t, s[2]))
    elif k == 'call':
        for t in shrink_text(s[2], 0): out.append((k, s[1], t))
    elif k == 'callv':
        for t in shrink_text(s[3], 0): out.append(s[:3] + (t,))
    elif k == 'ifchain':
        for j, e in enumerate(s[1]):
            for b in shrink_list(e[2]):
                if b: out.append(('ifchain', s[1][:j] + [(e[0], e[1], b)] + s[1][j + 1:]))
            if j > 0: out.append(('ifchain', s[1][:j] + s[1][j + 1:]))
    elif k == 'iflist':
        for b in shrink_list(s[2]):
            if b: out.append(('iflist', s[1], b, s[3]))
        if s[3]: out.append(('iflist', s[1], s[2], []))
    elif k == 'while':
        for b in shrink_list(s[2]):
            if b: out.append(('while', s[1], b, s[3]))
    return out


def shrink_text(t, minlen=1):
    out = []
    for i in range(len(t)):
        if len(t) > minlen: out.append(t[:i] + t[i + 1:])
        if t[i] != 'x': out.append(t[:i] + ['x'] + t[i + 1:])
    return out


def shrink_prog(p):
    out = []
    for i in range(len(p['calls'])):
        if len(p['calls']) > 1: out.append(dict(p, calls=p['calls'][:i] + p['calls'][i + 1:]))
        c = p['calls'][i]
        for alt in (('cmd', ''), ('cmd', 'a'), ('cmd', 'a b'), ('fn', ''), ('fn', '"a"')):
            if alt != c: out.append(dict(p, calls=p['calls'][:i] + [alt] + p['calls'][i + 1:]))
    for key in ('m1', 'm2'):
        for l in shrink_list(p[key]):
            if l or key == 'm2': out.append(dict(p, **{key: l}))
    for i in range(len(p['globals'])):
        out.append(dict(p, globals=p['globals'][:i] + p['globals'][i + 1:]))
    return out


def psize(p): return len('\n'.join(render_prog(p)))


def features(p):
    txt = '\n'.join(render_prog(p))
    f = set()
    import re
    for m in re.findall(r'%\{?[-A-Za-z0-9#*?]+(?:-[^}]*)?\}?|\$\[|\$\(|\$\$|\\\\|%%|/(?:if|elseif|else|while|break|return|result|shift|eval|test|for|let|set|first|rest|last|nth|car|cdr|length)\b|\b(?:m1|m2)\(', txt):
        f.add(re.sub(r'\d+', 'N', m) if m.startswith('%') else m)
    for m in re.findall(r'(?<=\n)/(?:test E\(m1|w1)', txt): f.add(m)
    return f


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--seed', type=int, default=1)
    ap.add_argument('--count', type=int, default=300)
    ap.add_argument('--batch', type=int, default=60)
    ap.add_argument('--no-shrink', action='store_true')
    ap.add_argument('--gtf')
    ap.add_argument('--timeout', type=int, default=30)
    ap.add_argument('--max-report', type=int, default=40)
    a = ap.parse_args()
    if a.gtf: C.GTF = a.gtf
    rng = random.Random(a.seed)
    progs = [gen_prog(rng) for _ in range(a.count)]
    div = []
    for b in range(0, len(progs), a.batch):
        chunk = progs[b:b + a.batch]
        d, ra, rb = C.compare([render_prog(p) for p in chunk], a.timeout)
        for i, ta, gb in d: div.append((chunk[i], ta, gb))
    print('programs run: %d   diverging: %d' % (len(progs), len(div)))
    cur = [list(x) for x in div]
    if not a.no_shrink and cur:
        # shrink each divergence independently but batched; keep the divergence 'kind'
        for rnd in range(25):
            cand = []
            for i, (p, ta, gb) in enumerate(cur):
                ss = [s for s in shrink_prog(p) if psize(s) < psize(p)]
                ss.sort(key=psize)
                for s in ss[:45]: cand.append((i, s))
            if not cand: break
            d, ra, rb = C.compare([render_prog(s) for _, s in cand], a.timeout)
            dset = {i for i, _, _ in d}
            best = {}
            for ci, (ri, s) in enumerate(cand):
                if ci in dset and C.kind(ra[ci], rb[ci]) == C.kind(cur[ri][1], cur[ri][2]):
                    if ri not in best or psize(s) < psize(best[ri][0]): best[ri] = (s, ci)
            if not best: break
            for ri, (s, ci) in best.items(): cur[ri] = [s, ra[ci], rb[ci]]
    seen, groups = set(), collections.OrderedDict()
    for p, ta, gb in cur:
        txt = '\n'.join(render_prog(p))
        if txt in seen: continue
        seen.add(txt)
        groups.setdefault(C.kind(ta, gb) + ' ' + ','.join(sorted(features(p))), []).append((p, ta, gb))
    n = 0
    for k, lst in sorted(groups.items(), key=lambda kv: psize(kv[1][0][0])):
        n += 1
        if n > a.max_report: continue
        p, ta, gb = lst[0]
        print('\n[%s] (x%d)' % (k, len(lst)))
        for l in render_prog(p): print('   | ' + l)
        print('   tf  :', C.show(ta))
        print('   gtf :', C.show(gb))
    print('\ndiverging categories: %d' % n)
    print('hangs/crashes: %s' % {k: v for k, v in C.STATS.items() if v})
    sys.exit(1 if div else 0)


if __name__ == '__main__':
    main()
