# Source me. Points the differential harness at a reference tinyfugue build.
#
#   TFREF_DIR  directory of a built TinyFugue tree (containing src/tf and
#              lib/tf). Default: ~/.cache/gtfparser/tf
#
# Build one from a TinyFugue source tree with a UTF-8 (WIDECHAR) build so
# regexps behave as in a normal install; see scripts/pcre_build_tfwide.sh.
: "${TFREF_DIR:=$HOME/.cache/gtfparser/tf}"
export GOWORK=off TF_REF=$TFREF_DIR/src/tf TF_LIB=$TFREF_DIR/lib/tf LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8
