/if /true %; /then /echo T1 %; /endif
/if /false %; /then /echo F1 %; /else /echo F2 %; /endif
/_echo -x hello
/_echo   two  spaces
/: ignored
/if /: %; /then /echo colon %; /endif
/def -i chk = /if /true %; /then /echo in-macro-true %; /endif
/chk
