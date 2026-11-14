/def -i _b0 = /echo $[strlen("a","b")] x%;/echo after
/_b0
/def -i _b1 = /echo $[1 ?] x%;/echo after
/_b1
/def -i _b2 = /test strlen("a","b")%;/echo after
/_b2
/def -i _b3 = /echo a%;/test sin()%;/echo after
/_b3
/def -i _b4 = /echo $[nosuchfn(1)] x%;/echo after
/_b4
/def -i _b5 = /def f = /echo in f%;/test strlen()%;/echo f-after%;/f%;/echo after-f
/_b5
