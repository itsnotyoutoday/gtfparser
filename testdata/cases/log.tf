/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/set shpause=off
/test addworld("hw", "")
/test addworld("hx", "")
/def lg = \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log -g /tmp/gtf_lg_g.txt%; \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log -i /tmp/gtf_lg_i.txt%; \
  /log -l /tmp/gtf_lg_l.txt%; \
  /log -whw /tmp/gtf_lg_w.txt%; \
  /log -whx /tmp/gtf_lg_x.txt%; \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log -g /tmp/gtf_lg_g2.txt%; \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log -whw OFF%; \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log -l off%; \
  /log -i OFF%; \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log OFF%; \
  /echo nlog=$[nlog()]%; \
  /cap log%; \
  /log OFF%; \
  /echo r=$[0]%; \
  /log -whw -l x%; \
  /log -wnosuch x%; \
  /log -ligx%; \
  /echo nlog=$[nlog()]
/lg
/def lg2 = \
  /set LOGFILE=nope%; \
  /def LOGFILE = /tmp/gtf_lg_dflt.txt%; \
  /log -g ON%; \
  /cap log%; \
  /log -g%; \
  /cap log%; \
  /log OFF%; \
  /echo nlog=$[nlog()]
/lg2
/sh rm -f /tmp/gtf_lg_g.txt /tmp/gtf_lg_i.txt /tmp/gtf_lg_l.txt /tmp/gtf_lg_w.txt /tmp/gtf_lg_x.txt /tmp/gtf_lg_g2.txt /tmp/gtf_lg_dflt.txt
