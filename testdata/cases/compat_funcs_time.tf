/def -i _b0 = /echo $[test("1+1")] $[test("x := 5")] $[x]
/_b0
/def -i _b1 = /echo $[tan("1:30")] $[tan("0x1f")] $[eval("1+1")] $[getopts("ab","")] $[filename("~") =~ "~"]
/_b1
