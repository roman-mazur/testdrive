package testdrive

import (
	"context"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// State encapsulates the current state of executing a script.
// It holds a cue.Value representing the result of the last Command and the last execution error.
type State struct {
	context.Context

	engine *Engine
	cueCtx *cue.Context

	srcName string // name of the current source (e.g., file name)
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

// Location returns information about the source of the last executed Command.
func (s *State) Location() (source string, lineno int) { return s.srcName, s.lineno }

// LastError returns the last error produced by executing a Command.
func (s *State) LastError() error { return s.err }

// PushValue should be used by a Command to add the value it calculated.
func (s *State) PushValue(val cue.Value) {
	s.values = append([]cue.Value{val}, s.values...)
	if len(s.values) > 10 {
		s.values = s.values[:10]
	}
}

func (s *State) CompileValue(str string) (cue.Value, error) {
	val := s.cueCtx.CompileString(str, s.cueOptions()...)
	return val, val.Err()
}

func (s *State) UnifyValue(val cue.Value) (cue.Value, error) {
	var last cue.Value
	if len(s.values) > 0 {
		last = s.values[0]
	} else {
		last, _ = s.CompileValue("_")
	}
	v := last.Unify(val)
	return v, v.Err()
}

type exprScope struct {
	LastValue *cue.Value  `json:"$"`
	History   []cue.Value `json:"$history"`
}

func (s *State) cueOptions() []cue.BuildOption {
	refs := exprScope{History: s.values}
	if len(s.values) > 0 {
		refs.LastValue = &s.values[0]
	}

	return []cue.BuildOption{
		cue.Filename(s.srcName),
		cue.Scope(s.cueCtx.Encode(refs)),
	}
}

// Expand can be used by a Command to evaluate a string interpolation using the current state data.
// This method looks for occurrences of expressions enclosed in \( and ), evaluates this expression using
// CompileValue and replaces the expression with the calculated value.
// For example, if the state values history includes 1 and 2, the following string
//
//	Numbers are \($) and \($history[1]).
//
// will expand to
//
//	Numbers are 1 and 2.
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
