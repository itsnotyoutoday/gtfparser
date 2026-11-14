/require textutil.tf

/def mkfile = /test h := tfopen("/tmp/gtf_c2_a.txt", "w")%; /test tfwrite(h, "alpha")%; /test tfwrite(h, "beta gamma")%; /test tfwrite(h, "")%; /test tfwrite(h, "alpha")%; /test tfwrite(h, "alpha")%; /test tfwrite(h, "Delta  epsilon")%; /test tfwrite(h, "  x  ")%; /test tfwrite(h, "9")%; /test tfclose(h)
/mkfile
/def opn = /test h := tfopen("/tmp/gtf_c2_a.txt", "r")
/def cls = /test tfclose(h)
/def r_readfile = /echo [$(/readfile %*)]
/r_readfile /tmp/gtf_c2_a.txt
/r_readfile /tmp/gtf_c2_nonexistent
/def r_head = /opn%; /echo [$(/head %* %h)]%; /cls
/r_head
/r_head -n1
/r_head -n3
/r_head -n0
/r_head -n100
/def r_wc = /opn%; /wc %* %h%; /cls
/r_wc
/r_wc -l
/r_wc -w
/r_wc -c
/r_wc -lw
/r_wc -wc
/r_wc -lwc
/def r_copyio = /opn%; /echo [$(/copyio %h)]%; /cls
/r_copyio
/def r_randline = /opn%; /let r=$(/randline %h)%; /cls%; /echo got:%r
/def wf = /test E(wf_x)
/test _o2 := tfopen("/tmp/gtf_c2_b.txt", "w")
/test tfwrite(_o2, "one")
/test tfwrite(_o2, "two")
/test tfclose(_o2)
/test E(tfopen("/tmp/gtf_c2_b.txt", "r") >= 0)
/def cnt = /let n=0%; /let l=%; /test h:=tfopen("/tmp/gtf_c2_b.txt","r")%; /while (tfread(h, l) >= 0) /test ++n%; /echo line:%l%; /done%; /test tfclose(h)%; /echo n=%n
/cnt
