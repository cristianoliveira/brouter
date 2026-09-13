package main

import "testing"

func TestL001FlagsStrayPrintsOutsideCompositionAndTests(t *testing.T) {
	// Given a library file prints with fmt or builtins, when it is
	// analyzed, L001 is reported at the call site.
	src := []byte(`package domain

import "fmt"

func Render(s string) string {
	fmt.Println(s)
	println("debug")
	return s
}
`)
	findings := analyze("internal/domain/render.go", src)

	if len(findings) != 2 {
		t.Fatalf("findings = %v, want 2 L001 findings", findings)
	}
	for _, f := range findings {
		if f.Rule != "L001" {
			t.Errorf("rule = %q, want L001", f.Rule)
		}
	}
	if findings[0].Line != 6 || findings[1].Line != 7 {
		t.Errorf("lines = %d,%d, want 6,7", findings[0].Line, findings[1].Line)
	}
}

func TestL001AllowsPrintsInCmdCompositionAndTests(t *testing.T) {
	// Given printing happens in CLI composition or test files, when the
	// files are analyzed, no finding is reported.
	composition := []byte(`package main

import "fmt"

func main() {
	fmt.Println("usage")
}
`)
	tests := []byte(`package domain

import "fmt"
import "testing"

func TestX(t *testing.T) {
	fmt.Println("diagnostic")
}
`)
	if findings := analyze("cmd/brouter/main.go", composition); len(findings) != 0 {
		t.Errorf("cmd findings = %v, want none", findings)
	}
	if findings := analyze("internal/domain/render_test.go", tests); len(findings) != 0 {
		t.Errorf("test findings = %v, want none", findings)
	}
}

func TestL002FlagsEmptyInterfaceOutsideTests(t *testing.T) {
	// Given a non-test file uses interface{}, when analyzed, L002 is
	// reported; `any` is accepted.
	src := []byte(`package domain

func Decode(raw interface{}) string {
	return string(raw.(interface{}).(string))
}
`)
	findings := analyze("internal/domain/decode.go", src)

	if len(findings) != 2 {
		t.Fatalf("findings = %v, want 2 L002 findings", findings)
	}
	for _, f := range findings {
		if f.Rule != "L002" {
			t.Errorf("rule = %q, want L002", f.Rule)
		}
	}

	modern := []byte(`package domain

func Decode(raw any) any { return raw }
`)
	if findings := analyze("internal/domain/decode.go", modern); len(findings) != 0 {
		t.Errorf("findings = %v, want none for any", findings)
	}
}

func TestAnalysisIsDeterministicAcrossRuns(t *testing.T) {
	// Given the same sources, when analysis runs twice, findings are
	// identical in order and content.
	src := []byte(`package domain

import "fmt"

func A() { fmt.Println("a") }
func B() { println("b") }
`)
	first := analyze("a.go", src)
	second := analyze("a.go", src)

	if len(first) != 2 {
		t.Fatalf("findings = %v, want 2", first)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("finding %d unstable: %v vs %v", i, first[i], second[i])
		}
	}
}
