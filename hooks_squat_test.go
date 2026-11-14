package gtfparser

import "testing"

func TestSquatHooksOptional(t *testing.T) {
	in := New(nil)
	var out []string
	in.Warn = func(s string) { out = append(out, s) }
	in.Exec("/def -hSQUAT_PAIR sp = /set squat_seen=%{*}")
	if in.Var("squat_seen") != "" {
		t.Fatal("hook defined before it was enabled ran")
	}
	if err := in.EnableHookEvents("SQUAT_CONNECT", "SQUAT_DISCONNECT", "SQUAT_PAIR"); err != nil {
		t.Fatal(err)
	}
	in.Exec("/def -hSQUAT_PAIR sp = /set squat_seen=%{*}")
	in.FireHookArgs("SQUAT_PAIR", "phone", "operator")
	if got := in.Var("squat_seen"); got != "phone operator" {
		t.Fatalf("squat_seen = %q (warnings %v)", got, out)
	}
}
