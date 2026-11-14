/def sf = /test E(strcat("sf%{1-0}=[", status_fields({1-0}), "]"))
/def sw = /test E(strcat("w ", {1}, "=", status_width({1})))
/sf
/sf 1
/sf 9
/sw @more
/test E(strcat("sfv=", status_fields))
/test E(strcat("h=", status_height))
/status_add -c foo:5:Br bar:-3 "lit"
/sf
/status_add -r1 baz:4:Cred
/sf 1
/set status_height=2
/status_add -r1 baz:4:Cred
/sf 1
/test E(strcat("sfv=", status_fields))
/status_add -c -B @clock:5 :2 @world
/sf
/status_edit @world:4:B
/sf
/status_rm @world
/sf
/status_add -x @clock:7
/sf
/status_add -c x:3:BUr y:4:Cred
/sf
/status_add -c x:3:Bu y:4:Ccyan:u
/sf
/status_add -c x:3:rBu y:4:Ccyan z:-0:Bg
/sf
/status_add -c x:3 y:0 w:7 
/sf
/sf
/status_add -c @more:8:Br :1 @world :1 @read:6 :1 @active:11 :1 @log:5 :1 @mail:6 :1 insert:6 :1 kbnum:4 :1 @clock:5
/sf
/status_rm @mail
/sf
/status_rm @more
/sf
/status_rm @clock
/sf
/status_rm @world
/sf
/status_clear -r0
/sf
/status_add foo:3
/status_add -A foo:3
/sf
/status_add -B bar
/sf
/status_add -s3 -Afoo zed:4
/sf
/status_add -s0 -Bfoo zz:4
/sf
/echo --- return values
/status_add -c a:3 b:4
/status_add -Anone zz
/status_add -x a:9
/status_rm nosuch
/status_rm a
/status_edit nosuch:3
/status_edit b:7:B
/status_clear -r0
/status_clear 
/status_add @bogus
/status_add -c x:1 y:0 z:0
/status_add -c x:1 y:0
/sf
/echo --- variable
/set warn_status=off
/set status_fields=aa:3 bb :2 "cc"
/sf
/test E(strcat("v=", status_fields))
/status_add -c dd:4
/test E(strcat("v=", status_fields))
/set status_fields=@bogus
/sf
/test E(strcat("v=", status_fields))
/set status_fields=
/sf
/echo --- height and rows
/set status_height=3
/status_add -r0 r0a:2
/status_add -r1 r1a:2 :1 r1b:3
/status_add -r2 r2a:5:Cred,Cbgblue
/sf 0
/sf 1
/sf 2
/status_rm -r1 r1a
/sf 1
/status_edit -r2 r2a:6:Bu
/sf 2
/status_rm -r0 r2a
/set status_height=1
/status_add -c "a b":0 'it''s'
/sf
/status_add -c "a b" "it's:9"
/sf
/status_add -c `q`:3
/sf
/status_add -c "x\"y\\z":5
/sf
/echo --- attrs
/status_add -c a:1:Bu b:1:uB c:1:Cgreen d:1:Cbgred e:1:CredCbgblue f:1:Cbgblue,Cred g:1:C9 h:1:Cgray12 i:1:Crgb123 j:1:n k:1:xBr l:1:G m:1:rufdibBh
/sf
/status_add -c a:1:E
/sf
/status_add -c a:-1 b:-0 c:--2 d:---3
/sf
/echo --- defaults
/status_defaults
/sf
/status_save mine
/status_add -c q:3
/sf
/status_restore mine
/sf
