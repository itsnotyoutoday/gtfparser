/test E(borg)
/test E(+borg)
/test E(borg == 1)
/test E(!borg)
/set borg=off
/test E(borg)
/test E(+borg)
/test E(!borg)
/set borg=1
/test E(borg)
/set borg=on
/test E(borg + 1)
/test E(sigfigs)
/test E(matching)
/test E(+matching)
/test E(matching =~ "glob")
/set matching=regexp
/test E(+matching)
/set matching=bogus
/test E(matching)
/test E(sub)
/test E(+sub)
/test E(wrapsize := 5)
/test E(wrapsize)
/set wrapsize=abc
/test E(wrapsize)
/test E(histsize)
/set histsize=-5
/test E(histsize)
/test E(ptime)
/test E(alert_time)
/test E(alert_attr)
/test E(mprefix)
/test E(textdiv_str)
/test E(max_recur)
/test E(tabsize)
/set sigfigs=5
/test E(1/3.0)
/test E(gag)
/test E(hilite)
/test E(emulation)
/test E(+emulation)
/set mecho=all
/test E(+mecho)
/test E(binary_eol)
/test E(wordpunct)
/test E(isize)
/test E(status_pad)
