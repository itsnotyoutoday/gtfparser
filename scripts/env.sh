# source me: points the harness at a reference tf build.
# Prefer the UTF-8 (WIDECHAR) build so regexps behave as in a normal install;
# build it with scripts/pcre_build_tfwide.sh DESTDIR.
S=/private/tmp/claude-501/-Users-rock-mush/67b79c2b-ca3c-41e1-9ae4-177603d4bb02/scratchpad
if [ -x $S/tfwide/src/tf ]; then T=$S/tfwide; else T=$S/tfref; fi
export GOWORK=off TF_REF=$T/src/tf TF_LIB=$T/lib/tf LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8
