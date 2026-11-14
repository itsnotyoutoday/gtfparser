/def ism = /if /ismacro %1%; /then /echo yes:%1%; /else /echo no:%1%; /endif
/def sv_wrap = /wrap %*%; /echo wrap=[%{wrap}]
/def sv_background = /background %*%; /echo background=[%{background}]
/def sv_bamf = /bamf %*%; /echo bamf=[%{bamf}]
/def sv_borg = /borg %*%; /echo borg=[%{borg}]
/def sv_insert = /insert %*%; /echo insert=[%{insert}]
/def sv_login = /login %*%; /echo login=[%{login}]
/def sv_lp = /lp %*%; /echo lp=[%{lp}]
/def sv_lpquote = /lpquote %*%; /echo lpquote=[%{lpquote}]
/def sv_quiet = /quiet %*%; /echo quiet=[%{quiet}]
/def sv_redef = /redef %*%; /echo redef=[%{redef}]
/def sv_shpause = /shpause %*%; /echo shpause=[%{shpause}]
/def sv_sockmload = /sockmload %*%; /echo sockmload=[%{sockmload}]
/def sv_sub = /sub %*%; /echo sub=[%{sub}]
/def sv_visual = /visual %*%; /echo visual=[%{visual}]
/def sv_gpri = /gpri %*%; /echo gpri=[%{gpri}]
/def sv_hpri = /hpri %*%; /echo hpri=[%{hpri}]
/def sv_isize = /isize %*%; /echo isize=[%{isize}]
/def sv_ptime = /ptime %*%; /echo ptime=[%{ptime}]
/def sv_wrapspace = /wrapspace %*%; /echo wrapspace=[%{wrapspace}]
/sv_wrap on
/sv_wrap off
/sv_wrap 1
/sv_wrap 0
/sv_wrap 5
/sv_wrap 
/sv_background on
/sv_background off
/sv_background 1
/sv_background 0
/sv_background 5
/sv_background 
/sv_bamf on
/sv_bamf off
/sv_bamf 1
/sv_bamf 0
/sv_bamf 5
/sv_bamf 
/sv_borg on
/sv_borg off
/sv_borg 1
/sv_borg 0
/sv_borg 5
/sv_borg 
/sv_insert on
/sv_insert off
/sv_insert 1
/sv_insert 0
/sv_insert 5
/sv_insert 
/sv_login on
/sv_login off
/sv_login 1
/sv_login 0
/sv_login 5
/sv_login 
/sv_lp on
/sv_lp off
/sv_lp 1
/sv_lp 0
/sv_lp 5
/sv_lp 
/sv_lpquote on
/sv_lpquote off
/sv_lpquote 1
/sv_lpquote 0
/sv_lpquote 5
/sv_lpquote 
/sv_quiet on
/sv_quiet off
/sv_quiet 1
/sv_quiet 0
/sv_quiet 5
/sv_quiet 
/sv_redef on
/sv_redef off
/sv_redef 1
/sv_redef 0
/sv_redef 5
/sv_redef 
/sv_shpause on
/sv_shpause off
/sv_shpause 1
/sv_shpause 0
/sv_shpause 5
/sv_shpause 
/sv_sockmload on
/sv_sockmload off
/sv_sockmload 1
/sv_sockmload 0
/sv_sockmload 5
/sv_sockmload 
/sv_sub on
/sv_sub off
/sv_sub 1
/sv_sub 0
/sv_sub 5
/sv_sub 
/sv_visual on
/sv_visual off
/sv_visual 1
/sv_visual 0
/sv_visual 5
/sv_visual 
/sv_gpri 1
/sv_gpri 0
/sv_gpri 5
/sv_hpri 1
/sv_hpri 0
/sv_hpri 5
/sv_isize 1
/sv_isize 0
/sv_isize 5
/sv_ptime on
/sv_ptime off
/sv_ptime 1
/sv_ptime 0
/sv_ptime 5
/sv_ptime 
/sv_wrapspace on
/sv_wrapspace off
/sv_wrapspace 1
/sv_wrapspace 0
/sv_wrapspace 5
/sv_wrapspace 
/wrap 80
/test E(strcat(wrap, "|", wrapsize))
/wrap off
/test E(strcat(wrap, "|", wrapsize))
/wrap 100
/test E(strcat(wrap, "|", wrapsize))
/def pfx = /%1 %-1%; /echo [%{$1echo}][%{$1prefix}]
/set qecho=0
/qecho on
/test E(qecho)
/qecho off
/test E(qecho)
/qecho all
/test E(qecho)
/kecho off
/test E(kecho)
/kecho on
/test E(kecho)
/kecho all
/test E(kecho)
/mecho all
/test E(mecho)
/mecho 2
/test E(mecho)
/mecho on
/test E(mecho)
/qecho >>
/test E(strcat(qecho, "|", qprefix))
/kecho >>>
/test E(strcat(kecho, "|", kprefix))
