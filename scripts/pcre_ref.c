/* pcre_ref.c: run PCRE2 directly with TinyFugue's option set, for cases the tf
 * script interface cannot express (invalid UTF-8 subjects, start offsets, NOTBOL).
 *
 * stdin lines: pattern TAB subject-as-hex TAB start TAB notbol(0/1) TAB caseless(0/1)
 * stdout lines: "ERR msg" | "0" | space-separated ovector (start,end per group, -1 unset)
 *
 *   cc -O2 -I$PCRE2/include -L$PCRE2/lib -lpcre2-8 -o pcre_ref pcre_ref.c
 */
#define PCRE2_CODE_UNIT_WIDTH 8
#include <pcre2.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <locale.h>

int main(void) {
  char line[8192];
  /* like tf: locale-derived character tables (tf calls setlocale, then pcre2_maketables) */
  setlocale(LC_CTYPE, getenv("PCRE_REF_LOCALE") ? getenv("PCRE_REF_LOCALE") : "en_US.UTF-8");
  const unsigned char *tables = pcre2_maketables(NULL);
  pcre2_compile_context *cc = pcre2_compile_context_create(NULL);
  pcre2_set_character_tables(cc, tables);
  while (fgets(line, sizeof line, stdin)) {
    line[strcspn(line, "\n")] = 0;
    char *f[5]; int n = 0; char *p = line;
    f[n++] = p;
    while (n < 5 && (p = strchr(p, '\t'))) { *p++ = 0; f[n++] = p; }
    if (n < 5) { puts("BADLINE"); continue; }
    size_t hl = strlen(f[1]); unsigned char *subj = malloc(hl / 2 + 1); size_t sl = 0;
    for (size_t i = 0; i + 1 < hl; i += 2) { unsigned v; sscanf(f[1] + i, "%2x", &v); subj[sl++] = v; }
    uint32_t opt = PCRE2_DOLLAR_ENDONLY | PCRE2_DOTALL | PCRE2_UTF | PCRE2_UCP | PCRE2_MATCH_INVALID_UTF;
    if (atoi(f[4])) opt |= PCRE2_CASELESS;
    int ec; PCRE2_SIZE eo;
    pcre2_code *re = pcre2_compile((PCRE2_SPTR)f[0], PCRE2_ZERO_TERMINATED, opt, &ec, &eo, cc);
    if (!re) { PCRE2_UCHAR msg[256]; pcre2_get_error_message(ec, msg, sizeof msg); printf("ERR %d %s\n", (int)eo, msg); free(subj); continue; }
    pcre2_match_data *md = pcre2_match_data_create_from_pattern(re, NULL);
    int rc = pcre2_match(re, subj, sl, atoi(f[2]), atoi(f[3]) ? PCRE2_NOTBOL : 0, md, NULL);
    if (rc < 0) { if (rc == PCRE2_ERROR_NOMATCH) puts("0"); else printf("ERROR %d\n", rc); }
    else {
      PCRE2_SIZE *ov = pcre2_get_ovector_pointer(md); uint32_t c = pcre2_get_ovector_count(md);
      for (uint32_t i = 0; i < c; i++) {
        if (ov[2*i] == PCRE2_UNSET) printf("-1 -1"); else printf("%d %d", (int)ov[2*i], (int)ov[2*i+1]);
        putchar(i + 1 < c ? ' ' : '\n');
      }
    }
    pcre2_match_data_free(md); pcre2_code_free(re); free(subj);
  }
  return 0;
}
