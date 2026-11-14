/def kb = /test E(strcat("[", kbhead(), "|", kbtail(), "] pt=", kbpoint(), " len=", kblen()))
/def r = /test E(strcat("r=", %*))
/echo --- input and movement
/kb
/input hello world foo
/kb
/test kbgoto(5)
/kb
/test kbgoto(100)
/kb
/test kbgoto(-4)
/kb
/test kbgoto(6)
/input XY
/kb
/set insert=off
/test kbgoto(2)
/input ab
/kb
/test kbgoto(kblen()-1)
/input 12345
/kb
/set insert=on
/echo --- kbdel
/test kbgoto(3)
/test E(strcat("d=", kbdel(1)))
/kb
/test E(strcat("d=", kbdel(kblen())))
/kb
/test E(strcat("d=", kbdel(0)))
/kb
/test E(strcat("d=", kbdel(99)))
/kb
/test kbgoto(2)
/test E(strcat("d=", kbdel(2)))
/kb
/echo --- words
/dokey dline
/input foo bar_baz  qux.quux (a[b]{c})
/test kbgoto(0)
/def w = /test E(strcat("wr(", %1, ")=", kbwordright(%1), " wl=", kbwordleft(%1)))
/w 0
/w 2
/w 3
/w 4
/w 8
/w 12
/w 16
/w 27
/w 40
/set wordpunct=_.
/w 8
/w 16
/set wordpunct=_
/test E(strcat("wr=", kbwordright(), " wl=", kbwordleft()))
/test kbgoto(10)
/test E(strcat("wr=", kbwordright(), " wl=", kbwordleft()))
/echo --- match
/def m = /test E(strcat("m(", %1, ")=", kbmatch(%1)))
/m 0
/m 3
/m 17
/m 19
/m 21
/m 22
/m 25
/m 26
/m 29
/m 30
/m 40
/test kbgoto(20)
/test E(strcat("m=", kbmatch()))
/echo --- chars
/dokey dline
/input héllo
/test kbgoto(kblen())
/test E(strcat("back=", kbcharback(1), " ", kbcharback(2), " ", kbcharback(9), " fwd=", kbcharfwd(1), " ", kbcharfwd(3)))
/test kbgoto(1)
/test E(strcat("back=", kbcharback(1), " fwd=", kbcharfwd(1), " ", kbcharfwd(2)))
/test kbgoto(3)
/test E(strcat("back=", kbcharback(1), " ", kbcharback(2), " fwd=", kbcharfwd(1), " ", kbcharfwd(2)))
/echo --- dokey
/dokey dline
/input one two three four
/test kbgoto(kblen())
/dokey bword
/kb
/dokey bword
/kb
/input   
/dokey bword
/kb
/dokey home
/kb
/dokey dch
/kb
/dokey right
/kb
/dokey left
/kb
/dokey wright
/kb
/dokey wleft
/kb
/dokey end
/kb
/dokey bspc
/kb
/dokey home
/dokey dword
/kb
/dokey wright
/dokey deol
/kb
/input again here
/test kbgoto(6)
/dokey deol
/kb
/dokey dline
/kb
/dokey bspc
/kb
/dokey dch
/kb
/dokey home
/kb
/dokey nosuchop
/echo --- kbnum
/dokey dline
/input abcdefghij
/set kbnum=3
/dokey bspc
/kb
/set kbnum=
/test kbgoto(2)
/set kbnum=4
/dokey dch
/kb
/set kbnum=2
/dokey right
/kb
/set kbnum=2
/dokey left
/kb
/set kbnum=2
/input Z
/kb
/echo --- history
/dokey dline
/dokey recallb
/kb
/echo --- kbnum words
/dokey dline
/input aa bb cc dd ee
/test kbgoto(0)
/set kbnum=2
/dokey wright
/kb
/set kbnum=-1
/dokey wright
/kb
/set kbnum=2
/dokey wleft
/kb
/test kbgoto(6)
/set kbnum=2
/dokey dword
/kb
/set kbnum=3
/dokey deol
/kb
/test kbgoto(kblen())
/set kbnum=2
/dokey bword
/kb
/set kbnum=
/echo --- page keys without output
/set kbnum=2
/dokey page
/dokey pageback
/dokey hpage
/dokey hpageback
/dokey line
/dokey lineback
/dokey pgup
/dokey pgdn
/set kbnum=
