package dotcontext

import "testing"

func TestDotString(t *testing.T) {
	d := Dot{ID: "a", Seq: 3}
	if got := d.String(); got != "a:3" {
		t.Errorf("Dot.String() = %q, want %q", got, "a:3")
	}
}
