/set g=global
/def s1 = /let g=local%; /echo %g%; /s2
/def s2 = /echo %g %{g}
/s1
/echo %g
/def s3 = /test g := "changed"
/s3
/echo %g
/def s4 = /let z=1%; /s5 %z
/def s5 = /echo got %1 %{z-unset}
/s4
/def s6 = /test new1 := "n"
/s6
/echo %new1
/def lp = /let t=0%; /for i 1 3 /test t := t + %%i%; /echo t=%t
/lp
