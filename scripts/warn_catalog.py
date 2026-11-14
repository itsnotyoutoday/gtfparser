#!/usr/bin/env python3
"""Regenerate testdata/warnings/catalog.tsv.

usage: warn_catalog.py TF_SRC_DIR            (writes testdata/warnings/catalog.tsv)

Inputs: the diagnostics extracted from the tf C source (scripts/warn_extract.py), the case
files testdata/warnings/*.tf (a header `;;; key [opts] # description` makes an entry
testable and supplies its `trigger` text), and the tables below (entries that cannot be
triggered headlessly, hand-added templates, context overrides).

Catalog columns (tab separated, \\t and \\n escaped, one header row):
  key      <source-stem>.<line> (suffix .b/.c when one line has several calls)
  kind     E error (eprintf / %s: strerror / do_hook "!..." / parse_error), W warning (tf_wprintf,
           "Warning: " is prepended), O oprintf("% ..."), T tfprintf(tferr, ...), H do_hook
           default text, I internal_error
  file,line,func   where it is raised in tf's C source
  template the printf-style message as written in the C source (after the context prefix);
           %s %S %d %c ... are arguments, %% is a literal percent
  context  which prefix real tf puts in front:
             eprefix   "% [<file>, line N: ][<command>[ -<opt>]: ]" then (for W) "Warning: " then message
             none      no prefix; the line is "% " plus the message (oprintf / hook default text)
             custom    the call builds its own "% ..." text (tfprintf to tferr)
  trigger  how a script provokes it (from the case header), or empty
  status   testable | untestable
  case     the case key in testdata/warnings/*.tf (== key) when testable
  note     why untestable / remarks
"""
import os, re, subprocess, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WDIR = os.path.join(ROOT, 'testdata', 'warnings')
HERE = os.path.dirname(os.path.abspath(__file__))
COLS = ['key', 'kind', 'file', 'line', 'func', 'template', 'context', 'trigger', 'status', 'case', 'note']

# Raw rows that are not real diagnostics (definitions, helpers whose text is built elsewhere).
DROP = {
    'expand.1912', 'expand.1919',      # parse_error()/parse_error_suggest() bodies: call sites are listed
    'tfio.751', 'util.477.b',          # tfpywprintf wrapper, comment text
    'tfio.763', 'tfio.764', 'tfio.776', 'tfio.778',  # internal_error plumbing (the I rows stand for it)
    'dstring.80', 'dstring.84',
}

# Rows the extractor cannot see (format string built dynamically or via a macro).
ADD = [
    # key, kind, file, line, func, template, note
    ('command.580', 'H', 'command.c', 580, 'do_file_load', '%% Loading commands from %s.', 'only when not quiet (/load without -q)'),
    ('expr.2015.b', 'E', 'expr.c', 2015, 'unary_expr', '%s: found %d arguments, expected between %d and %d', ''),
    ('ctx.file-line', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe: <file>, line N: '),
    ('ctx.macro', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe: macro name after the location'),
    ('ctx.macro-nested', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe: macro called from macro'),
    ('ctx.multiline', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe: "lines N-M: " for continued lines'),
    ('ctx.eval', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.trigger', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.hook', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.load-nested', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe: inner file named'),
    ('ctx.option-suffix', 'E', '(context)', 0, 'ctx', 'invalid option', 'context-rule probe: " -c" option suffix'),
    ('ctx.function-name', 'E', '(context)', 0, 'ctx', 'real value too large to convert to integer', 'context-rule probe: function name as command'),
    ('ctx.cmdsub', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.builtin-in-macro', 'E', '(context)', 0, 'ctx', 'Macro "%s" was not defined.', 'context-rule probe'),
    ('ctx.shadowed-builtin', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.nonexistent-command', 'E', '(context)', 0, 'ctx', '%s: no such command or macro', 'context-rule probe (via hook NOMACRO)'),
    ('ctx.nonexistent-in-macro', 'E', '(context)', 0, 'ctx', '%s: no such command or macro', 'context-rule probe'),
    ('ctx.load-error', 'E', '(context)', 0, 'ctx', '%s: %s', 'context-rule probe (hook LOADFAIL text)'),
    ('ctx.load-implicit-dir', 'E', '(context)', 0, 'ctx', '%s: %s', 'context-rule probe'),
    ('ctx.eval-macro', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.if-condition', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.while-body', 'E', '(context)', 0, 'ctx', '%s: no such function', 'context-rule probe'),
    ('ctx.typed', 'E', '(context)', 0, 'ctx', '%s: no such function', 'typed at the prompt: no file location'),
    ('ctx.typed-macro', 'E', '(context)', 0, 'ctx', '%s: no such function', 'typed at the prompt: macro name only'),
    ('ctx.typed-builtin', 'E', '(context)', 0, 'ctx', 'Macro "%s" was not defined.', 'typed at the prompt'),
    ('ctx.typed-unknown', 'E', '(context)', 0, 'ctx', '%s: no such command or macro', 'typed at the prompt'),
    ('ctx.typed-option', 'E', '(context)', 0, 'ctx', 'invalid option', 'typed at the prompt: option suffix'),
    ('output.1504.b', 'E', 'output.c', 1504, 'find_statusfield_by_template', '%s: no such field in row %d', ''),
]
# Rows whose extracted template is wrong/ambiguous.
FIX = {
    'expr.2015': '%s: found %d arguments, expected %d',
    'output.1504': '%s: no such field',
    'socket.553': '%% Unable to connect to %s: %s %s: %s',
}

# Why an entry cannot be provoked from a headless script: (key-regex, reason). First match wins.
UNTESTABLE = [
    (r'^(dstring|malloc)\.', 'internal consistency check / memory allocation'),
    (r'^lua\.', 'lua support is not compiled into the reference tf build (no /calllua, /loadlua, /purgelua)'),
    (r'^tfpython\.', 'python support is not compiled in (and gated by tfpy_debug)'),
    (r'.*', None),
]
REASONS = {
    'command.283': '/sh waits for a keypress on the terminal ("Press any key to continue tf.")',
    'command.292': 'interactive /sh (no argument) needs a terminal',
    'command.357': 'getwd() fallback, only compiled when getcwd() is missing',
    'command.417': 'only reachable in squat sandbox mode (scripts uploaded by a remote client)',
    'command.558': 'needs an invalid TFPATH environment variable (the harness fixes the environment)',
    'command.566': 'needs an invalid TFLIBDIR environment variable (the harness fixes the environment)',
    'command.597': 'needs a user interrupt (^C) while a file loads',
    'command.642': 'only in "tinytalk" mode (world-definition lines before the first "/" line of an old-style file); a -f script never enters it',
    'expand.243': 'builtin commands disabled at compile time; this build has none',
    'expand.401': 'no script found that reaches this production',
    'expand.466': 'macro-echo tracing (/set mecho) did not print anything headless',
    'expand.598': 'macro-echo tracing; compiled out (#if 0)',
    'expand.763': 'macro-echo tracing (/set mecho) did not print anything headless',
    'expand.1029': 'macro-echo tracing (/set mecho) did not print anything headless',
    'expand.506': 'needs a user interrupt (^C) during macro execution',
    'expand.776': 'no script found that reaches this warning',
    'expand.925': 'internal expression-stack consistency check',
    'expand.1387': 'no script found that reaches this production',
    'expand.1466': 'no script found that reaches this production',
    'expand.1641': 'no script found that reaches this production',
    'expr.345': 'dead code (#if 0)',
    'expr.634': 'impossible result type (internal consistency check)',
    'expr.669': 'needs a variable holding the most negative integer; integer literals that large parse as reals',
    'expr.677': 'needs a variable holding the largest integer; integer literals that large parse as reals',
    'expr.1705': 'unimplemented function opcode (internal)',
    'expr.1739': 'needs libm to set errno (EDOM); macOS libm does not, glibc does',
    'expr.1743': 'needs libm to set errno (ERANGE); macOS libm does not, glibc does',
    'expr.1355': 'world_info() reached no error path with an unknown field in any script tried',
    'help.47': 'help file missing (installation dependent)',
    'help.56': 'help file unreadable (installation dependent)',
    'history.355': 'no input reaches this parsenumber failure ("-x" is taken as an option)',
    'history.447': 'needs a user interrupt (^C)',
    'history.618': 'needs text arriving from a world (socket)',
    'keyboard.499': 'echo of keyboard input (/set kecho); needs interactive input',
    'macro.723': 'needs an invisible /key_ macro (internal key macros)',
    'macro.807': 'rebinding of key macros; no script found that reaches it',
    'macro.810': 'rebinding of key macros; no script found that reaches it',
    'macro.950': 'the "Redefined macro" message is not printed in this build for ordinary redefinitions',
    'macro.1773': 'no script found that reaches the unnamed-macro listing branch',
    'macro.1957': 'macro-echo tracing (/set mecho) did not print anything headless',
    'output.531': 'terminal setup at startup (TERMCAP); the harness fixes the environment',
    'output.536': 'terminal setup at startup (TERMCAP); the harness fixes the environment',
    'output.546': 'terminal setup at startup (TERM undefined); the harness fixes the environment',
    'output.548': 'terminal setup at startup (unsupported TERM); the harness fixes the environment',
    'output.759': 'visual mode needs a capable terminal',
    'output.762': 'visual mode needs a capable terminal',
    'output.1802': 'needs a variable-width status field already in the row; no script found',
    'pattern.297': 'dead code (#if 0)',
    'pattern.335': 'internal consistency check',
    'pattern.505': 'unreachable: smatch_check() rejects the pattern first (see pattern.583)',
    'process.246': 'needs a user interrupt (^C) of a synchronous process',
    'process.468': 'tracing of /quote lines; needs a running /quote process with echo',
    'process.599': 'popen() failure for /quote !cmd',
    'tfio.106': 'tfname() without a default macro; every caller found supplies one or checks first',
    'tfio.125': 'compiled out where getpwnam() exists',
    'tfio.220': 'cygwin32 builds only',
    'tfio.853': 'needs nested keyboard reads (interactive input)',
    'util.566': 'ch_locale() always uses the environment (""); needs an invalid LANG (harness fixes the environment)',
    'util.581': 'no setlocale() on the platform',
    'util.674': 'needs HOME/MAIL environment lacking a mail file name (startup)',
    'util.744': 'mail-file check; needs MAIL/TFMAILPATH set to an unreadable file',
    'util.758': 'mail-file check; needs a mail file with mail in it',
    'util.763': 'mail-file check; needs a mail file that grew',
    'util.767': 'mail-file check; needs a mail file that grew',
    'util.912': 'only in builds without floating point (NO_FLOAT)',
    'variable.116': 'setting_nearest path with pedantic; no script found that reaches it',
    'variable.540': 'only reachable in squat sandbox mode',
    'variable.629': 'illegal-at-top-level check; no script found that reaches it (/let at file level is accepted)',
    'world.198': 'needs a world with a live socket',
    'world.329': 'world put by a squat bridge',
}
KIND_REASON = {'I': 'internal error (impossible state), cannot be provoked from a script'}
NOMEM = re.compile(r'not enough memory|out of memory|Not enough memory')
SOCKETISH = re.compile(r'^(socket|squat|restart|signals|lua)\.')


def cunesc(s):
    out, i = '', 0
    while i < len(s):
        if s[i] == '\\' and i + 1 < len(s):
            n = s[i + 1]
            out += {'"': '"', '\\': '\\', 'n': '\n', 't': '\t'}.get(n, '\\' + n)
            i += 2
        else:
            out += s[i]; i += 1
    return out


def esc(s):
    return s.replace('\\', '\\\\').replace('\t', '\\t').replace('\n', '\\n')


def load_cases():
    cs = {}
    import glob
    for path in sorted(glob.glob(os.path.join(WDIR, '*.tf'))):
        for l in open(path):
            if l.startswith(';;; '):
                head, _, desc = l[4:].rstrip('\n').partition('#')
                key = head.split()[0]
                cs[key] = desc.strip()
    return cs


def main(src):
    raw = subprocess.run([sys.executable, os.path.join(HERE, 'warn_extract.py'), src], capture_output=True, text=True, check=True).stdout
    rows, seen = [], {}
    for l in raw.split('\n'):
        if not l: continue
        f, line, func, kind, tpl, _args = (l.split('\t') + [''] * 6)[:6]
        stem = f[:-2]
        key = '%s.%s' % (stem, line)
        if key in seen:
            seen[key] += 1
            key += '.' + 'abcdef'[seen[key] - 1]
        else:
            seen[key] = 1
        rows.append([key, kind, f, int(line), func, cunesc(tpl), '', ''])
    for key, kind, f, line, func, tpl, note in ADD:
        rows.append([key, kind, f, line, func, tpl, '', note])
    out = []
    cases = load_cases()
    keys = set()
    for key, kind, f, line, func, tpl, note in [(r[0], r[1], r[2], r[3], r[4], r[5], r[7]) for r in rows]:
        if key in DROP: continue
        tpl = FIX.get(key, tpl).replace('<strerror>', '%s')
        ctx = {'E': 'eprefix', 'W': 'eprefix', 'I': 'eprefix', 'O': 'none', 'H': 'none', 'T': 'custom'}[kind]
        if tpl.startswith('%% '):
            tpl = tpl[3:]   # the literal "% " lead-in of oprintf/hook text is not part of the template
        trig = cases.get(key)
        status, why = 'testable', note
        if key not in cases:
            status = 'untestable'
            why = note
            if key in REASONS: why = REASONS[key]
            for rx, reason in ([] if key in REASONS else UNTESTABLE):
                if re.match(rx, key) and reason:
                    why = reason; break
            if not why and kind in KIND_REASON: why = KIND_REASON[kind]
            if not why and NOMEM.search(tpl): why = 'out-of-memory condition'
            if not why and SOCKETISH.match(key): why = 'needs a live socket / OS failure / external bridge'
            if not why: why = 'TODO'
        keys.add(key)
        out.append([key, kind, f, str(line), func, tpl, ctx, trig or '', status, key if key in cases else '', why if status == 'untestable' else note])
    for k in cases:
        if k not in keys: print('warning: case %s has no catalog entry' % k, file=sys.stderr)
    with open(os.path.join(WDIR, 'catalog.tsv'), 'w') as f:
        f.write('\t'.join(COLS) + '\n')
        for r in out: f.write('\t'.join(esc(x) for x in r) + '\n')
    st = {}
    for r in out: st[r[8]] = st.get(r[8], 0) + 1
    todo = [r[0] for r in out if r[10] == 'TODO']
    print(len(out), 'entries', st, 'TODO:', len(todo))
    if todo and '-v' in sys.argv: print(' '.join(todo))


if __name__ == '__main__':
    main(sys.argv[1])
