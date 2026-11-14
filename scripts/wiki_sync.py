#!/usr/bin/env python3
"""Publish docs/ to the GitHub Wiki.

The wiki is generated, never edited by hand: docs/ in the repository is the
source of truth (it is versioned with the code and its examples are verified
by scripts/doccheck.py). GitHub wikis are flat, so subfolders are flattened
(docs/language/X.md -> Language-X, docs/reference/X.md -> Reference-X,
docs/engine/X.md -> Engine-X), relative links are rewritten, and _Sidebar.md
and _Footer.md are generated.

usage:
  scripts/wiki_sync.py --out DIR            write the wiki pages into DIR (dry run)
  scripts/wiki_sync.py --push               clone the wiki repo, replace its pages, commit and push
"""
import argparse, os, posixpath, re, shutil, subprocess, sys, tempfile

REPO = 'itsnotyoutoday/gtfparser'
BLOB = f'https://github.com/{REPO}/blob/main/'
GUIDE_ORDER = ['Home', 'Getting-Started', 'Embedding-Guide', 'Extending-the-Engine', 'Use-Cases',
               'Safety-and-Sandboxing', 'Concurrency-and-Performance', 'Testing-and-Compatibility',
               'Deliberate-Differences', 'Diagnostics', 'Architecture', 'FAQ']
SECTIONS = [('language', 'Language', 'Language guide'), ('reference', 'Reference', 'Built-in reference'),
            ('engine', 'Engine', 'Engine notes')]

def wiki_name(rel):
    """docs-relative path (no .md) -> flat wiki page name."""
    parts = rel.split('/')
    for folder, prefix, _ in SECTIONS:
        if parts[0] == folder:
            if len(parts) == 2 and parts[1] == 'README':
                return {'language': 'Language-Guide', 'reference': 'Built-in-Reference', 'engine': 'Engine-Notes'}[folder]
            tail = ['Known-Differences' if x == 'FINDINGS' else x for x in parts[1:]]
            return prefix + '-' + '-'.join(tail)
    return '-'.join(w[:1].upper() + w[1:] for w in '-'.join(parts).split('-'))

def collect(docs):
    pages = {}
    for d, _, fs in os.walk(docs):
        for f in fs:
            if f.endswith('.md'):
                rel = os.path.relpath(os.path.join(d, f), docs)[:-3].replace(os.sep, '/')
                pages[rel] = os.path.join(d, f)
    return pages

LINK = re.compile(r'(\]\()([^)\s]+)(\))')

def rewrite(text, rel, pages):
    here = posixpath.dirname(rel)
    def sub(m):
        url = m.group(2)
        if re.match(r'^[a-z][a-z0-9+.-]*:', url) or url.startswith('#') or url.startswith('mailto:'):
            return m.group(0)
        path, _, anchor = url.partition('#')
        target = posixpath.normpath(posixpath.join(here, path)) if path else rel
        key = target[:-3] if target.endswith('.md') else target
        if key in pages:
            return m.group(1) + wiki_name(key) + ('#' + anchor if anchor else '') + m.group(3)
        if posixpath.isabs(target) or target.startswith('..'):
            repo_path = posixpath.normpath(posixpath.join('docs', here, path))
            return m.group(1) + BLOB + repo_path + m.group(3)
        # a link to something in docs/ that is not a page (a file): point at the repo
        return m.group(1) + BLOB + 'docs/' + target + m.group(3)
    return LINK.sub(sub, text)

def sidebar(pages):
    lines = ['**[gtfparser](Home)**', '']
    names = {rel: wiki_name(rel) for rel in pages}
    lines.append('**Guides**')
    for g in GUIDE_ORDER:
        if g in pages and g != 'Home':
            lines.append(f'- [{g.replace("-", " ")}]({g})')
    for folder, prefix, title in SECTIONS:
        rels = sorted(r for r in pages if r.startswith(folder + '/'))
        if not rels:
            continue
        lines += ['', f'**{title}**']
        idx = [r for r in rels if r.endswith('/README')]
        for r in idx:
            lines.append(f'- [{title} index]({names[r]})')
        for r in rels:
            if r.endswith('/README'):
                continue
            label = r.split('/', 1)[1].replace('FINDINGS', 'Known differences').replace('-', ' ')
            lines.append(f'- [{label}]({names[r]})')
    lines += ['', '[Source on GitHub](https://github.com/' + REPO + ')']
    return '\n'.join(lines) + '\n'

FOOTER = ('_This wiki is generated from the `docs/` folder of the repository ([browse it](https://github.com/'
          + REPO + '/tree/main/docs)). Edit the docs there; changes made here are overwritten._\n')

def build(docs, out):
    pages = collect(docs)
    if os.path.isdir(out):
        for f in os.listdir(out):
            if f != '.git':
                p = os.path.join(out, f)
                shutil.rmtree(p) if os.path.isdir(p) else os.remove(p)
    os.makedirs(out, exist_ok=True)
    for rel, path in pages.items():
        text = open(path).read()
        text = rewrite(text, rel, pages)
        open(os.path.join(out, wiki_name(rel) + '.md'), 'w').write(text)
    open(os.path.join(out, '_Sidebar.md'), 'w').write(sidebar(pages))
    open(os.path.join(out, '_Footer.md'), 'w').write(FOOTER)
    return len(pages)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--docs', default=os.path.join(os.path.dirname(__file__), '..', 'docs'))
    ap.add_argument('--out')
    ap.add_argument('--push', action='store_true')
    a = ap.parse_args()
    docs = os.path.abspath(a.docs)
    if a.out:
        n = build(docs, os.path.abspath(a.out))
        print(f'{n} pages written to {a.out}')
    if a.push:
        tmp = tempfile.mkdtemp()
        wiki = os.path.join(tmp, 'wiki')
        url = f'git@github.com:{REPO}.wiki.git'
        r = subprocess.run(['git', 'clone', url, wiki], capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit('cannot clone the wiki repo (enable Wikis on the repository and create the first page once in the web UI):\n' + r.stderr)
        n = build(docs, wiki)
        git = lambda *c: subprocess.run(['git', '-C', wiki] + list(c), check=True)
        git('add', '-A')
        if subprocess.run(['git', '-C', wiki, 'diff', '--cached', '--quiet']).returncode == 0:
            print('wiki already up to date'); return
        git('-c', 'user.name=gtf', '-c', 'user.email=dev@localhost', 'commit', '-m', 'Sync wiki from docs/')
        git('push', 'origin', 'HEAD')
        print(f'{n} pages published')

main()
