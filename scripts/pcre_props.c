/* pcre_props.c: dump the code point ranges of Unicode properties as PCRE2
 * knows them (UTF+UCP).  Reads one property name per line on stdin and prints
 * "=name" followed by "lo hi" (hex) lines, or "!name" if PCRE2 rejects it.
 * Used by pcre_genprops.py to generate pcre/props_gen.go.
 *
 *   cc -O2 -I$PCRE2/include -L$PCRE2/lib -lpcre2-8 -o pcre_props pcre_props.c
 */
#define PCRE2_CODE_UNIT_WIDTH 8
#include <pcre2.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
static int enc(unsigned c, char *o){
  if(c<0x80){o[0]=c;return 1;}
  if(c<0x800){o[0]=0xc0|c>>6;o[1]=0x80|(c&63);return 2;}
  if(c<0x10000){o[0]=0xe0|c>>12;o[1]=0x80|((c>>6)&63);o[2]=0x80|(c&63);return 3;}
  o[0]=0xf0|c>>18;o[1]=0x80|((c>>12)&63);o[2]=0x80|((c>>6)&63);o[3]=0x80|(c&63);return 4;}
int main(int argc,char**argv){
  /* subject: every scalar value; offsets map back via table */
  char *buf=malloc(5*0x110000); unsigned *cp=malloc(4*(5*0x110000)); size_t n=0;
  for(unsigned c=0;c<0x110000;c++){ if(c>=0xd800&&c<=0xdfff)continue; int w=enc(c,buf+n); for(int k=0;k<w;k++)cp[n+k]=c; n+=w;}
  char line[256];
  while(fgets(line,sizeof line,stdin)){
    line[strcspn(line,"\n")]=0; if(!*line)continue;
    char pat[300]; snprintf(pat,sizeof pat,"\\p{%s}+",line);
    int ec; PCRE2_SIZE eo;
    pcre2_code*re=pcre2_compile((PCRE2_SPTR)pat,PCRE2_ZERO_TERMINATED,PCRE2_UTF|PCRE2_UCP,&ec,&eo,NULL);
    if(!re){printf("!%s\n",line);continue;}
    pcre2_match_data*md=pcre2_match_data_create_from_pattern(re,NULL);
    size_t off=0; printf("=%s\n",line);
    while(off<=n){
      int rc=pcre2_match(re,(PCRE2_SPTR)buf,n,off,0,md,NULL);
      if(rc<0)break;
      PCRE2_SIZE*ov=pcre2_get_ovector_pointer(md);
      unsigned lo=cp[ov[0]],hi=cp[ov[1]-1];
      printf("%x %x\n",lo,hi);
      off=ov[1];
    }
  }
}
