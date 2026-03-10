package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var replicas = map[string]Replica{}

func main() {
	fmt.Println("CRDT REPL — type 'help' for commands")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("crdt> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		cmd := fields[0]
		args := fields[1:]

		switch cmd {
		case "help":
			printHelp()
		case "exit", "quit":
			return
		case "new":
			doNew(args)
		case "show":
			doShow(args)
		case "sync":
			doSync(args)
		case "replicas":
			doList()
		default:
			// Treat as: <replica> <op> [args...]
			doOp(cmd, args)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}
}

func doNew(args []string) {
	if len(args) < 2 {
		fmt.Println("usage: new <type> <name>")
		return
	}
	kind, name := args[0], args[1]
	if _, exists := replicas[name]; exists {
		fmt.Printf("error: %q already exists\n", name)
		return
	}
	r, err := newReplica(kind, name)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	replicas[name] = r
	fmt.Printf("created %s replica %q\n", kind, name)
}

func doShow(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: show <name>")
		return
	}
	r, ok := replicas[args[0]]
	if !ok {
		fmt.Printf("error: unknown replica %q\n", args[0])
		return
	}
	fmt.Printf("%s (%s) = %s\n", args[0], r.Type(), r.Show())
}

func doSync(args []string) {
	if len(args) < 2 {
		fmt.Println("usage: sync <name1> <name2>")
		return
	}
	r1, ok := replicas[args[0]]
	if !ok {
		fmt.Printf("error: unknown replica %q\n", args[0])
		return
	}
	r2, ok := replicas[args[1]]
	if !ok {
		fmt.Printf("error: unknown replica %q\n", args[1])
		return
	}
	if err := r1.Sync(r2); err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Printf("synced %s <-> %s\n", args[0], args[1])
}

func doList() {
	if len(replicas) == 0 {
		fmt.Println("no replicas")
		return
	}
	for name, r := range replicas {
		fmt.Printf("  %-12s %-12s %s\n", name, r.Type(), r.Show())
	}
}

func doOp(name string, args []string) {
	r, ok := replicas[name]
	if !ok {
		fmt.Printf("error: unknown command or replica %q\n", name)
		return
	}
	if len(args) < 1 {
		fmt.Printf("usage: %s <op> [args...] (ops: %s)\n", name, r.Ops())
		return
	}
	result, err := r.Do(args[0], args[1:])
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Println(result)
}

func printHelp() {
	fmt.Println(`Commands:
  new <type> <name>       Create a replica
  <name> <op> [args...]   Execute an operation on a replica
  show <name>             Display replica state
  sync <name1> <name2>    Bidirectional delta exchange
  replicas                List all replicas
  help                    Show this help
  exit                    Quit

Types and operations:
  awset        add <elem>, remove <elem>
  rwset        add <elem>, remove <elem>
  pncounter    inc <n>, dec <n>
  gcounter     inc <n>
  lwwregister  set <value> <timestamp>
  ewflag       enable, disable
  dwflag       enable, disable
  mvregister   write <value>
  rga          insert <index> <value>, delete <index>

Example session:
  new awset alice
  new awset bob
  alice add hello
  alice add world
  show alice          → {hello, world}
  bob add world
  bob remove hello
  sync alice bob
  show alice          → {hello, world}
  show bob            → {hello, world}`)
}
