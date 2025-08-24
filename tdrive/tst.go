package tdrive

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rmazur.io/testdrive"
)

// RunTests reads .testdrive files from the specified directory and executes a subtest for each of them.
// testdrive.Engine ExecuteScript method is used to run each of the scripts.
func RunTests(t *testing.T, ctx context.Context, dir string, config func(t *testing.T, engine *testdrive.Engine)) {
	scripts, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	const ext = ".testdrive"

	for _, script := range scripts {
		if script.IsDir() || !strings.HasSuffix(script.Name(), ext) {
			continue
		}
		name := strings.TrimSuffix(script.Name(), ext)
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, script.Name()))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = f.Close() })

			var engine testdrive.Engine
			config(t, &engine)

			state, err := engine.ExecuteScript(ctx, name, f)
			if err != nil {
				t.Fatal(err)
			}
			if err := state.LastError(); err != nil {
				srcName, line := state.Location()
				t.Errorf("%s:%d %s", srcName, line, err)
			}
		})
	}
}
