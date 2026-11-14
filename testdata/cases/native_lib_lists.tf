; the list and purge families, /untrig, /unhook, /undeft
/def -i T = /eval %*
/def -mglob -t'a*b' -ag gg1 = x
/def -mglob -t"it's" -ag gg2
/def -mglob -t'foo*' -F trg1 = /echo t
/def -mregexp -t'^x(y)' -P1B pp1
/def -mglob -t'hh*' -aB hi1
/def -mglob -t'q?' -ahu hi2
/def -b'^X^Y' kb1 = /echo bind
/def -b'^X^Z' kb2 = /echo bind2
/def -mglob -h'CONNECT fo*' hk1 = /echo hook
/def -mglob -hDISCONNECT hk2 = /echo hook2
/def -i invis = /echo hidden
/def plain1 = /echo p1
/T /echo listgag=[$(/listgag)]
/T /echo listgag-pat=[$(/listgag a*b)]
/T /echo listgag-quote=[$(/listgag it's)]
/T /echo listgag-none=[$(/listgag nomatch)]
/T /echo listtrig=[$(/listtrig)]
/T /echo listtrig-pat=[$(/listtrig foo*)]
/T /echo listhilite=[$(/listhilite)]
/T /echo listfullhilite=[$(/listfullhilite)]
/T /echo listfullhilite-pat=[$(/listfullhilite hh*)]
/T /echo listpartial=[$(/listpartial)]
/T /echo listbind=[$(/listbind)]
/T /echo listbind-pat=[$(/listbind ^X^Y)]
/T /echo listhook=[$(/listhook)]
/T /echo listhook-pat=[$(/listhook DISCONNECT)]
/T /echo listdef=[$(/listdef plain*)]
/T /echo listdef-opts=[$(/listdef -mglob -s gg*)]
/T /echo purgegag=[$(/purgegag a*b)]
/T /echo listgag=[$(/listgag)]
/T /echo purgehilite=[$(/purgehilite)]
/T /echo listhilite=[$(/listhilite)]
/T /echo purgebind=[$(/purgebind ^X^Y)]
/T /echo listbind=[$(/listbind)]
/T /echo purgehook=[$(/purgehook DISCONNECT)]
/T /echo listhook=[$(/listhook)]
/T /echo purgetrig=[$(/purgetrig foo*)]
/T /echo listtrig=[$(/listtrig)]
/T /echo purgedef=[$(/purgedef plain*)]
/T /echo listdef=[$(/listdef plain*)]
/purgedef nomatchxx
/T /echo purgedef-none=%?
/def -i p2 = /echo p2
/def -i p3 = /echo p3
/purgedef p2
/T /echo p2gone=[$(/listdef p*)]
/purgedef
/T /echo all-gone=[$(/listdef -s)]
/def -mglob -t'dd*' -ag d1
/def -mglob -t'dd*' -ah d2
/def -mglob -t'ee*' -F d3 = /echo x
/purgedeft dd*
/T /echo deft=[$(/listdef -s d*)]
/untrig ee*
/T /echo untrig=[$(/listdef -s d*)]
/untrig nosuchtrig
/T /echo untrig-none=%?
/def -mglob -t'ff*' -ag f1
/untrig -ag ff*
/T /echo untrig-ag=[$(/listdef -s f*)]
/def -mglob -hCONNECT hh1
/unhook CONNECT
/T /echo unhook=[$(/listdef -s hh*)]
/unhook CONNECT
/T /echo unhook-none=%?
/def -mglob -t'gg*' -ag g1
/def -mglob -t'gg*' -aB g2
/def -mglob -t'gg*' g3 = /echo x
/undeft gg*
/T /echo undeft=[$(/listdef -s g?)]
/def -mglob -t'one two' -aB h1

