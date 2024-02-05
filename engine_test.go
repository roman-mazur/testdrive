package testdrive_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rmazur.io/testdrive"
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

	engine := testdrive.Engine{
		Parsers: testdrive.DefaultParsers(),
		Log:     log.New(os.Stdout, "", 0).Printf,
	}

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
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, scriptEntry := range entries {
		name := scriptEntry.Name()
		if !strings.HasSuffix(name, ".testdrive") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			src, err := os.Open(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = src.Close()
			})

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			engine := testdrive.Engine{
				Parsers: testdrive.DefaultParsers(),
				Log:     t.Logf,
			}
			sections, err := engine.Parse(name, src)
			if err != nil {
				t.Fatal(err)
			}
			if err := engine.Execute(testdrive.NewState(ctx), sections); err != nil {
				t.Fatal(err)
			}
		})
	}
}
