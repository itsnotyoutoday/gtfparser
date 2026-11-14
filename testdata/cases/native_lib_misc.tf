; /prompt, /substitute, /more, /redef and a user macro shadowing a library command
/def -i T = /eval %*
/prompt abc
/T /echo prompt=%?
/prompt -a
/T /echo prompt-a=%?
/prompt -p -aBCred @{B}hi
/T /echo prompt-p=%?
/substitute hello
/T /echo substitute=%?
/substitute -a -p x
/T /echo substitute-ap=%?
/T /echo fn=[$[prompt("x")]] [$[substitute("y")]]
/def -i first = /result {2}
/T /echo shadowed=[$(/first a b c)] [$[first("a", "b")]]
/undef first
/T /echo undefed=[$(/first a b c)]
/def -i first = /result {1}
/T /echo redefined-same=[$(/first a b c)]
/def -i rest = /result {-2}
/T /echo shadow-rest=[$(/rest a b c)]
/undef rest
/T /echo ismacro-rest=[$[ismacro("rest")]]
/T /echo escape-fn=[$[escape("'", "a'b")]]
/def -mglob -t'x*' tr1 = /echo trig %*
/T /echo list-has-tr1=[$(/listtrig)]
/undef tr1
