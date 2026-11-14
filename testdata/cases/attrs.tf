/def capl = /list %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/set hiliteattr=B
/set error_attr=Cred
/set warning_attr=u
/set info_attr=Cgreen
/echo -- -an
/def -an -t'x0' at0
/capl at0
/capl -s at0
/echo -- -ax
/def -ax -t'x1' at1
/capl at1
/capl -s at1
/echo -- -ag
/def -ag -t'x2' at2
/capl at2
/capl -s at2
/echo -- -aG
/def -aG -t'x3' at3
/capl at3
/capl -s at3
/echo -- -aL
/def -aL -t'x4' at4
/capl at4
/capl -s at4
/echo -- -aA
/def -aA -t'x5' at5
/capl at5
/capl -s at5
/echo -- -au
/def -au -t'x6' at6
/capl at6
/capl -s at6
/echo -- -ar
/def -ar -t'x7' at7
/capl at7
/capl -s at7
/echo -- -af
/def -af -t'x8' at8
/capl at8
/capl -s at8
/echo -- -ai
/def -ai -t'x9' at9
/capl at9
/capl -s at9
/echo -- -ad
/def -ad -t'x10' at10
/capl at10
/capl -s at10
/echo -- -aB
/def -aB -t'x11' at11
/capl at11
/capl -s at11
/echo -- -ab
/def -ab -t'x12' at12
/capl at12
/capl -s at12
/echo -- -ah
/def -ah -t'x13' at13
/capl at13
/capl -s at13
/echo -- -aE
/def -aE -t'x14' at14
/capl at14
/capl -s at14
/echo -- -aW
/def -aW -t'x15' at15
/capl at15
/capl -s at15
/echo -- -aI
/def -aI -t'x16' at16
/capl at16
/capl -s at16
/echo -- -aCred
/def -aCred -t'x17' at17
/capl at17
/capl -s at17
/echo -- -aCbgblue
/def -aCbgblue -t'x18' at18
/capl at18
/capl -s at18
/echo -- -aCred,Cbgblue
/def -aCred,Cbgblue -t'x19' at19
/capl at19
/capl -s at19
/echo -- -aCbgredCblue
/def -aCbgredCblue -t'x20' at20
/capl at20
/capl -s at20
/echo -- -aCbrightred
/def -aCbrightred -t'x21' at21
/capl at21
/capl -s at21
/echo -- -aCgray
/def -aCgray -t'x22' at22
/capl at22
/capl -s at22
/echo -- -aCrgb123
/def -aCrgb123 -t'x23' at23
/capl at23
/capl -s at23
/echo -- -aCgray5
/def -aCgray5 -t'x24' at24
/capl at24
/capl -s at24
/echo -- -aC100
/def -aC100 -t'x25' at25
/capl at25
/capl -s at25
/echo -- -aCbg255
/def -aCbg255 -t'x26' at26
/capl at26
/capl -s at26
/echo -- -aCwhite,Cbgblack
/def -aCwhite,Cbgblack -t'x27' at27
/capl at27
/capl -s at27
/echo -- -aBuC9
/def -aBuC9 -t'x28' at28
/capl at28
/capl -s at28
/echo -- -axB
/def -axB -t'x29' at29
/capl at29
/capl -s at29
/echo -- -anB
/def -anB -t'x30' at30
/capl at30
/capl -s at30
/echo -- -agB
/def -agB -t'x31' at31
/capl at31
/capl -s at31
/echo -- -aGLA
/def -aGLA -t'x32' at32
/capl at32
/capl -s at32
/echo -- -aCred,Cgreen
/def -aCred,Cgreen -t'x33' at33
/capl at33
/capl -s at33
/echo -- -aCbgred,Cbggreen
/def -aCbgred,Cbggreen -t'x34' at34
/capl at34
/capl -s at34
/echo -- -aCRED
/def -aCRED -t'x35' at35
/capl at35
/capl -s at35
/echo -- -aBCBLUE
/def -aBCBLUE -t'x36' at36
/capl at36
/capl -s at36
/echo -- -ahu
/def -ahu -t'x37' at37
/capl at37
/capl -s at37
/echo -- -aubrh
/def -aubrh -t'x38' at38
/capl at38
/capl -s at38
/echo -- -aCbrightwhite,Cbgbrightred
/def -aCbrightwhite,Cbgbrightred -t'x39' at39
/capl at39
/capl -s at39
/echo -- -aC0
/def -aC0 -t'x40' at40
/capl at40
/capl -s at40
/echo -- -aCbg0
/def -aCbg0 -t'x41' at41
/capl at41
/capl -s at41
/echo -- -aC255
/def -aC255 -t'x42' at42
/capl at42
/capl -s at42
/echo -- -aCgray23
/def -aCgray23 -t'x43' at43
/capl at43
/capl -s at43
/echo -- -aCrgb555
/def -aCrgb555 -t'x44' at44
/capl at44
/capl -s at44
/echo -- -aQ
/def -aQ -t'x45' at45
/capl at45
/capl -s at45
/echo -- -aCnosuch
/def -aCnosuch -t'x46' at46
/capl at46
/capl -s at46
/echo -- -aCbgnosuch
/def -aCbgnosuch -t'x47' at47
/capl at47
/capl -s at47
/echo -- -aC256
/def -aC256 -t'x48' at48
/capl at48
/capl -s at48
/echo -- -au,B,r
/def -au,B,r -t'x49' at49
/capl at49
/capl -s at49
/echo -- -auBrb
/def -auBrb -t'x50' at50
/capl at50
/capl -s at50
/echo -- -anxg
/def -anxg -t'x51' at51
/capl at51
/capl -s at51
/echo -- attribute variables are expanded when the macro is defined
/set error_attr=Bu
/def -aE -t'ev' ev1
/capl ev1
/set error_attr=
/def -aEW -t'ev' ev2
/capl ev2
/echo -- partial hilites
/echo -- -P1B;2u
/def -mregexp -t'(a)(b)' -P1B;2u pa0
/capl pa0
/capl -s pa0
/echo -- -PBu
/def -mregexp -t'(a)(b)' -PBu pa1
/capl pa1
/capl -s pa1
/echo -- -P0Cred
/def -mregexp -t'(a)(b)' -P0Cred pa2
/capl pa2
/capl -s pa2
/echo -- -P0x
/def -mregexp -t'(a)(b)' -P0x pa3
/capl pa3
/capl -s pa3
/echo -- -P1Cred;1Cgreen
/def -mregexp -t'(a)(b)' -P1Cred;1Cgreen pa4
/capl pa4
/capl -s pa4
/echo -- -P2;1B
/def -mregexp -t'(a)(b)' -P2;1B pa5
/capl pa5
/capl -s pa5
/echo -- -P1B;;2u
/def -mregexp -t'(a)(b)' -P1B;;2u pa6
/capl pa6
/capl -s pa6
/echo -- -P1n
/def -mregexp -t'(a)(b)' -P1n pa7
/capl pa7
/capl -s pa7
/echo -- -P1xCblue
/def -mregexp -t'(a)(b)' -P1xCblue pa8
/capl pa8
/capl -s pa8
/echo -- -P3B
/def -mregexp -t'(a)(b)' -P3B pa9
/capl pa9
/capl -s pa9
/echo -- -PL
/def -mregexp -t'(a)(b)' -PL pa10
/capl pa10
/capl -s pa10
/echo -- -PR
/def -mregexp -t'(a)(b)' -PR pa11
/capl pa11
/capl -s pa11
/echo -- -PLu
/def -mregexp -t'(a)(b)' -PLu pa12
/capl pa12
/capl -s pa12
/echo -- -PRr
/def -mregexp -t'(a)(b)' -PRr pa13
/capl pa13
/capl -s pa13
/echo -- -P-1B
/def -mregexp -t'(a)(b)' -P-1B pa14
/capl pa14
/capl -s pa14
/echo -- -PxB
/def -mregexp -t'(a)(b)' -PxB pa15
/capl pa15
/capl -s pa15
/echo -- -P1B;2
/def -mregexp -t'(a)(b)' -P1B;2 pa16
/capl pa16
/capl -s pa16
/echo -- -P1Q
/def -mregexp -t'(a)(b)' -P1Q pa17
/capl pa17
/capl -s pa17
/echo -- -P conflicts
/def -mglob -t'q' -P1B pc1
/def -mregexp -P1B -mglob -t'q' pc2
/def -P1B -P2u -mregexp -t'(a)(b)' pc3
/def -P1B pc4
/def -mregexp -P1B pc5 = /echo x
/capl pc5
/capl -s pc5
/echo -- -A match attributes
/def -Ag -t'z' ma1
/capl ma1
/capl -s ma1
/echo -- /list -a and -P selection
/def -aB -t'sel1' sel1
/def -au -t'sel2' sel2
/def -aBu -t'sel3' sel3
/def -aCred -t'sel4' sel4
/def -aCbgred -t'sel5' sel5
/def -t'sel6' sel6
/echo -- list -aB sel1
/capl -s -aB sel1
/echo -- list -aB sel2
/capl -s -aB sel2
/echo -- list -aB sel3
/capl -s -aB sel3
/echo -- list -aB sel4
/capl -s -aB sel4
/echo -- list -aB sel5
/capl -s -aB sel5
/echo -- list -aB sel6
/capl -s -aB sel6
/echo -- list -au sel1
/capl -s -au sel1
/echo -- list -au sel2
/capl -s -au sel2
/echo -- list -au sel3
/capl -s -au sel3
/echo -- list -au sel4
/capl -s -au sel4
/echo -- list -au sel5
/capl -s -au sel5
/echo -- list -au sel6
/capl -s -au sel6
/echo -- list -aBu sel1
/capl -s -aBu sel1
/echo -- list -aBu sel2
/capl -s -aBu sel2
/echo -- list -aBu sel3
/capl -s -aBu sel3
/echo -- list -aBu sel4
/capl -s -aBu sel4
/echo -- list -aBu sel5
/capl -s -aBu sel5
/echo -- list -aBu sel6
/capl -s -aBu sel6
/echo -- list -aCred sel1
/capl -s -aCred sel1
/echo -- list -aCred sel2
/capl -s -aCred sel2
/echo -- list -aCred sel3
/capl -s -aCred sel3
/echo -- list -aCred sel4
/capl -s -aCred sel4
/echo -- list -aCred sel5
/capl -s -aCred sel5
/echo -- list -aCred sel6
/capl -s -aCred sel6
/echo -- list -aCblue sel1
/capl -s -aCblue sel1
/echo -- list -aCblue sel2
/capl -s -aCblue sel2
/echo -- list -aCblue sel3
/capl -s -aCblue sel3
/echo -- list -aCblue sel4
/capl -s -aCblue sel4
/echo -- list -aCblue sel5
/capl -s -aCblue sel5
/echo -- list -aCblue sel6
/capl -s -aCblue sel6
/echo -- list -aCbgred sel1
/capl -s -aCbgred sel1
/echo -- list -aCbgred sel2
/capl -s -aCbgred sel2
/echo -- list -aCbgred sel3
/capl -s -aCbgred sel3
/echo -- list -aCbgred sel4
/capl -s -aCbgred sel4
/echo -- list -aCbgred sel5
/capl -s -aCbgred sel5
/echo -- list -aCbgred sel6
/capl -s -aCbgred sel6
/echo -- list -an sel1
/capl -s -an sel1
/echo -- list -an sel2
/capl -s -an sel2
/echo -- list -an sel3
/capl -s -an sel3
/echo -- list -an sel4
/capl -s -an sel4
/echo -- list -an sel5
/capl -s -an sel5
/echo -- list -an sel6
/capl -s -an sel6
/echo -- list -ag sel1
/capl -s -ag sel1
/echo -- list -ag sel2
/capl -s -ag sel2
/echo -- list -ag sel3
/capl -s -ag sel3
/echo -- list -ag sel4
/capl -s -ag sel4
/echo -- list -ag sel5
/capl -s -ag sel5
/echo -- list -ag sel6
/capl -s -ag sel6
