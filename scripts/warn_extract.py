#!/usr/bin/env python3
"""Extract every diagnostic-emitting call from the tf C source into a raw TSV.

usage: warn_extract.py TF_SRC_DIR > raw.tsv
Columns: file line func kind template
kind: E eprintf/operror, W tf_wprintf ("Warning: " is added), O oprintf("% ..."),
      T tfprintf(tferr, ...), I internal_error.
The curated catalog (testdata/warnings/catalog.tsv) was seeded from this output.
"""
import glob, os, re, sys

CALLS = re.compile(r'\b(eprintf|tf_wprintf|tfpywprintf|operror|internal_error2?|oprintf|tfprintf|tfputs|parse_error|parse_error_suggest|do_hook)\s*\(')

def balanced(s, i):
    depth = 0; j = i; instr = None
    while j < len(s):
        c = s[j]
        if instr:
            if c == '\\': j += 1
            elif c == instr: instr = None
        elif c in '"\'': instr = c
        elif c == '(': depth += 1
        elif c == ')':
            depth -= 1
            if depth == 0: return s[i+1:j]
        j += 1
    return None

def split_args(a):
    out = []; depth = 0; cur = ''; instr = None; i = 0
    while i < len(a):
        c = a[i]
        if instr:
            cur += c
            if c == '\\': cur += a[i+1]; i += 1
            elif c == instr: instr = None
        elif c in '"\'': instr = c; cur += c
        elif c in '([': depth += 1; cur += c
        elif c in ')]': depth -= 1; cur += c
        elif c == ',' and depth == 0: out.append(cur.strip()); cur = ''
        else: cur += c
        i += 1
    out.append(cur.strip())
    return out

def literal(arg):
    parts = re.findall(r'"((?:[^"\\]|\\.)*)"', arg)
    return ''.join(parts) if parts else None

def main(src):
    rows = []
    for path in sorted(glob.glob(os.path.join(src, '*.c'))):
        text = open(path, errors='replace').read()
        lines = text.split('\n')
        # function starts: col-0 identifier followed by '(' (return type on previous line)
        func_at = []
        for n, l in enumerate(lines, 1):
            m = re.match(r'^(?:[A-Za-z_][\w \*]*?\b)?(\w+)\s*\(', l)
            if m and not l.startswith(('#', '}', ' ', '\t', 'if', 'for', 'while', 'switch', 'return', 'static const', 'typedef')) and not l.rstrip().endswith(';'):
                func_at.append((n, m.group(1)))
        fname = os.path.basename(path)
        for m in CALLS.finditer(text):
            line = text.count('\n', 0, m.start()) + 1
            # skip definitions / comments
            ls = lines[line-1]
            if re.match(r'^(void|static|int)\b', ls) or ls.lstrip().startswith(('*', '/*', '//')): continue
            args = balanced(text, m.end()-1)
            if args is None: continue
            a = split_args(args)
            if m.group(1) in ('parse_error', 'parse_error_suggest'):
                if ls.startswith(('void', 'static')) or len(a) < 3: continue
                ty, ex = literal(a[1]), literal(a[2])
                if ty is None or ex is None:
                    ty = ty or '%s'; ex = ex or '%s'
                tpl = '%s syntax error: expected %s, found %%s.' % (ty, ex)
                if m.group(1) == 'parse_error_suggest': tpl += '  (' + (literal(a[3]) or '%s') + ')'
                fn = ''
                for n, nm in func_at:
                    if n <= line: fn = nm
                rows.append((fname, line, fn, 'E', tpl, ''))
                continue
            if m.group(1) == 'do_hook':
                if len(a) < 3 or not a[1].startswith('"') and 'fmt' not in a[1].lower(): continue
                t = literal(a[1])
                if t is None or t == '': continue
                kind = 'E' if t.startswith('!') else 'H'
                t = t[1:] if t.startswith('!') else t
                if kind == 'H' and not t.startswith('%%'): continue
                fn = ''
                for n, nm in func_at:
                    if n <= line: fn = nm
                rows.append((fname, line, fn, kind, t, a[0]))
                continue
            kind = {'eprintf':'E','operror':'E','tf_wprintf':'W','tfpywprintf':'W','oprintf':'O','tfprintf':'T','tfputs':'T'}.get(m.group(1), 'I')
            if m.group(1).startswith('internal_error'): kind = 'I'
            if kind == 'T':
                if not a or a[0] != 'tferr': continue
                a = a[1:]
            if m.group(1) == 'tfputs':
                continue
            if m.group(1) == 'operror':
                tpl = literal(a[0]) or ('<' + a[0] + '>')
                tpl = tpl + ': %s'  # strerror
                if literal(a[0]) is None: tpl = '%s: <strerror>'
            else:
                if m.group(1).startswith('internal_error'): a = a[2:] if m.group(1)=='internal_error' else a[4:]
                if not a: continue
                tpl = literal(a[0])
                if tpl is None: tpl = '<' + a[0] + '>' + ' (' + ', '.join(a[1:]) + ')'
            if kind == 'O' and not tpl.startswith('%%'): continue
            fn = ''
            for n, nm in func_at:
                if n <= line: fn = nm
            tpl = tpl.replace('\t', '\\t')
            rows.append((fname, line, fn, kind, tpl, ' '.join(', '.join(a[1:]).split())))
    for r in rows: print('\t'.join(map(str, r)).replace('\n', ' '))
main(sys.argv[1])
