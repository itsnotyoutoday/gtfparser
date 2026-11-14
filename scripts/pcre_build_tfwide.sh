#!/bin/sh
# Build a WIDECHAR (UTF-8 / ICU) copy of the reference tf for scripts/pcre_diff.py.
# usage: . scripts/env.sh && scripts/pcre_build_tfwide.sh DESTDIR
# then:  TF_REF=DESTDIR/src/tf TF_LIB=DESTDIR/lib/tf python3 scripts/pcre_diff.py
# Needs Homebrew icu4c (override with ICU=/path/to/icu4c) and a configured tf tree
# at $(dirname $TF_REF)/.. (what env.sh points at).
set -e
dest=${1:?usage: pcre_build_tfwide.sh DESTDIR}
src=$(cd "$(dirname "${TF_REF:?source scripts/env.sh first}")/.." && pwd)
icu=${ICU:-$(ls -d /opt/homebrew/opt/icu4c* | tail -1)}
rm -rf "$dest"
cp -R "$src" "$dest"
cd "$dest/src"
rm -f ./*.o tf
sed -i.bak 's/#define WIDECHAR 0/#define WIDECHAR 1/' tfconfig.h
# _DARWIN_C_SOURCE: WIDECHAR defines _XOPEN_SOURCE, which hides BSD socket helpers on macOS
sed -i.bak "s|^CFLAGS     = |CFLAGS     = -D_DARWIN_C_SOURCE -I$icu/include |; s|^LIBS       = |LIBS       = -L$icu/lib -licuuc |" Makefile
make >/dev/null
echo "built $dest/src/tf"
