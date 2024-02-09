package testdrive

import (
	"bufio"
	"strings"
)

type valueCommand interface {
	Command
	~string
}

func parseValueCmd[T valueCommand](prefix string, in *bufio.Reader) (Command, int, error) {
	if !strings.HasPrefix(prefix, "^") {
		return T(prefix), 0, nil
	}

	var buffer strings.Builder
	lc, err := readLines(in, prefix[1:], func(line string) {
		buffer.WriteString(line)
		buffer.WriteString("\n")
	})

	if err != nil {
		return nil, lc, err
	}
	return T(buffer.String()), lc, nil
}

type setValue string

func (sv setValue) Run(state *State) error {
	val, err := state.CompileValue(string(sv))
	if err != nil {
		return err
	}
	state.PushValue(val)
	return nil
}

type matchValue string

func (mv matchValue) Run(state *State) error {
	val, err := state.CompileValue(string(mv))
	if err != nil {
		return err
	}
	newValue, err := state.UnifyValue(val)
	if err != nil {
		return err
	}
	state.PushValue(newValue)
	return nil
}
