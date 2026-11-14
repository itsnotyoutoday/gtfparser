/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/test addworld("hw", "")
/def go = \
  /recordline -whw l1 a%; /recordline -whw l2 b%; /recordline -whw l3 foo%; /recordline -whw l4 d%; /recordline -whw l5 e%; /recordline -whw l6 f%; /recordline -whw l7 foo%; /recordline -whw l8 h%; /recordline -whw l9 i%; /recordline -whw l10 foo%; /recordline -whw l11 k%; \
  /echo --a%; /cap recall -whw -q -A1 1- *foo*%; \
  /echo --b%; /cap recall -whw -q -B1 1- *foo*%; \
  /echo --c%; /cap recall -whw -q -C1 1- *foo*%; \
  /echo --d%; /cap recall -whw -q -C1 /2 *foo*%; \
  /echo --e%; /cap recall -whw -q #-C1 1- *foo*%; \
  /echo --f%; /cap recall -whw -q -v 1-5 *foo*%; \
  /echo --g%; /cap recall -whw -q -v /2 *foo*%; \
  /echo --h%; /cap recall -whw -q 3-%; \
  /echo --i%; /cap recall -whw -q -5%; \
  /echo --j%; /cap recall -whw -q - -5%; \
  /echo --k%; /cap recall -whw -q 3-5%; \
  /echo --l%; /cap recall -whw -q 20-30%; \
  /echo --m%; /cap recall -whw -q 0-2%; \
  /echo --n%; /cap recall -whw -q /0%; \
  /echo --o%; /cap recall -whw -q /3 *f*%; \
  /echo --p%; /cap recall -whw  2-3%; \
  /echo --q%; /cap recall -whw -q 1000000000%; \
  /echo --r%; /cap recall -whw -q -1000000000%; \
  /echo --s%; /cap recall -whw -q 1.5-%; \
  /echo --v%; /cap recall -whw -q -msimple 1- l5 e%; \
  /echo --w%; /cap recall -whw -q -msubstr 1- foo%; \
  /echo --x%; /cap recall -whw -q -mregexp 1- ^l[0-9] f%; \
  /echo --z%; /cap recall -whw -q #   2-3%; \
  /echo --za%; /cap recall -whw -q # 2-3 l*
/go
