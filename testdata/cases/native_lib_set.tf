; library commands that wrap /set
/def -i T = /eval %*
/set lp=0
/lp on
/T /echo lp=%lp
/lp off
/T /echo lp=%lp
/lp 1
/T /echo lp=%lp
/set borg=0
/borg on
/T /echo borg=%borg
/reply off
/T /echo reply-sets-borg=%borg
/set quiet=0
/quiet on
/T /echo quiet=%quiet
/noquiet
/T /echo noquiet=%quiet
/set login=0
/login 1
/T /echo login=%login
/nologin
/T /echo nologin=%login
/nowrap
/T /echo nowrap=%wrap
/wrap 40
/T /echo wrap=%wrap size=%wrapsize
/wrap off
/T /echo wrap=%wrap
/wrap on
/T /echo wrap=%wrap
/wrap 0x
/T /echo wrap=%wrap size=%wrapsize
/set insert=0
/insert on
/T /echo insert=%insert
/set bamf=0
/bamf on
/T /echo bamf=%bamf
/bamf 2
/T /echo bamf=%bamf
/bamf off
/set background=0
/background on
/T /echo background=%background
/clearfull on
/T /echo clearfull=%clearfull
/cleardone on
/T /echo cleardone=%cleardone
/quitdone on
/T /echo quitdone=%quitdone
/sockmload on
/T /echo sockmload=%sockmload
/sub full
/T /echo sub=%sub
/sub off
/T /echo sub=%sub
/isize 3
/T /echo isize=%isize
/ptime 5
/T /echo ptime=%ptime
/wrapspace 4
/T /echo wrapspace=%wrapspace
/redef off
/T /echo redef=%redef
/redef on
/T /echo redef=%redef
/set more=0
/more off
/T /echo more=%more
/more on
/T /echo more=%more
/more off
; a typed /set with no value lists; a macro that passes an empty %* sets the value empty
/def -i mylp = /lp %*
/set lp=1
/mylp
/T /echo lp-after-empty=[%lp]
/def -i eb = /set borg %*
/set borg=1
/eb
/T /echo borg-after-empty=[%borg]
/T /echo [$(/borg)]
/T /echo borg-after-bare=[%borg]
; act is /trig
/act foo bar
/T /echo act=[$(/listtrig)]
; a user macro shadows the library command
/def -i lp = /echo shadow-lp %*
/lp on
/@lp on
/T /echo lp=%lp
