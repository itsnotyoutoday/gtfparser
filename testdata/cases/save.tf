/def sv1 = /echo one
/def -t'sv*' -p3 -F -aB sv2 = /echo two %1
/def -mregexp -t'(a)(b)' -P1u;2B -F sv3
/def -h'CONNECT x*' -n2 sv4 = /echo hook
/def -E'foo' -w'' -c40 -q sv5 = body
/def -i sv6 = invisible
/def -b'^Xz' sv7 = /echo bound
/def -ag -msubstr -t"it's" sv8
/def rdsave = /test fd := tfopen("/tmp/gtf_save_test.tf", "r")%; /let l=%; /while (tfread(fd, l) >= 0) /echo [%l]%; /done%; /test tfclose(fd)
/save /tmp/gtf_save_test.tf sv*
/rdsave
/save -a /tmp/gtf_save_test.tf -msimple sv1
/rdsave
/save /tmp/gtf_save_test.tf -msimple nosuchmacroxyz
/rdsave
/save
/save /tmp/gtf_save_test.tf -i sv6
/rdsave
; saved macros load back to the same definitions
/save /tmp/gtf_save_test.tf sv*
/undef sv1 sv2 sv3 sv4 sv5 sv7 sv8
/load /tmp/gtf_save_test.tf
/save /tmp/gtf_save_test.tf sv*
/rdsave
