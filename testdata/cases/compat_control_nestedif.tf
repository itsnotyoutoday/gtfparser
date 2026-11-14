/def n = /if ({1}) /if ({2}) /echo both%; /endif%; /else /echo outer false%; /endif
/n 0 1
/n 1 1
/n 1 0
/def w = /if ({1}) /if ({2}) /echo both%; /endif%; /else /echo outer false%; /endif
/def x = /echo before%; /if ({1}) %; /if ({2}) /echo A%; /else /echo B%; /endif%; /else /echo C%; /endif
/w 0 0
/x 1 0
/x 0 0
/def y = /if ({1}) /echo a%; /elseif ({2}) /if ({1}) /echo b%; /endif%; /endif%; /echo end
/y 0 1
/y 0 0
/def z = /let i=0%; /while (i < 2) /while (i < 5) /test ++i%; /done%; /done%; /echo %i
/z
