package testdrive_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

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

	logger := log.New(os.Stdout, "", 0)
	engine := testdrive.Engine{
		Parsers: map[string]testdrive.Parser{
			"VALUE": testdrive.ParseValueCmd[testdrive.SetValue],
			"MATCH": testdrive.ParseValueCmd[testdrive.MatchValue],
		},
		Log: logger.Printf,
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
