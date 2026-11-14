/def -i _b0 = /echo $[tolower("ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÐÑÒÓÔÕÖØÙÚÛÜÝÞß")]
/_b0
/def -i _b1 = /echo $[toupper("àáâãäåæçèéêëìíîïðñòóôõöøùúûüýþÿµ")]
/_b1
/def -i _b2 = /echo $[ascii("é")] $[ascii("")] $[ascii(char(233))] $[strlen(char(233))] $[char(10000)] $[char(65)] $[char(321)]
/_b2
/def -i _b3 = /echo $[strlen("日本語")] $[substr("日本語",1,1)] $[strstr("日本語","本")] $[pad("日本",5,"x",-3)]
/_b3
