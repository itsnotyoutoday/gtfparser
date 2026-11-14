/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/set shpause=off
/set wrapsize=80
/sh rm -f /tmp/gtf_sw_1.txt /tmp/gtf_sw_2.txt
/def mk = \
  /test addworld("foo", "tiny", "host.example", "4000", "bob", "sekret", "", "", "")%; \
  /test addworld("bar", "")%; \
  /test addworld("baz", "lp", "h2", "23", "", "", "mf", "xe", "src")%; \
  /test addworld("default", "", "", "", "me", "pw", "")%; \
  /test addworld("Qux", "diku", "q.example", "5555", "a'b", "c d", "", "p", "")
/mk
/def rd = /test fd := tfopen("%1", "r")%; /while (tfread(fd, l) >= 0) /echo [%l]%; /done%; /test tfclose(fd)
/def sw = \
  /saveworld /tmp/gtf_sw_1.txt%; /echo r=%?%; \
  /rd /tmp/gtf_sw_1.txt%; \
  /unworld bar baz Qux foo default%; \
  /cap listworlds -s%; \
  /load /tmp/gtf_sw_1.txt%; \
  /cap listworlds -c%; \
  /saveworld -a /tmp/gtf_sw_1.txt%; /echo r=%?%; \
  /rd /tmp/gtf_sw_1.txt%; \
  /saveworld /tmp/gtf_nodir/x/y%; /echo r=%?%; \
  /saveworld -z x%; /echo r=%?
/sw
/def sw2 = \
  /def WORLDFILE = /tmp/gtf_sw_2.txt%; \
  /saveworld%; /echo r=%?%; /rd /tmp/gtf_sw_2.txt
/sw2
/sh rm -f /tmp/gtf_sw_1.txt /tmp/gtf_sw_2.txt
/echo --conn
/def cn = \
  /dc%; /echo r=%?%; \
  /dc nosuch%; /echo r=%?%; \
  /dc -all%; /echo r=%?%; \
  /fg nosuch%; /echo r=%?%; \
  /fg -s nosuch%; /echo r=%?%; \
  /fg -n%; /echo r=%?%; \
  /fg%; /echo r=%?%; \
  /fg -z%; /echo r=%?%; \
  /cap listsockets%; \
  /cap listsockets -s%; \
  /echo r=$[nactive()]$[is_connected("foo")]$[is_open("foo")]
/cn
