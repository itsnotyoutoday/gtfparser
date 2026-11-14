/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/test addworld("hw", "")
/test addworld("hx", "")
/def go = \
  /recordline -whw first line%; /recordline -whw -t1000000000 old line%; /recordline -whw second line foo%; /recordline -whw -ag gagged line%; /recordline -whw third foo line%; \
  /echo --a%; /cap recall -whw -q 1-%; \
  /echo --b%; /cap recall -whw -q #1-%; \
  /echo --c%; /cap recall -whw -q /2%; \
  /echo --d%; /cap recall -whw -q 2%; \
  /echo --e%; /cap recall -whw -q 1- *foo*%; \
  /echo --f%; /cap recall -whw -q -t%%Y 1-%; \
  /echo --g%; /cap recall -whw -q -tx%%Y 3-%; \
  /echo --h%; /cap recall -whw -q -v - -1%; \
  /echo --i%; /cap recall -whw 1-2%; \
  /echo --j%; /cap recall -whw -ag -q 1-%; \
  /echo --k%; /cap recall -whw -a -q 1-%; \
  /echo --l%; /cap recall -whw -q -ag #1-%; \
  /echo --m%; /cap recall -whw -q 1-5 *line*%; \
  /echo --n%; /cap recall -whw -q 3-4%; \
  /echo --o%; /cap recall -whw -ag -q 3-4%; \
  /echo --p%; /cap recall -whw -q -3%; \
  /echo --q%; /cap recall -wnosuch -q 1-%; \
  /echo --r%; /cap recall -whw -l -q 1-%; \
  /echo --s%; /cap recall -whw -a@ -q 1-%; \
  /echo --t%; /cap recall -whw -mfoo -q 1-%; \
  /echo --u%; /cap recall -whw -q 1-2x%; \
  /echo --v%; /cap recall -whw -q%; \
  /echo --w%; /cap recall -whx -q 1-%; \
  /echo --x%; /cap recall -whw -q -t 1000000000-1000000001%; \
  /echo --y%; /cap recall -whw -q -A9 1000000000-1000000001
/go
/def go2 = \
  /cap histsize -whw%; /cap histsize -whw 3%; /cap histsize -whw%; /cap recall -whw -q 1-%; \
  /cap histsize -whx%; /cap histsize -whx 0%; /cap histsize -whx x%; /cap histsize -l%; /cap histsize -i%; /cap histsize -g%; /cap histsize -g 50%; /cap histsize -g%; \
  /set histsize=10%; /cap histsize -whx%; /cap histsize -wnosuch
/go2
/def go3 = \
  /recordline -whx a1%; /recordline -whx a2%; /recordline -whx a3%; /recordline -whx a4%; /recordline -whx a5%; /recordline -whx a6%; /recordline -whx a7%; \
  /cap histsize -whx%; /cap histsize -whx 4%; /cap recall -whx -q #1-%; \
  /recordline -whx a8%; /cap recall -whx -q #1-%; /cap recall -whx -q #6-7%; /cap recall -whx -q 2
/go3
/def go4 = \
  /recordline -i one%; /recordline -i two%; /recordline -i two%; /recordline -i three%; \
  /cap recall -i -q 1-%; /cap recall -i -q #1-%;
/go4
/def go5 = /echo [$[0]]%; /recordline -w%; /recordline -ligl x%; /recordline -p -whw @{B}bold@{n} and @@at%; /cap recall -whw -q /1
/go5
