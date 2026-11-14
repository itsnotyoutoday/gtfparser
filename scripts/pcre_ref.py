#!/usr/bin/env python3
"""Generate pcre/testdata/ref_expected.txt from pcre/testdata/ref_cases.txt
using real PCRE2 (scripts/pcre_ref.c) with TinyFugue's options.  These cases
cover what tf scripts cannot express: start offsets, NOTBOL, \\G, lookbehind
before the start, and invalid UTF-8 subjects (PCRE2_MATCH_INVALID_UTF).

ref_cases.txt: pattern TAB subject TAB start TAB notbol TAB caseless
(subject escapes as in corpus.txt, but \\xHH are raw bytes and may be invalid UTF-8).

usage: pcre_ref.py [--pcre2 DIR]
"""
import argparse, os, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
TD = os.path.join(HERE, '..', 'pcre', 'testdata')


def unescape(s):
    out, i = bytearray(), 0
    m = {'n': 10, 't': 9, 'r': 13, 'f': 12, 'v': 11, 'e': 27, '\\': 92}
    while i < len(s):
        if s[i] != '\\':
            out += s[i].encode('utf-8'); i += 1; continue
        c = s[i + 1]
        if c == 'x':
            out.append(int(s[i + 2:i + 4], 16)); i += 4
        elif c == 'u':
            out += chr(int(s[i + 2:i + 6], 16)).encode('utf-8'); i += 6
        else:
            out.append(m[c]); i += 2
    return bytes(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--pcre2', default='/opt/homebrew/opt/pcre2')
    a = ap.parse_args()
    exe = os.path.join(tempfile.mkdtemp(), 'pcre_ref')
    subprocess.check_call(['cc', '-O2', '-I' + a.pcre2 + '/include', '-L' + a.pcre2 + '/lib', '-lpcre2-8',
                           '-o', exe, os.path.join(HERE, 'pcre_ref.c')])
    lines = [l for l in open(os.path.join(TD, 'ref_cases.txt'), encoding='utf-8').read().split('\n')
             if l and not l.startswith('#')]
    inp = []
    for l in lines:
        pat, subj, start, notbol, caseless = l.split('\t')
        inp.append('\t'.join([pat, unescape(subj).hex(), start, notbol, caseless]))
    out = subprocess.run([exe], input='\n'.join(inp) + '\n', capture_output=True, text=True, check=True).stdout.split('\n')[:-1]
    if len(out) != len(lines):
        sys.exit('pcre_ref returned %d lines for %d cases' % (len(out), len(lines)))
    open(os.path.join(TD, 'ref_expected.txt'), 'w', encoding='utf-8').write('\n'.join(out) + '\n')
    print('wrote %d expectations' % len(out))


main()
