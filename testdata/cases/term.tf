/def ee = /test E(strcat(%*))
/ee "keycode up=[", keycode("Up"), "]"
/ee "keycode bs=[", keycode("Backspace"), "]"
/ee "keycode unk=[", keycode("Nosuchkey"), "]"
/ee "keycode f1=[", keycode("f1"), "]"
/ee "morepaused=", morepaused()
/ee "morepaused w=", morepaused("nowhere")
/ee "moresize=", moresize()
/ee "moresize n=", moresize("n")
/ee "moresize ln=", moresize("ln")
/ee "moresize bad=", moresize("z")
/ee "moresize w=", moresize("", "nowhere")
/ee "morescroll=", morescroll(5)
/ee "morescroll-=", morescroll(-5)
/ee "nactive=", nactive()
/ee "nactive w=", nactive("nowhere")
/ee "prompt=", prompt("hi> ")
/ee "fake_recv=", fake_recv("hello")
/ee "fake_recv w=", fake_recv("hello", "nowhere")
/ee "fake_recv bad=", fake_recv("hello", "", "z")
/localecho on
/localecho off
/beep off
/test E(strcat("beep=", beep))
/beep on
/test E(strcat("beep=", beep))
/beep 0
/set beep=off
/test E(strcat("beep=", beep))
/set beep=on
/echo --- status vars
/test E(strcat("sh=", status_height, " sp=", status_pad, " sa=[", status_attr, "]"))
/set status_pad=x
/test E(strcat("sp=", status_pad))
/echo --- /input kbnum and sub
/dokey dline
/set kbnum=3
/input ab
/test E(strcat("[", kbhead(), "|", kbtail(), "]"))
/test E(strcat("kbnum=[", kbnum, "]"))
/dokey dline
/input ^A
/test E(strcat("[", kbhead(), "]"))
