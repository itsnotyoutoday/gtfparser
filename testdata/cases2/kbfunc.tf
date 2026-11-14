/def kb1 = /test input("hello world")%; /echo [$[kbhead()]] [$[kbtail()]] [$[kbpoint()]] [$[kblen()]]
/kb1
/test input("")
/test input("hello world foo")
/test kbgoto(0)
/test E(kbpoint())
/test E(kblen())
/test E(kbhead())
/test E(kbtail())
/test kbgoto(5)
/test E(kbhead())
/test E(kbtail())
/test E(kbwordleft())
/test E(kbwordright())
/test E(kbcharfwd(1))
/test E(kbcharback(1))
/test kbdel(3)
/test E(strcat(kbhead(), "|", kbtail()))
/test input("abc def ghi")
/test kbgoto(4)
/test E(strcat(kbhead(), "|", kbtail()))
/test kbgoto(0)
/test input("one two three")
/test kbgoto(4)
/kb_upcase_word
/test E(strcat(kbhead(), "|", kbtail()))
/test input("one two three")
/test kbgoto(4)
/kb_capitalize_word
/test E(strcat(kbhead(), "|", kbtail()))
/test input("ONE TWO")
/test kbgoto(0)
/kb_downcase_word
/test E(strcat(kbhead(), "|", kbtail()))
/test input("abcdef")
/test kbgoto(3)
/kb_transpose_chars
/test E(strcat(kbhead(), "|", kbtail()))
/test input("a   b    c")
/test kbgoto(2)
/kb_collapse_space
/test E(strcat(kbhead(), "|", kbtail()))
/test input("")

