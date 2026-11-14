package gtfparser

import (
	"math"
	"strings"
)

// funcArity is the argument-count range tf enforces for each builtin
// function when an expression is compiled (funclist.h); -1 is unbounded.
var funcArity = map[string][2]int{
	"abs":           {1, 1},
	"acos":          {1, 1},
	"addworld":      {2, 9},
	"ascii":         {1, 1},
	"asin":          {1, 1},
	"atan":          {1, 1},
	"atcp":          {1, 2},
	"calllua":       {1, -1},
	"char":          {1, 1},
	"columns":       {0, 0},
	"cos":           {1, 1},
	"cputime":       {0, 0},
	"decode_ansi":   {1, 1},
	"decode_attr":   {1, 3},
	"echo":          {1, 4},
	"encode_ansi":   {1, 1},
	"encode_attr":   {1, 1},
	"eval":          {1, 2},
	"exp":           {1, 1},
	"fake_recv":     {1, 3},
	"fg_world":      {0, 0},
	"filename":      {1, 1},
	"ftime":         {0, 2},
	"fwrite":        {2, 2},
	"gethostname":   {0, 0},
	"getopts":       {1, 2},
	"getpid":        {0, 0},
	"gmcp":          {1, 2},
	"idle":          {0, 1},
	"is_connected":  {0, 1},
	"is_open":       {0, 1},
	"isatty":        {0, 0},
	"kbcharback":    {1, 1},
	"kbcharfwd":     {1, 1},
	"kbdel":         {1, 1},
	"kbgoto":        {1, 1},
	"kbhead":        {0, 0},
	"kblen":         {0, 0},
	"kbmatch":       {0, 1},
	"kbpoint":       {0, 0},
	"kbtail":        {0, 0},
	"kbwordleft":    {0, 1},
	"kbwordright":   {0, 1},
	"keycode":       {1, 1},
	"lines":         {0, 0},
	"ln":            {1, 1},
	"log10":         {1, 1},
	"mktime":        {1, 7},
	"mod":           {2, 2},
	"morepaused":    {0, 1},
	"morescroll":    {1, 1},
	"moresize":      {0, 2},
	"msdp":          {1, 2},
	"nactive":       {0, 1},
	"nlog":          {0, 0},
	"nmail":         {0, 0},
	"nread":         {0, 0},
	"option102":     {1, 2},
	"pad":           {1, -1},
	"pow":           {2, 2},
	"prompt":        {1, 1},
	"python":        {1, 1},
	"rand":          {0, 2},
	"read":          {0, 0},
	"regmatch":      {2, 2},
	"replace":       {3, 3},
	"send":          {1, 3},
	"sidle":         {0, 1},
	"sin":           {1, 1},
	"sqrt":          {1, 1},
	"status_fields": {0, 1},
	"status_width":  {1, 1},
	"strcat":        {1, -1},
	"strchr":        {2, 3},
	"strcmp":        {2, 2},
	"strcmpattr":    {2, 2},
	"strip_attr":    {1, 1},
	"strlen":        {1, 1},
	"strncmp":       {3, 3},
	"strrchr":       {2, 3},
	"strrep":        {2, 2},
	"strstr":        {2, 3},
	"substitute":    {1, 3},
	"substr":        {2, 3},
	"systype":       {0, 0},
	"tan":           {1, 1},
	"test":          {1, 1},
	"tfclose":       {1, 1},
	"tfflush":       {1, 2},
	"tfopen":        {0, 2},
	"tfread":        {1, 2},
	"tfreadable":    {1, 1},
	"tfwrite":       {1, 2},
	"time":          {0, 0},
	"tolower":       {1, 2},
	"toupper":       {1, 2},
	"trunc":         {1, 1},
	"whatis":        {1, 1},
	"winlines":      {0, 0},
	"world_info":    {0, 2},
}

var tfCmdSet = func() map[string]bool {
	m := make(map[string]bool, len(tfCommands))
	for _, n := range tfCommands {
		m[n] = true
	}
	return m
}()

// isTfCommand reports whether name is one of real tf's builtin commands.
func (in *Interp) isTfCommand(name string) bool { return tfCmdSet[strings.ToLower(name)] }

// cAtoi is C's atoi: leading blanks, a sign, then digits; anything else is 0.
func cAtoi(s string) int {
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	i, neg := 0, false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	var n int64 // wide, so the saturation at 2^31 below fits where int is 32 bits
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int64(s[i]-'0')
		if n > 1<<31 {
			n = 1 << 31
		}
	}
	if neg {
		n = -n
	}
	if n > math.MaxInt { // 32-bit platforms
		n = math.MaxInt
	}
	return int(n)
}

// cmdFuncName is the lower-case name of the tf builtin command that name
// (as written in a function call) refers to, or "".
func cmdFuncName(name string) string {
	if tfCmdSet[name] {
		return name
	}
	for i := 0; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' {
			if ln := strings.ToLower(name); tfCmdSet[ln] {
				return ln
			}
			break
		}
	}
	return ""
}

var upperNames = map[string]string{}

func init() {
	for n := range tfCmdSet {
		upperNames[n] = strings.ToUpper(n)
	}
}

// upperCmd is the upper-case spelling of a builtin command name (tf's cmd->name).
func upperCmd(name string) string {
	if u, ok := upperNames[name]; ok {
		return u
	}
	return strings.ToUpper(name)
}
