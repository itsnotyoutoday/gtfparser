/def m1 = /echo n %%%;/echo b
/m1
/def m2 = /echo [%{-L}]|
/def m3 = /test E(m2("x","","z"))
/m3
/m2 a b c
