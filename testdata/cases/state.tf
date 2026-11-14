/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/def lsnk = /while (tfread(l) >= 0) /echo [$[strlen(l) > 60 ? strcat(substr(l, 0, 20), "<out>") : l]]%; /done
/def lcap = /%* %| /lsnk
/set shpause=off
/echo --limit
/def lm = \
  /limit%; /echo r=%?%; \
  /relimit%; /echo r=%?%; \
  /unlimit%; /echo r=%?%; \
  /limit foo%; /echo r=%?%; \
  /limit%; /echo r=%?%; \
  /relimit%; /echo r=%?%; \
  /unlimit%; /echo r=%?%; \
  /limit -a%; /echo r=%?%; \
  /limit -mbogus x%; /echo r=%?%; \
  /limit -z%; /echo r=%?
/lm
/echo --features
/def ft = /features float%; /echo r=%?%; /features nosuch%; /echo r=%?%; /features FLOAT%; /echo r=%?%; /features ssl%; /echo r=%?
/ft
/echo --watch
/def wt = \
  /cap watchdog%; /cap watchdog 3 6%; /cap watchdog%; /cap watchdog off%; /cap watchdog%; /cap watchdog on%; /cap watchdog OFF%; \
  /cap watchname%; /cap watchname on%; /cap watchname 2 3%; /cap watchname off%; /cap watchname ON%; /cap watchname off%; \
  /cap watchname 1%; /cap watchname x y%; /cap watchname -1 2%; /cap watchdog 0 0%; /cap watchdog off%; \
  /echo [%watchdog][%watchname]
/wt
/echo --streams
/def ls = \
  /lcap liststreams%; \
  /test fd1 := tfopen("/tmp/gtf_st_1.txt", "w")%; \
  /test fd2 := tfopen("/dev/null", "r")%; \
  /test fd3 := tfopen("/tmp/gtf_st_1.txt", "a")%; \
  /lcap liststreams%; \
  /test tfclose(fd2)%; \
  /lcap liststreams%; \
  /test tfclose(fd1)%; /test tfclose(fd3)%; \
  /lcap liststreams
/ls
/sh rm -f /tmp/gtf_st_1.txt
/echo --macros
/def m1 = /echo one
/def -t'abc*' m2 = /echo two
/def -p5 -mregexp -t'x(y)' -F m3 = /echo three
/def -h'CONNECT w*' m4 = /echo four
/def -i m5 = /echo five
/def m6
/def m7 = /echo seven %{1}
/def -n3 -ag -t'zz*' m8 = /echo eight
/test addworld("foo", "")
/def -T'tiny' -w'foo' m10 = /echo ten
/def -a'B' m11
/def -E'1 == 1' m12 = /echo twelve
/def -q m13 = /echo thirteen
/def -p3 -hRESIZE m14
/def -aB m15 = /echo 15
/sh rm -f /tmp/gtf_sv_1.txt /tmp/gtf_sv_2.txt
/def sv = \
  /save /tmp/gtf_sv_1.txt m*%; \
  /save /tmp/gtf_sv_2.txt -t* m*%; \
  /save -a /tmp/gtf_sv_2.txt m1%; \
  /save%; \
  /save /tmp/gtf_sv_none.txt nosuchmacro_*%; \
  /echo n=%?
/sv
/def rd = /test fd := tfopen("%1", "r")%; /while (tfread(fd, l) >= 0) /echo [%l]%; /done%; /test tfclose(fd)
/rd /tmp/gtf_sv_1.txt
/echo --
/rd /tmp/gtf_sv_2.txt
/sh rm -f /tmp/gtf_sv_1.txt /tmp/gtf_sv_2.txt /tmp/gtf_sv_none.txt
