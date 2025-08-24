package testdrive_test

import (
	"context"
	"fmt"
	"iter"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rmazur.io/testdrive"
	"rmazur.io/testdrive/internal/testserver"
	"rmazur.io/testdrive/tdrive"
)

func ExampleEngine() {
	script := `
# basic example

VALUE {"foo":"bar"}

MATCH ^END
foo: "bar"
END
`

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state := testdrive.NewState(ctx)

	var engine testdrive.Engine
	engine.Configure(testdrive.WithLog(log.New(os.Stdout, "", 0).Printf))

	sections, err := engine.Parse("example", strings.NewReader(script))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Sections count:", len(sections))
	if err := engine.Execute(state, sections); err != nil {
		log.Fatal(err)
	}

	// Output:
	// Sections count: 1
	// Executing section "basic example"
	// {
	//	foo: "bar"
	// }
}

func TestEngine(t *testing.T) {
	entries, srv := initScriptsTest(t)

	for scriptEntry := range entries {
		name := scriptEntry.Name()
		t.Run(name, func(t *testing.T) {
			src := openTestScript(t, name)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			var engine testdrive.Engine
			engine.Configure(testdrive.WithCommonParsers(), testdrive.WithLog(t.Logf), testdrive.WithBaseURL(srv.URL))

			sections, err := engine.Parse(name, src)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.Execute(testdrive.NewState(ctx), sections); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestEngine_ExecuteScript(t *testing.T) {
	_, srv := initScriptsTest(t)
	tdrive.RunTests(t, context.Background(), "testdata", func(t *testing.T, engine *testdrive.Engine) {
		engine.Configure(testdrive.WithCommonParsers(), testdrive.WithLog(t.Logf), testdrive.WithBaseURL(srv.URL))
	})
}

func TestState_Expand(t *testing.T) {
	state := testdrive.NewState(context.Background())
	val, err := state.CompileValue("{a:true,n:21,s:\"abc\"}")
	if err != nil {
		t.Fatal(err)
	}
	state.PushValue(val)

	for _, tc := range []struct {
		name string
		in   string
		out  string
		err  string
	}{
		{name: "no vars", in: "string", out: "string"},
		{name: "string", in: "string \\($.s)", out: "string abc"},
		{name: "int", in: "int \\($.n)", out: "int 21"},
		{name: "bool", in: "bool \\($.a)", out: "bool true"},
		{name: "not closed", in: "some \\($.a text", out: "some \\($.a text"},
		{name: "brackets", in: "some ($.a) text", out: "some ($.a) text"},
		{name: "string in the middle", in: "some \\($.s) text", out: "some abc text"},
		{name: "only value", in: "\\($.n)", out: "21"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if res, err := state.Expand(tc.in); err != nil {
				if err.Error() != tc.err {
					t.Errorf("unexpected error [%s], wanted [%s]", err, tc.err)
				}
			} else if tc.err != "" {
				t.Errorf("expected error [%s], but got nothing", tc.err)
			} else if res != tc.out {
				t.Errorf("unexpected result [%s], wanted [%s]", res, tc.out)
			}
		})
	}
}

func initScriptsTest(t *testing.T) (iter.Seq[os.DirEntry], *httptest.Server) {
	t.Helper()
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(testserver.CreateHandler())
	t.Cleanup(srv.Close)

	it := func(yield func(entry os.DirEntry) bool) {
		for _, scriptEntry := range entries {
			name := scriptEntry.Name()
			if !strings.HasSuffix(name, ".testdrive") {
				continue
			}
			yield(scriptEntry)
		}
	}
	return it, srv
}

func openTestScript(t *testing.T, name string) *os.File {
	src, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = src.Close()
	})
	return src
}
