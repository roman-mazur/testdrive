package testdrive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Engine keeps configuration for executing a script.
type Engine struct {
	Parsers map[string]Parser

	Log func(fmt string, args ...any)
}

// Parse parses the input script forming a slice of Section that can be executed via a dedicated method of the Engine.
func (e *Engine) Parse(srcName string, input io.Reader) (result []Section, err error) {
	in := bufio.NewReader(input)

	var (
		lineno       int
		section      Section
		firstSection = true
	)

	flush := func() {
		firstSection = false
		result = append(result, section)
		section = Section{}
	}

	for {
		lineno++

		line, lineErr := in.ReadString('\n')
		if lineErr != nil && lineErr != io.EOF {
			err = fmt.Errorf("failed to read source %s at line %d: %s", srcName, lineno, lineErr)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			if lineErr == io.EOF {
				break
			}
			continue
		}

		delim := strings.Index(line, " ")
		if delim == -1 {
			delim = len(line)
		}

		switch prefix := line[:delim]; prefix {
		case "#":
			// A new section.
			if !firstSection {
				flush()
			}
			section.Name = strings.TrimSpace(line[2:])
			section.Lineno = lineno

		default:
			// Command.
			parser, exists := e.Parsers[prefix]
			if !exists {
				err = fmt.Errorf("unknown command %s at line %d", prefix, lineno)
				return
			}
			lineToParse := ""
			if len(prefix) < len(line) {
				lineToParse = strings.TrimSpace(line[len(prefix):])
			}
			cmd, inc, cmdError := parser(lineToParse, in)
			if cmdError != nil {
				err = fmt.Errorf("failed to parse command %s at line %d: %s", prefix, lineno, cmdError)
				return
			}
			section.Commands = append(section.Commands, LocCommand{cmd, lineno, srcName})
			lineno += inc
		}
	}

	if len(section.Commands) > 0 || section.Name != "" {
		flush()
	}
	return
}

// Execute executes the provided sections.
// Commands are executed one at a time. This function exits when the execution stops.
// If a Command fails, subsequent commands are not executed or parsed.
func (e *Engine) Execute(state *State, sections []Section) error {
	for _, sec := range sections {
		e.log("Executing section \"%s\"", sec.Name)
		for _, cmdPos := range sec.Commands {
			state.srcName = cmdPos.SourceName
			state.lineno = cmdPos.Lineno
			if err := cmdPos.Command.Run(state); err != nil {
				return err
			}
		}
		e.Log("%s", state.val)
	}
	return nil
}

func (e *Engine) log(fmt string, args ...any) {
	if e.Log != nil {
		e.Log(fmt, args...)
	}
}

// Section is a named sequence of commands.
// Can be considered as a group of commands that may form a particular named test.
type Section struct {
	Name     string
	Commands []LocCommand
	Lineno   int
}

// LocCommand holds Command and its line number in the source.
type LocCommand struct {
	Command    Command
	Lineno     int
	SourceName string
}

// Command represents a command interface. It can be run given the current State.
type Command interface {
	Run(*State) error // Updates the given State with the result of execution.
}

// Parser can parse the script input and produce a Command.
type Parser func(line string, reader *bufio.Reader) (cmd Command, lines int, err error)

// State encapsulates the current state of executing a script.
// It holds a cue.Value representing the result of the last Command and the last execution error.
type State struct {
	context.Context

	cueCtx *cue.Context

	srcName string // name of the current source (e.g. file name)
	lineno  int    // line number in the current source

	err error     // error from the last command
	val cue.Value // value from the last command
}

func NewState(ctx context.Context) *State {
	return &State{
		Context: ctx,
		cueCtx:  cuecontext.New(),
	}
}

func (s *State) Location() (source string, lineno int) { return s.srcName, s.lineno }
func (s *State) SetValue(val cue.Value)                { s.val = val }
func (s *State) LastError() error                      { return s.err }

func (s *State) CompileValue(str string) (cue.Value, error) {
	val := s.cueCtx.CompileString(str, cue.Filename(s.srcName))
	return val, val.Err()
}

func (s *State) UnifyValue(val cue.Value) (cue.Value, error) {
	v := s.val.Unify(val)
	return v, v.Err()
}
