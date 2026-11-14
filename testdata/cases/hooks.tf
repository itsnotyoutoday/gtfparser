/def capl = /list %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
; events and argument patterns (/trigger -h fires a hook like a real event)
/def -hCONNECT -F h1 = /echo h1 [%*]
/def -h'CONNECT mud*' -F h2 = /echo h2 [%*]
/def -h'CONNECT|DISCONNECT {mud|tiny}*' -F h3 = /echo h3 [%*]
/def -mregexp -h'DISCONNECT ^(\w+) closed' -F h4 = /echo h4 P1=[%P1]
/def -h* -F -p0 hall = /echo hall
/echo -- CONNECT mud1
/trigger -hCONNECT mud1
/echo -- connect tiny
/trigger -hconnect tiny
/echo -- DISCONNECT mud closed
/trigger -hDISCONNECT mud1 closed
/echo -- a different event
/trigger -hWORLD foo
/echo -- bad event
/trigger -hNOSUCH foo
/echo -- result is the count of non-quiet macros
/def -hLOG -q lq = /echo quiet hook
/def -hLOG -F -p0 lq2 = /echo loud hook
/test E(1)
/echo -- priority and fall-thru
/def -hKILL -p5 -F kf5 = /echo kf5
/def -hKILL -p4 k4 = /echo k4
/def -hKILL -p3 k3 = /echo never
/trigger -hKILL 123
/echo -- shots
/def -hPROCESS -n2 ps = /echo process shot
/trigger -hPROCESS 1
/trigger -hPROCESS 2
/trigger -hPROCESS 3
/echo -- hook flag
/set hook=off
/def -hMORE hm = /echo more hook
/trigger -hMORE
/set hook=on
/trigger -hMORE
/echo -- /hook command
/hook SIGHUP = /echo sighup hook
/trigger -hSIGHUP
/hook SIGUSR1 foo* = /echo usr1 foo %*
/trigger -hSIGUSR1 foobar
/trigger -hSIGUSR1 bar
/hook SIGUSR2|SIGTERM = /echo usr2/term %*
/trigger -hSIGTERM x
/trigger -hSIGUSR2 y
/echo -- /list formats
/def -hCONNECT|DISCONNECT lh1
/capl lh1
/capl -s lh1
/def -h'WORLD x y' -mregexp lh2
/capl lh2
/def -mglob -hWORLD lh3 = /echo x
/capl lh3
/capl -hRESIZE
/hook RESIZE = /echo rz
/def -hRESIZE -n3 -p7 -F -c50 lh4 = /echo rz4
/capl lh4
/capl -s lh4
/capl -h'RESIZE' lh4
/capl -h0 lh4
/capl -h lh4
/capl -hMORE lh4
/echo -- REDEF and CONFLICT hooks run from /def
/def -hREDEF -F rd = /echo redef: %*
/def -hCONFLICT -F cf = /echo conflict: %*
/def rdm = /echo one
/def rdm = /echo two
/def rdm = /echo two
/def -t'redefme' rdm = /echo three
/def gag = /echo gag macro
/def Gag = /echo gag macro 2
/undef gag
/echo -- NOMACRO hook
/def -hNOMACRO -F nm = /echo nomacro: %*
/nosuchcommandxyz arg
/echo -- hook with attributes and -E
/set hflag=0
/def -hSEND -E'hflag' -F sd1 = /echo send hook %*
/trigger -hSEND foo
/set hflag=1
/trigger -hSEND foo
/echo -- max_hook
/set max_hook=2
/def -hLOADFAIL -F lf = /echo loadfail %*
/trigger -hLOADFAIL 1
/trigger -hLOADFAIL 2
/trigger -hLOADFAIL 3
/test E(hook)
/set max_hook=1000
