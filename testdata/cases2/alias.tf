/require alias.tf
/alias hi /echo hello %*
/def ism = /if /ismacro %1%; /then /echo yes:%1%; /else /echo no:%1%; /endif
/ism ~alias_body_hi
/ism ~alias_call_hi
/ism ~alias_body_nope
/~alias_body_hi a b c
/~alias_body_hi
/alias two /echo [%1] [%2] [%{*}]
/~alias_body_two x y z
/~alias_body_two x
/alias nested /hi nested %*
/~alias_body_nested q r
/def lal = /let r=$(/~listalias ~alias_body_hi)%; /echo list:%r
/unalias hi
/ism ~alias_body_hi
/ism ~alias_call_hi
/unalias hi
/unalias nosuch
/purgealias
/ism ~alias_body_two
/ism ~alias_call_two
/alias one /echo only one
/alias
