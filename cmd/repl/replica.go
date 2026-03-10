package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aalpar/crdt/awset"
	"github.com/aalpar/crdt/dotcontext"
	"github.com/aalpar/crdt/dwflag"
	"github.com/aalpar/crdt/ewflag"
	"github.com/aalpar/crdt/gcounter"
	"github.com/aalpar/crdt/lwwregister"
	"github.com/aalpar/crdt/mvregister"
	"github.com/aalpar/crdt/pncounter"
	"github.com/aalpar/crdt/rga"
	"github.com/aalpar/crdt/rwset"
)

// Replica is the type-erased interface for CRDT instances in the REPL.
// Each concrete wrapper stores its typed deltas and handles sync internally.
type Replica interface {
	Do(op string, args []string) (string, error)
	Show() string
	Type() string
	Sync(other Replica) error
	Ops() string
}

// newReplica creates a typed CRDT wrapper by name.
func newReplica(kind, id string) (Replica, error) {
	rid := dotcontext.ReplicaID(id)
	switch kind {
	case "awset":
		return &awsetReplica{set: awset.New[string](rid)}, nil
	case "rwset":
		return &rwsetReplica{set: rwset.New[string](rid)}, nil
	case "pncounter":
		return &pncounterReplica{ctr: pncounter.New(rid)}, nil
	case "gcounter":
		return &gcounterReplica{ctr: gcounter.New(rid)}, nil
	case "lwwregister":
		return &lwwregisterReplica{reg: lwwregister.New[string](rid)}, nil
	case "ewflag":
		return &ewflagReplica{flag: ewflag.New(rid)}, nil
	case "dwflag":
		return &dwflagReplica{flag: dwflag.New(rid)}, nil
	case "mvregister":
		return &mvregisterReplica{reg: mvregister.New[string](rid)}, nil
	case "rga":
		return &rgaReplica{seq: rga.New[string](rid)}, nil
	default:
		return nil, fmt.Errorf("unknown type %q (try: %s)", kind, strings.Join(typeNames, ", "))
	}
}

var typeNames = []string{
	"awset", "rwset", "pncounter", "gcounter",
	"lwwregister", "ewflag", "dwflag", "mvregister", "rga",
}

// --- AWSet ---

type awsetReplica struct {
	set    *awset.AWSet[string]
	deltas []*awset.AWSet[string]
}

func (r *awsetReplica) Type() string { return "awset" }
func (r *awsetReplica) Ops() string  { return "add <elem>, remove <elem>" }

func (r *awsetReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "add":
		if len(args) < 1 {
			return "", fmt.Errorf("add requires an element")
		}
		r.deltas = append(r.deltas, r.set.Add(args[0]))
		return fmt.Sprintf("added %q", args[0]), nil
	case "remove":
		if len(args) < 1 {
			return "", fmt.Errorf("remove requires an element")
		}
		r.deltas = append(r.deltas, r.set.Remove(args[0]))
		return fmt.Sprintf("removed %q", args[0]), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *awsetReplica) Show() string {
	elems := r.set.Elements()
	sort.Strings(elems)
	if len(elems) == 0 {
		return "{}"
	}
	return "{" + strings.Join(elems, ", ") + "}"
}

func (r *awsetReplica) Sync(other Replica) error {
	o, ok := other.(*awsetReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync awset with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.set.Merge(d)
	}
	for _, d := range o.deltas {
		r.set.Merge(d)
	}
	return nil
}

// --- RWSet ---

type rwsetReplica struct {
	set    *rwset.RWSet[string]
	deltas []*rwset.RWSet[string]
}

func (r *rwsetReplica) Type() string { return "rwset" }
func (r *rwsetReplica) Ops() string  { return "add <elem>, remove <elem>" }

func (r *rwsetReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "add":
		if len(args) < 1 {
			return "", fmt.Errorf("add requires an element")
		}
		r.deltas = append(r.deltas, r.set.Add(args[0]))
		return fmt.Sprintf("added %q", args[0]), nil
	case "remove":
		if len(args) < 1 {
			return "", fmt.Errorf("remove requires an element")
		}
		r.deltas = append(r.deltas, r.set.Remove(args[0]))
		return fmt.Sprintf("removed %q", args[0]), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *rwsetReplica) Show() string {
	elems := r.set.Elements()
	sort.Strings(elems)
	if len(elems) == 0 {
		return "{}"
	}
	return "{" + strings.Join(elems, ", ") + "}"
}

func (r *rwsetReplica) Sync(other Replica) error {
	o, ok := other.(*rwsetReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync rwset with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.set.Merge(d)
	}
	for _, d := range o.deltas {
		r.set.Merge(d)
	}
	return nil
}

// --- PNCounter ---

type pncounterReplica struct {
	ctr    *pncounter.Counter
	deltas []*pncounter.Counter
}

func (r *pncounterReplica) Type() string { return "pncounter" }
func (r *pncounterReplica) Ops() string  { return "inc <n>, dec <n>" }

func (r *pncounterReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "inc":
		n, err := requireInt64(args)
		if err != nil {
			return "", fmt.Errorf("inc: %w", err)
		}
		r.deltas = append(r.deltas, r.ctr.Increment(n))
		return fmt.Sprintf("incremented by %d → %d", n, r.ctr.Value()), nil
	case "dec":
		n, err := requireInt64(args)
		if err != nil {
			return "", fmt.Errorf("dec: %w", err)
		}
		r.deltas = append(r.deltas, r.ctr.Decrement(n))
		return fmt.Sprintf("decremented by %d → %d", n, r.ctr.Value()), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *pncounterReplica) Show() string {
	return strconv.FormatInt(r.ctr.Value(), 10)
}

func (r *pncounterReplica) Sync(other Replica) error {
	o, ok := other.(*pncounterReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync pncounter with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.ctr.Merge(d)
	}
	for _, d := range o.deltas {
		r.ctr.Merge(d)
	}
	return nil
}

// --- GCounter ---

type gcounterReplica struct {
	ctr    *gcounter.Counter
	deltas []*gcounter.Counter
}

func (r *gcounterReplica) Type() string { return "gcounter" }
func (r *gcounterReplica) Ops() string  { return "inc <n>" }

func (r *gcounterReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "inc":
		n, err := requireUint64(args)
		if err != nil {
			return "", fmt.Errorf("inc: %w", err)
		}
		r.deltas = append(r.deltas, r.ctr.Increment(n))
		return fmt.Sprintf("incremented by %d → %d", n, r.ctr.Value()), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *gcounterReplica) Show() string {
	return strconv.FormatUint(r.ctr.Value(), 10)
}

func (r *gcounterReplica) Sync(other Replica) error {
	o, ok := other.(*gcounterReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync gcounter with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.ctr.Merge(d)
	}
	for _, d := range o.deltas {
		r.ctr.Merge(d)
	}
	return nil
}

// --- LWWRegister ---

type lwwregisterReplica struct {
	reg    *lwwregister.LWWRegister[string]
	deltas []*lwwregister.LWWRegister[string]
}

func (r *lwwregisterReplica) Type() string { return "lwwregister" }
func (r *lwwregisterReplica) Ops() string  { return "set <value> <timestamp>" }

func (r *lwwregisterReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "set":
		if len(args) < 2 {
			return "", fmt.Errorf("set requires <value> <timestamp>")
		}
		ts, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid timestamp %q: %w", args[1], err)
		}
		r.deltas = append(r.deltas, r.reg.Set(args[0], ts))
		return fmt.Sprintf("set %q at ts=%d", args[0], ts), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *lwwregisterReplica) Show() string {
	v, ts, ok := r.reg.Value()
	if !ok {
		return "<empty>"
	}
	return fmt.Sprintf("%q (ts=%d)", v, ts)
}

func (r *lwwregisterReplica) Sync(other Replica) error {
	o, ok := other.(*lwwregisterReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync lwwregister with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.reg.Merge(d)
	}
	for _, d := range o.deltas {
		r.reg.Merge(d)
	}
	return nil
}

// --- EWFlag ---

type ewflagReplica struct {
	flag   *ewflag.EWFlag
	deltas []*ewflag.EWFlag
}

func (r *ewflagReplica) Type() string { return "ewflag" }
func (r *ewflagReplica) Ops() string  { return "enable, disable" }

func (r *ewflagReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "enable":
		r.deltas = append(r.deltas, r.flag.Enable())
		return fmt.Sprintf("enabled → %v", r.flag.Value()), nil
	case "disable":
		r.deltas = append(r.deltas, r.flag.Disable())
		return fmt.Sprintf("disabled → %v", r.flag.Value()), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *ewflagReplica) Show() string {
	return strconv.FormatBool(r.flag.Value())
}

func (r *ewflagReplica) Sync(other Replica) error {
	o, ok := other.(*ewflagReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync ewflag with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.flag.Merge(d)
	}
	for _, d := range o.deltas {
		r.flag.Merge(d)
	}
	return nil
}

// --- DWFlag ---

type dwflagReplica struct {
	flag   *dwflag.DWFlag
	deltas []*dwflag.DWFlag
}

func (r *dwflagReplica) Type() string { return "dwflag" }
func (r *dwflagReplica) Ops() string  { return "enable, disable" }

func (r *dwflagReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "enable":
		r.deltas = append(r.deltas, r.flag.Enable())
		return fmt.Sprintf("enabled → %v", r.flag.Value()), nil
	case "disable":
		r.deltas = append(r.deltas, r.flag.Disable())
		return fmt.Sprintf("disabled → %v", r.flag.Value()), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *dwflagReplica) Show() string {
	return strconv.FormatBool(r.flag.Value())
}

func (r *dwflagReplica) Sync(other Replica) error {
	o, ok := other.(*dwflagReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync dwflag with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.flag.Merge(d)
	}
	for _, d := range o.deltas {
		r.flag.Merge(d)
	}
	return nil
}

// --- MVRegister ---

type mvregisterReplica struct {
	reg    *mvregister.MVRegister[string]
	deltas []*mvregister.MVRegister[string]
}

func (r *mvregisterReplica) Type() string { return "mvregister" }
func (r *mvregisterReplica) Ops() string  { return "write <value>" }

func (r *mvregisterReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "write":
		if len(args) < 1 {
			return "", fmt.Errorf("write requires a value")
		}
		r.deltas = append(r.deltas, r.reg.Write(args[0]))
		return fmt.Sprintf("wrote %q", args[0]), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *mvregisterReplica) Show() string {
	vals := r.reg.Values()
	if len(vals) == 0 {
		return "<empty>"
	}
	sort.Strings(vals)
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func (r *mvregisterReplica) Sync(other Replica) error {
	o, ok := other.(*mvregisterReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync mvregister with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.reg.Merge(d)
	}
	for _, d := range o.deltas {
		r.reg.Merge(d)
	}
	return nil
}

// --- RGA ---

type rgaReplica struct {
	seq    *rga.RGA[string]
	deltas []*rga.RGA[string]
}

func (r *rgaReplica) Type() string { return "rga" }
func (r *rgaReplica) Ops() string  { return "insert <index> <value>, delete <index>" }

func (r *rgaReplica) Do(op string, args []string) (string, error) {
	switch op {
	case "insert":
		if len(args) < 2 {
			return "", fmt.Errorf("insert requires <index> <value>")
		}
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			return "", fmt.Errorf("invalid index %q: %w", args[0], err)
		}
		r.deltas = append(r.deltas, r.seq.InsertAt(idx, args[1]))
		return fmt.Sprintf("inserted %q at %d", args[1], idx), nil
	case "delete":
		if len(args) < 1 {
			return "", fmt.Errorf("delete requires <index>")
		}
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			return "", fmt.Errorf("invalid index %q: %w", args[0], err)
		}
		r.deltas = append(r.deltas, r.seq.DeleteAt(idx))
		return fmt.Sprintf("deleted index %d", idx), nil
	default:
		return "", fmt.Errorf("unknown op %q (try: %s)", op, r.Ops())
	}
}

func (r *rgaReplica) Show() string {
	elems := r.seq.Elements()
	if len(elems) == 0 {
		return "[]"
	}
	parts := make([]string, len(elems))
	for i, e := range elems {
		parts[i] = e.Value
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (r *rgaReplica) Sync(other Replica) error {
	o, ok := other.(*rgaReplica)
	if !ok {
		return fmt.Errorf("type mismatch: cannot sync rga with %s", other.Type())
	}
	for _, d := range r.deltas {
		o.seq.Merge(d)
	}
	for _, d := range o.deltas {
		r.seq.Merge(d)
	}
	return nil
}

// --- helpers ---

func requireInt64(args []string) (int64, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("requires a number")
	}
	return strconv.ParseInt(args[0], 10, 64)
}

func requireUint64(args []string) (uint64, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("requires a number")
	}
	return strconv.ParseUint(args[0], 10, 64)
}
