package readme_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// TestREADMEGoSnippetsCompile extracts Go code blocks from README.md,
// wraps each in a compilable scaffold, and verifies they compile against
// the current crdt API. This catches API drift between documented examples
// and the actual public API (e.g., a renamed method, a changed return type).
func TestREADMEGoSnippetsCompile(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	blocks := extractGoCodeBlocks(string(readme))
	if len(blocks) == 0 {
		t.Fatal("no Go code blocks found in README.md")
	}

	repoRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	tmpDir := t.TempDir()
	writeREADMETestGoMod(t, tmpDir, repoRoot)

	source := assembleREADMESource(blocks)
	srcPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(srcPath, []byte(source), 0o644)
	if err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyOutput, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed:\n%s", tidyOutput)
	}

	buildCmd := exec.CommandContext(ctx, "go", "build", ".")
	buildCmd.Dir = tmpDir
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("README Go snippets failed to compile:\n%s\n\nGenerated source:\n%s", buildOutput, source)
	}

	t.Logf("verified %d Go code blocks from README.md compile", len(blocks))
}

func TestExtractGoCodeBlocks(t *testing.T) {
	c := qt.New(t)

	c.Run("TwoBlocks", func(c *qt.C) {
		md := "# Heading\n\n```go\na := 1\n```\n\nsome text\n\n```go\nb := 2\n```\n"
		blocks := extractGoCodeBlocks(md)
		c.Assert(blocks, qt.HasLen, 2)
		c.Assert(blocks[0], qt.Equals, "a := 1")
		c.Assert(blocks[1], qt.Equals, "b := 2")
	})
	c.Run("IgnoresNonGo", func(c *qt.C) {
		md := "```\nnot go\n```\n\n```python\nalso not go\n```\n\n```go\ngo code\n```\n"
		blocks := extractGoCodeBlocks(md)
		c.Assert(blocks, qt.HasLen, 1)
		c.Assert(blocks[0], qt.Equals, "go code")
	})
	c.Run("Empty", func(c *qt.C) {
		blocks := extractGoCodeBlocks("# No code here\n\nJust text.\n")
		c.Assert(blocks, qt.HasLen, 0)
	})
}

func TestReadmeTopLevelVars(t *testing.T) {
	c := qt.New(t)

	c.Run("Basic", func(c *qt.C) {
		block := "a := 1\nb := 2\nfmt.Println(a)"
		vars := readmeTopLevelVars(block)
		c.Assert(vars, qt.DeepEquals, []string{"a", "b"})
	})
	c.Run("MultiAssign", func(c *qt.C) {
		block := "x, y := foo()"
		vars := readmeTopLevelVars(block)
		c.Assert(vars, qt.DeepEquals, []string{"x", "y"})
	})
	c.Run("SkipsClosure", func(c *qt.C) {
		block := "a := 1\nfn := func() {\n\tb := 2\n}\nc := 3"
		vars := readmeTopLevelVars(block)
		// b is inside braces — should be excluded.
		c.Assert(vars, qt.DeepEquals, []string{"a", "fn", "c"})
	})
	c.Run("SkipsBlankIdentifier", func(c *qt.C) {
		block := "_ := ignored\na := 1"
		vars := readmeTopLevelVars(block)
		c.Assert(vars, qt.DeepEquals, []string{"a"})
	})
	c.Run("SkipsComments", func(c *qt.C) {
		block := "// x := commented out\na := 1"
		vars := readmeTopLevelVars(block)
		c.Assert(vars, qt.DeepEquals, []string{"a"})
	})
	c.Run("NoDuplicates", func(c *qt.C) {
		block := "a := 1\na := 2"
		vars := readmeTopLevelVars(block)
		c.Assert(vars, qt.DeepEquals, []string{"a"})
	})
}

func TestAssembleREADMESource(t *testing.T) {
	c := qt.New(t)

	c.Run("InfersImports", func(c *qt.C) {
		blocks := []string{"a := awset.New[string](\"r1\")\nfmt.Println(a)"}
		source := assembleREADMESource(blocks)
		c.Assert(strings.Contains(source, `"github.com/aalpar/crdt/awset"`), qt.IsTrue)
		c.Assert(strings.Contains(source, `"fmt"`), qt.IsTrue)
		// Should not import unused packages.
		c.Assert(strings.Contains(source, `"github.com/aalpar/crdt/ewflag"`), qt.IsFalse)
	})
	c.Run("WrapsInFunctions", func(c *qt.C) {
		blocks := []string{"x := 1", "y := 2"}
		source := assembleREADMESource(blocks)
		c.Assert(strings.Contains(source, "func readme_0()"), qt.IsTrue)
		c.Assert(strings.Contains(source, "func readme_1()"), qt.IsTrue)
		// Unused vars silenced.
		c.Assert(strings.Contains(source, "_ = x"), qt.IsTrue)
		c.Assert(strings.Contains(source, "_ = y"), qt.IsTrue)
	})
}

// goCodeBlockRe matches fenced ```go code blocks in Markdown.
// The \r? allows matching both LF and CRLF line endings.
var goCodeBlockRe = regexp.MustCompile("(?s)```go\\r?\\n(.*?)\\r?\\n```")

// assignRe matches short variable declarations at the start of a line.
var assignRe = regexp.MustCompile(`^(\w+(?:\s*,\s*\w+)*)\s*:=`)

// crdtPkgs maps short package names to their import paths within this module.
var crdtPkgs = map[string]string{
	"awset":       "github.com/aalpar/crdt/awset",
	"ewflag":      "github.com/aalpar/crdt/ewflag",
	"lwwregister": "github.com/aalpar/crdt/lwwregister",
	"pncounter":   "github.com/aalpar/crdt/pncounter",
	"ormap":       "github.com/aalpar/crdt/ormap",
	"dotcontext":  "github.com/aalpar/crdt/dotcontext",
}

func extractGoCodeBlocks(readme string) []string {
	matches := goCodeBlockRe.FindAllStringSubmatch(readme, -1)
	blocks := make([]string, len(matches))
	for i, m := range matches {
		blocks[i] = m[1]
	}
	return blocks
}

func writeREADMETestGoMod(t *testing.T, dir, repoRoot string) {
	t.Helper()
	content := fmt.Sprintf(
		"module readme_check\n\ngo 1.25\n\nrequire github.com/aalpar/crdt v0.0.0\n\nreplace github.com/aalpar/crdt => %q\n",
		repoRoot,
	)
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}

// assembleREADMESource builds a single Go source file where each README
// code block becomes its own function. Package imports are inferred from
// usage (e.g., awset. → import crdt/awset). Unused variables are silenced
// with blank-identifier assignments so compilation succeeds even for
// partial snippets.
func assembleREADMESource(blocks []string) string {
	// Detect which imports are needed across all blocks.
	imports := map[string]bool{}
	for _, block := range blocks {
		if strings.Contains(block, "fmt.") {
			imports[`"fmt"`] = true
		}
		for pkg, path := range crdtPkgs {
			if strings.Contains(block, pkg+".") {
				imports[`"`+path+`"`] = true
			}
		}
	}

	var src strings.Builder

	src.WriteString("package main\n\n")

	src.WriteString("import (\n")
	for imp := range imports {
		fmt.Fprintf(&src, "\t%s\n", imp)
	}
	src.WriteString(")\n\n")

	// Suppress "imported and not used" for fmt in case some blocks only
	// reference it inside comments or as a type, not a direct call.
	if imports[`"fmt"`] {
		src.WriteString("var _ = fmt.Println\n\n")
	}

	src.WriteString("func main() {}\n\n")

	for i, block := range blocks {
		fmt.Fprintf(&src, "func readme_%d() {\n", i)

		for _, line := range strings.Split(block, "\n") {
			if strings.TrimSpace(line) == "" {
				src.WriteString("\n")
			} else {
				fmt.Fprintf(&src, "\t%s\n", line)
			}
		}

		// Silence unused variables declared at the top level of this block.
		for _, v := range readmeTopLevelVars(block) {
			fmt.Fprintf(&src, "\t_ = %s\n", v)
		}

		src.WriteString("}\n\n")
	}

	return src.String()
}

// readmeTopLevelVars returns variable names declared with := at brace depth 0
// in the block. Variables inside closures (depth > 0) are excluded.
//
// Limitation: brace counting does not skip braces inside string literals or
// comments. This is sufficient for the current README snippets.
func readmeTopLevelVars(block string) []string {
	braceDepth := 0
	seen := map[string]bool{}
	var vars []string

	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if braceDepth == 0 {
			sub := assignRe.FindStringSubmatch(trimmed)
			if sub != nil {
				for _, v := range strings.Split(sub[1], ",") {
					v = strings.TrimSpace(v)
					if v != "_" && !seen[v] {
						vars = append(vars, v)
						seen[v] = true
					}
				}
			}
		}

		braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
	}

	return vars
}
