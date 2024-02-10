package testdrive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Engine keeps configuration for executing a script.
type Engine struct {
	parsers map[string]Parser
	logf    func(fmt string, args ...any)

	httpClient     *http.Client
	baseURL        string
	reqInterceptor func(req *http.Request)
}

type option func(*Engine)

// Configure applies the provided configuration options.
func (e *Engine) Configure(opts ...option) {
	for _, opt := range opts {
		opt(e)
	}
}

var defaultParsers = map[string]Parser{
	"VALUE": parseValueCmd[setValue],
	"MATCH": parseValueCmd[matchValue],
}

func WithParsers(p map[string]Parser) option {
	return func(engine *Engine) {
		parsers := make(map[string]Parser, len(p)+len(defaultParsers))
		for name, parser := range p {
			parsers[name] = parser
		}
		for name, parser := range defaultParsers {
			parsers[name] = parser
		}
		engine.parsers = parsers
	}
}

func WithCommonParsers() option {
	return WithParsers(map[string]Parser{
		"HTTP": ParseHTTP,
	})
}

func WithLog(log func(fmt string, args ...any)) option {
	return func(engine *Engine) { engine.logf = log }
}

func WithHTTPClient(hc *http.Client) option {
	return func(engine *Engine) { engine.httpClient = hc }
}

func WithBaseURL(url string) option {
	return func(engine *Engine) { engine.baseURL = url }
}

func WithRequestInterceptor(f func(*http.Request)) option {
	return func(engine *Engine) { engine.reqInterceptor = f }
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
		result = append(result, section)
		section = Section{}
	}

	parsers := e.parsers
	if parsers == nil {
		parsers = defaultParsers
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
			firstSection = false

		default:
			// Command.
			parser, exists := parsers[prefix]
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
	state.engine = e
	defer func() {
		state.engine = nil
	}()

	for _, sec := range sections {
		e.log("Executing section \"%s\"", sec.Name)
		for _, cmdPos := range sec.Commands {
			state.srcName = cmdPos.SourceName
			state.lineno = cmdPos.Lineno
			if err := cmdPos.Command.Run(state); err != nil {
				return err
			}
		}
		e.logf("%s", state.values[len(state.values)-1])
	}
	return nil
}

func (e *Engine) log(fmt string, args ...any) {
	if e.logf != nil {
		e.logf(fmt, args...)
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

	engine *Engine
	cueCtx *cue.Context

	srcName string // name of the current source (e.g. file name)
	lineno  int    // line number in the current source

	err    error       // error from the last command
	values []cue.Value // last 10 values
}

func NewState(ctx context.Context) *State {
	return &State{
		Context: ctx,
		cueCtx:  cuecontext.New(),
	}
}

func (s *State) Location() (source string, lineno int) { return s.srcName, s.lineno }
func (s *State) LastError() error                      { return s.err }

func (s *State) PushValue(val cue.Value) {
	s.values = append(s.values, val)
	if len(s.values) > 10 {
		s.values = s.values[:10]
	}
}

func (s *State) cueOptions() []cue.BuildOption {
	history := make([]cue.Value, len(s.values))
	for i := range s.values {
		history[len(s.values)-1-i] = s.values[i]
	}
	var lastValue *cue.Value
	if len(s.values) > 0 {
		lastValue = &history[0]
	}
	refs := scope{
		LastValue: lastValue,
		History:   history,
	}
	return []cue.BuildOption{cue.Filename(s.srcName), cue.Scope(s.cueCtx.Encode(refs))}
}

func (s *State) CompileValue(str string) (cue.Value, error) {
	val := s.cueCtx.CompileString(str, s.cueOptions()...)
	return val, val.Err()
}

func (s *State) UnifyValue(val cue.Value) (cue.Value, error) {
	last := s.values[len(s.values)-1]
	v := last.Unify(val)
	return v, v.Err()
}

func (s *State) Expand(str string) (string, error) {
	var (
		res                    strings.Builder
		startEscape, startExpr bool
		partIndex, exprIndex   int
	)
	for i := range str {
		switch str[i] {
		case '\\':
			if startEscape {
				startEscape = false
				res.WriteString(str[partIndex:i])
				partIndex = i + 1
			} else {
				startEscape = true
			}
		case '(':
			if startEscape {
				startEscape = false
				startExpr = true
				exprIndex = i + 1
				res.WriteString(str[partIndex : i-1])
				partIndex = i - 1
			}
		case ')':
			if startEscape {
				return "", fmt.Errorf("invalid escape sequence at position %d (%s)", i, str[i:i+1])
			}
			if startExpr {
				startExpr = false
				expr := str[exprIndex:i]
				val, err := s.CompileValue(expr)
				if err != nil {
					return "", fmt.Errorf("cannot evaluate expression %s at position %d: %s", expr, exprIndex, err)
				}
				_, _ = fmt.Fprintf(&res, "%s", val)
				partIndex = i + 1
			}
		default:
			if startEscape {
				return "", fmt.Errorf("invalid escape sequence at position %d (%s)", i, str[i:i+1])
			}
		}
	}

	if partIndex < len(str) {
		res.WriteString(str[partIndex:])
	}
	return res.String(), nil
}

type scope struct {
	LastValue *cue.Value  `json:"$"`
	History   []cue.Value `json:"$history"`
}
