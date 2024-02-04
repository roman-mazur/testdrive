package testdrive

import (
	"bufio"
	"io"
	"strings"
)

type SetValue string

func (sv SetValue) Run(state *State) error {
	val, err := state.CompileValue(string(sv))
	if err != nil {
		return err
	}
	state.SetValue(val)
	return nil
}

type MatchValue string

func (mv MatchValue) Run(state *State) error {
	val, err := state.CompileValue(string(mv))
	if err != nil {
		return err
	}
	newValue, err := state.UnifyValue(val)
	if err != nil {
		return err
	}
	state.SetValue(newValue)
	return nil
}

type ValueCommand interface {
	Command
	~string
}

func ParseValueCmd[T ValueCommand](prefix string, in *bufio.Reader) (Command, int, error) {
	if !strings.HasPrefix(prefix, "^") {
		return T(prefix), 0, nil
	}

	var (
		lc     int
		end    = prefix[1:]
		buffer strings.Builder
	)

	for {
		line, err := in.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, lc, err
		}
		lc++
		line = strings.TrimSpace(line)
		if line == end {
			break
		}
		buffer.WriteString(line)
		buffer.WriteString("\n")
		if err == io.EOF {
			break
		}
	}
	return T(buffer.String()), lc, nil
}
