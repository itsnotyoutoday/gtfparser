/set wrapsize=80
/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/def go = \
  /addworld -Ttiny w1 bob secret host.example 4000 mfile%; \
  /addworld w2 host2 23%; \
  /addworld -x -e -pTlp w3 host3 24%; \
  /addworld -s10.0.0.1 w4 host4 25 mf4%; \
  /addworld default me pw%; \
  /addworld w5%; \
  /addworld -Tdiku w6 char pass host6 26%; \
  /addtiny w7 host7 27%; \
  /addlp w8 c p host8 28%; \
  /cap listworlds -c%; \
  /cap listworlds
/go
/def go2 = \
  /purgeworld w1 w2%; /cap listworlds -s%; \
  /echo r=$[world_info("w3", "type")]$[world_info("w4", "src")]
/go2
