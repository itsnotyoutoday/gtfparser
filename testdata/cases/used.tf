/def capl = /list %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/def -t'ut' -F ub = /echo trig
/def -hKILL -F uh = /echo hook
/def uc = /echo called
/def -t'both' -hMORE -F ubh = /echo both
/def -b'^Xu' ukey = /echo key
/capl -su ub
/capl -su uh
/capl -su uc
/uc
/uc
/uc
/trigger ut
/trigger ut
/trigger -hKILL 1
/trigger both
/trigger -hMORE
/trigger -hMORE
/test E(uc())
/capl -su ub
/capl -su uh
/capl -su uc
/capl -su ubh
/capl -su ukey
/capl -s ubh
/echo -- one-shot macros called by name are not consumed
/def -n1 once = /echo once ran
/once
/once
/capl -su once
