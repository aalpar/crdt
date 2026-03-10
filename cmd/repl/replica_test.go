package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// --- Replica interface tests ---

func TestNewReplica_AllTypes(t *testing.T) {
	for _, kind := range typeNames {
		r, err := newReplica(kind, "r1")
		if err != nil {
			t.Errorf("newReplica(%q): %v", kind, err)
			continue
		}
		if r.Type() != kind {
			t.Errorf("Type() = %q, want %q", r.Type(), kind)
		}
		if r.Ops() == "" {
			t.Errorf("%s: Ops() is empty", kind)
		}
	}
}

func TestNewReplica_UnknownType(t *testing.T) {
	_, err := newReplica("bogus", "r1")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention type name, got: %v", err)
	}
}

func TestAWSet_DoAndShow(t *testing.T) {
	r := mustNew(t, "awset", "alice")

	mustDo(t, r, "add", "x")
	mustDo(t, r, "add", "y")
	assertShow(t, r, "{x, y}")

	mustDo(t, r, "remove", "x")
	assertShow(t, r, "{y}")
}

func TestRWSet_DoAndShow(t *testing.T) {
	r := mustNew(t, "rwset", "alice")

	mustDo(t, r, "add", "a")
	mustDo(t, r, "add", "b")
	assertShow(t, r, "{a, b}")

	mustDo(t, r, "remove", "a")
	assertShow(t, r, "{b}")
}

func TestPNCounter_DoAndShow(t *testing.T) {
	r := mustNew(t, "pncounter", "alice")

	mustDo(t, r, "inc", "5")
	assertShow(t, r, "5")

	mustDo(t, r, "dec", "3")
	assertShow(t, r, "2")
}

func TestGCounter_DoAndShow(t *testing.T) {
	r := mustNew(t, "gcounter", "alice")

	mustDo(t, r, "inc", "7")
	assertShow(t, r, "7")

	mustDo(t, r, "inc", "3")
	assertShow(t, r, "10")
}

func TestLWWRegister_DoAndShow(t *testing.T) {
	r := mustNew(t, "lwwregister", "alice")
	assertShow(t, r, "<empty>")

	mustDo(t, r, "set", "hello", "10")
	assertShow(t, r, `"hello" (ts=10)`)
}

func TestEWFlag_DoAndShow(t *testing.T) {
	r := mustNew(t, "ewflag", "alice")
	assertShow(t, r, "false")

	mustDo(t, r, "enable")
	assertShow(t, r, "true")

	mustDo(t, r, "disable")
	assertShow(t, r, "false")
}

func TestDWFlag_DoAndShow(t *testing.T) {
	r := mustNew(t, "dwflag", "alice")
	assertShow(t, r, "true")

	mustDo(t, r, "disable")
	assertShow(t, r, "false")

	mustDo(t, r, "enable")
	assertShow(t, r, "true")
}

func TestMVRegister_DoAndShow(t *testing.T) {
	r := mustNew(t, "mvregister", "alice")
	assertShow(t, r, "<empty>")

	mustDo(t, r, "write", "v1")
	assertShow(t, r, `["v1"]`)
}

func TestRGA_DoAndShow(t *testing.T) {
	r := mustNew(t, "rga", "alice")
	assertShow(t, r, "[]")

	mustDo(t, r, "insert", "0", "a")
	mustDo(t, r, "insert", "1", "b")
	mustDo(t, r, "insert", "2", "c")
	assertShow(t, r, "[a, b, c]")

	mustDo(t, r, "delete", "1")
	assertShow(t, r, "[a, c]")
}

// --- Sync tests ---

func TestAWSet_Sync(t *testing.T) {
	a := mustNew(t, "awset", "alice")
	b := mustNew(t, "awset", "bob")

	mustDo(t, a, "add", "x")
	mustDo(t, a, "add", "y")
	mustDo(t, b, "add", "y")
	mustDo(t, b, "add", "z")

	mustSync(t, a, b)
	assertShow(t, a, "{x, y, z}")
	assertShow(t, b, "{x, y, z}")
}

func TestAWSet_SyncConflict(t *testing.T) {
	a := mustNew(t, "awset", "alice")
	b := mustNew(t, "awset", "bob")

	// Both observe "x".
	mustDo(t, a, "add", "x")
	mustSync(t, a, b)

	// Concurrent: Alice re-adds "x", Bob removes "x".
	mustDo(t, a, "add", "x")
	mustDo(t, b, "remove", "x")
	mustSync(t, a, b)

	// Add-wins: "x" survives.
	assertShow(t, a, "{x}")
	assertShow(t, b, "{x}")
}

func TestRWSet_SyncConflict(t *testing.T) {
	a := mustNew(t, "rwset", "alice")
	b := mustNew(t, "rwset", "bob")

	// Both observe "x".
	mustDo(t, a, "add", "x")
	mustSync(t, a, b)

	// Concurrent: Alice re-adds "x", Bob removes "x".
	mustDo(t, a, "add", "x")
	mustDo(t, b, "remove", "x")
	mustSync(t, a, b)

	// Remove-wins: "x" is gone.
	assertShow(t, a, "{}")
	assertShow(t, b, "{}")
}

func TestPNCounter_Sync(t *testing.T) {
	a := mustNew(t, "pncounter", "alice")
	b := mustNew(t, "pncounter", "bob")

	mustDo(t, a, "inc", "10")
	mustDo(t, b, "inc", "5")
	mustDo(t, b, "dec", "2")

	mustSync(t, a, b)
	assertShow(t, a, "13")
	assertShow(t, b, "13")
}

func TestEWFlag_Sync(t *testing.T) {
	a := mustNew(t, "ewflag", "alice")
	b := mustNew(t, "ewflag", "bob")

	mustDo(t, a, "enable")
	mustDo(t, b, "disable")

	mustSync(t, a, b)
	// enable-wins: concurrent enable+disable → enabled
	assertShow(t, a, "true")
	assertShow(t, b, "true")
}

func TestDWFlag_Sync(t *testing.T) {
	a := mustNew(t, "dwflag", "alice")
	b := mustNew(t, "dwflag", "bob")

	mustDo(t, a, "enable")
	mustDo(t, b, "disable")

	mustSync(t, a, b)
	// disable-wins: concurrent enable+disable → disabled
	assertShow(t, a, "false")
	assertShow(t, b, "false")
}

func TestSync_TypeMismatch(t *testing.T) {
	a := mustNew(t, "awset", "alice")
	b := mustNew(t, "pncounter", "bob")

	if err := a.Sync(b); err == nil {
		t.Fatal("expected type mismatch error")
	}
}

// --- Do error cases ---

func TestDo_UnknownOp(t *testing.T) {
	for _, kind := range typeNames {
		r := mustNew(t, kind, "r1")
		_, err := r.Do("bogus", nil)
		if err == nil {
			t.Errorf("%s: expected error for unknown op", kind)
		}
	}
}

func TestDo_MissingArgs(t *testing.T) {
	tests := []struct {
		kind string
		op   string
	}{
		{"awset", "add"},
		{"awset", "remove"},
		{"rwset", "add"},
		{"rwset", "remove"},
		{"pncounter", "inc"},
		{"pncounter", "dec"},
		{"gcounter", "inc"},
		{"lwwregister", "set"},
		{"mvregister", "write"},
		{"rga", "insert"},
		{"rga", "delete"},
	}
	for _, tt := range tests {
		r := mustNew(t, tt.kind, "r1")
		_, err := r.Do(tt.op, nil)
		if err == nil {
			t.Errorf("%s %s: expected error for missing args", tt.kind, tt.op)
		}
	}
}

// --- Command dispatch tests (doNew, doShow, doSync, doOp, doList) ---

func TestDoNew_And_DoShow(t *testing.T) {
	resetReplicas(t)

	out := captureStdout(t, func() { doNew([]string{"awset", "alice"}) })
	if !strings.Contains(out, "created") {
		t.Errorf("doNew output = %q, want 'created'", out)
	}

	out = captureStdout(t, func() { doShow([]string{"alice"}) })
	if !strings.Contains(out, "{}") {
		t.Errorf("doShow output = %q, want '{}'", out)
	}
}

func TestDoNew_Duplicate(t *testing.T) {
	resetReplicas(t)

	captureStdout(t, func() { doNew([]string{"awset", "alice"}) })
	out := captureStdout(t, func() { doNew([]string{"awset", "alice"}) })
	if !strings.Contains(out, "already exists") {
		t.Errorf("doNew duplicate output = %q, want 'already exists'", out)
	}
}

func TestDoNew_MissingArgs(t *testing.T) {
	resetReplicas(t)

	out := captureStdout(t, func() { doNew(nil) })
	if !strings.Contains(out, "usage") {
		t.Errorf("doNew missing args output = %q, want 'usage'", out)
	}
}

func TestDoShow_UnknownReplica(t *testing.T) {
	resetReplicas(t)

	out := captureStdout(t, func() { doShow([]string{"nobody"}) })
	if !strings.Contains(out, "unknown replica") {
		t.Errorf("doShow output = %q, want 'unknown replica'", out)
	}
}

func TestDoSync_Integration(t *testing.T) {
	resetReplicas(t)

	captureStdout(t, func() {
		doNew([]string{"pncounter", "alice"})
		doNew([]string{"pncounter", "bob"})
		doOp("alice", []string{"inc", "10"})
		doOp("bob", []string{"inc", "5"})
		doSync([]string{"alice", "bob"})
	})

	out := captureStdout(t, func() { doShow([]string{"alice"}) })
	if !strings.Contains(out, "15") {
		t.Errorf("after sync alice = %q, want 15", out)
	}
	out = captureStdout(t, func() { doShow([]string{"bob"}) })
	if !strings.Contains(out, "15") {
		t.Errorf("after sync bob = %q, want 15", out)
	}
}

func TestDoOp_UnknownReplica(t *testing.T) {
	resetReplicas(t)

	out := captureStdout(t, func() { doOp("nobody", []string{"add", "x"}) })
	if !strings.Contains(out, "unknown command or replica") {
		t.Errorf("doOp output = %q, want 'unknown command or replica'", out)
	}
}

func TestDoOp_MissingOp(t *testing.T) {
	resetReplicas(t)

	captureStdout(t, func() { doNew([]string{"awset", "alice"}) })
	out := captureStdout(t, func() { doOp("alice", nil) })
	if !strings.Contains(out, "usage") {
		t.Errorf("doOp missing op output = %q, want 'usage'", out)
	}
}

func TestDoList_Empty(t *testing.T) {
	resetReplicas(t)

	out := captureStdout(t, func() { doList() })
	if !strings.Contains(out, "no replicas") {
		t.Errorf("doList output = %q, want 'no replicas'", out)
	}
}

func TestDoList_WithReplicas(t *testing.T) {
	resetReplicas(t)

	captureStdout(t, func() {
		doNew([]string{"awset", "alice"})
		doNew([]string{"pncounter", "bob"})
	})

	out := captureStdout(t, func() { doList() })
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Errorf("doList output = %q, want both alice and bob", out)
	}
}

// --- helpers ---

func mustNew(t *testing.T, kind, id string) Replica {
	t.Helper()
	r, err := newReplica(kind, id)
	if err != nil {
		t.Fatalf("newReplica(%q, %q): %v", kind, id, err)
	}
	return r
}

func mustDo(t *testing.T, r Replica, op string, args ...string) string {
	t.Helper()
	result, err := r.Do(op, args)
	if err != nil {
		t.Fatalf("Do(%q, %v): %v", op, args, err)
	}
	return result
}

func mustSync(t *testing.T, a, b Replica) {
	t.Helper()
	if err := a.Sync(b); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func assertShow(t *testing.T, r Replica, want string) {
	t.Helper()
	got := r.Show()
	if got != want {
		t.Errorf("Show() = %q, want %q", got, want)
	}
}

func resetReplicas(t *testing.T) {
	t.Helper()
	replicas = map[string]Replica{}
	t.Cleanup(func() { replicas = map[string]Replica{} })
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	r.Close()
	return buf.String()
}

// requireInt64 and requireUint64 are small helpers but let's verify edge cases.
func TestRequireInt64(t *testing.T) {
	_, err := requireInt64(nil)
	if err == nil {
		t.Error("expected error for empty args")
	}

	n, err := requireInt64([]string{"-42"})
	if err != nil || n != -42 {
		t.Errorf("requireInt64(-42) = %d, %v", n, err)
	}

	_, err = requireInt64([]string{"abc"})
	if err == nil {
		t.Error("expected error for non-numeric")
	}
}

func TestRequireUint64(t *testing.T) {
	_, err := requireUint64(nil)
	if err == nil {
		t.Error("expected error for empty args")
	}

	n, err := requireUint64([]string{"42"})
	if err != nil || n != 42 {
		t.Errorf("requireUint64(42) = %d, %v", n, err)
	}

	_, err = requireUint64([]string{"-1"})
	if err == nil {
		t.Error("expected error for negative")
	}
}

// printHelp smoke test — just verify it doesn't panic and produces output.
func TestPrintHelp(t *testing.T) {
	out := captureStdout(t, printHelp)
	if !strings.Contains(out, "awset") {
		t.Error("help output should mention awset")
	}
	if !strings.Contains(out, "new") {
		t.Error("help output should mention 'new' command")
	}
}

func TestDoSync_MissingArgs(t *testing.T) {
	resetReplicas(t)
	out := captureStdout(t, func() { doSync(nil) })
	if !strings.Contains(out, "usage") {
		t.Errorf("doSync missing args output = %q, want 'usage'", out)
	}
}

func TestDoSync_TypeMismatch(t *testing.T) {
	resetReplicas(t)
	captureStdout(t, func() {
		doNew([]string{"awset", "alice"})
		doNew([]string{"pncounter", "bob"})
	})
	out := captureStdout(t, func() { doSync([]string{"alice", "bob"}) })
	if !strings.Contains(out, "type mismatch") {
		t.Errorf("doSync type mismatch output = %q, want 'type mismatch'", out)
	}
}

func TestDoShow_MissingArgs(t *testing.T) {
	resetReplicas(t)
	out := captureStdout(t, func() { doShow(nil) })
	if !strings.Contains(out, "usage") {
		t.Errorf("doShow missing args output = %q, want 'usage'", out)
	}
}

// Verify all type names produce valid replicas with correct initial Show output.
func TestAllTypes_InitialState(t *testing.T) {
	expected := map[string]string{
		"awset":       "{}",
		"rwset":       "{}",
		"pncounter":   "0",
		"gcounter":    "0",
		"lwwregister": "<empty>",
		"ewflag":      "false",
		"dwflag":      "true",
		"mvregister":  "<empty>",
		"rga":         "[]",
	}
	for _, kind := range typeNames {
		r := mustNew(t, kind, fmt.Sprintf("r-%s", kind))
		want, ok := expected[kind]
		if !ok {
			t.Errorf("no expected initial state for %q", kind)
			continue
		}
		assertShow(t, r, want)
	}
}
