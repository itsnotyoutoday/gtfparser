/def -i _b0 = /test q := tfopen()%;/test tfwrite(q, 'alpha')%;/test tfwrite(q, 'beta')%;/echo [$[tfreadable(q)]] [$[tfread(q, a)]] [%a] [$[tfread(q, a)]] [%a] [$[tfread(q, a)]]%;/echo [$[tfclose(q)]] [$[tfclose(12345)]] [$[tfflush(12345)]] [$[tfreadable("o")]] [$[tfopen("x")]] [$[tfclose("e")]]
/_b0
/def -i _b1 = /test p := tfopen("echo hi; echo there", "p")%;/echo [$[tfread(p, v)]] [%v] [$[tfread(p, v)]] [%v] [$[tfread(p, v)]] [$[tfclose(p)]]%;/test p := tfopen("exit 3", "p")%;/echo [$[tfclose(p)]]
/_b1
/def -i _b2 = /test w := tfopen("/tmp/gtf_compat_streams.txt", "w")%;/echo [$[tfwrite(w, "a")]] [$[tfwrite(w, "b c")]] [$[tfreadable(w)]] [$[tfread(w, x)]] [$[tfflush(w, "off")]] [$[tfclose(w)]] [$[tfwrite(w, "z")]]%;/test r := tfopen("/tmp/gtf_compat_streams.txt", "r")%;/echo [$[tfread(r, x)]] [%x] [$[tfwrite(r, "q")]] [$[tfread(r, x)]] [%x] [$[tfread(r, x)]] [$[tfreadable(r)]] [$[tfclose(r)]]
/_b2
/def -i _b3 = /echo [$[fwrite("/tmp/gtf_compat_fw.txt","one")]] [$[fwrite("/tmp/gtf_compat_fw.txt","two")]]%;/test r := tfopen("/tmp/gtf_compat_fw.txt", "r")%;/echo [$[tfread(r, x)]][$[tfread(r, y)]] %x %y [$[tfclose(r)]]
/_b3
