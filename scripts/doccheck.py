#!/usr/bin/env python3
"""Check the examples in the documentation.

An example is a ```tf fenced block immediately followed by an ```output block.
The tf block is run through gtfrun in interactive mode (each line is run as if
typed with full substitution; /def and /load lines load as from a script
file) and what it echoes must equal the output block exactly (trailing
whitespace ignored). Blocks tagged ```tf-noverify and ```tf without an output
block are documentation only and are not run.

usage: scripts/doccheck.py [--gtfrun PATH] [--lib DIR] [files or dirs...]   (default: docs README.md)
"""
import argparse, os, re, subprocess, sys, tempfile

FENCE = re.compile(r'^```(\S*)\s*$')

def blocks(text):
    lines, i, out = text.split('\n'), 0, []
    while i < len(lines):
        m = FENCE.match(lines[i])
        if m:
            tag, start = m.group(1), i + 1
            j = start
            while j < len(lines) and not lines[j].startswith('```'):
                j += 1
            out.append((tag, '\n'.join(lines[start:j]), start))
            i = j + 1
        else:
            i += 1
    return out

def run(gtfrun, lib, code):
    with tempfile.NamedTemporaryFile('w', suffix='.tf', delete=False) as f:
        f.write(code + '\n'); path = f.name
    try:
        cmd = [gtfrun, '-i', '-f', path] + (['-L', lib] if lib else ['-n'])
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        return r.stdout
    finally:
        os.unlink(path)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--gtfrun', default=os.environ.get('GTFRUN', '/tmp/gtfrun'))
    ap.add_argument('--lib', default=None, help='tf library dir; loads stdlib.tf first (default: none)')
    ap.add_argument('paths', nargs='*', default=['docs', 'README.md'])
    a = ap.parse_args()
    files = []
    for p in a.paths:
        if os.path.isdir(p):
            for d, _, fs in os.walk(p):
                files += [os.path.join(d, f) for f in sorted(fs) if f.endswith('.md')]
        else:
            files.append(p)
    bad = total = 0
    for f in sorted(files):
        bs = blocks(open(f).read())
        for k, (tag, code, line) in enumerate(bs):
            if tag != 'tf' or k + 1 >= len(bs) or bs[k + 1][0] != 'output':
                continue
            total += 1
            want = '\n'.join(l.rstrip() for l in bs[k + 1][1].rstrip().split('\n'))
            got = '\n'.join(l.rstrip() for l in run(a.gtfrun, a.lib, code).rstrip().split('\n'))
            if got != want:
                bad += 1
                print(f'FAIL {f}:{line}\n--- code\n{code}\n--- want\n{want}\n--- got\n{got}\n')
    print(f'{total - bad}/{total} documented examples pass')
    sys.exit(1 if bad else 0)

main()
