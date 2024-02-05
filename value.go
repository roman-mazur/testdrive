package testdrive

import (
	"bufio"
	"io"
	"strings"
)

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

type setValue string

func (sv setValue) Run(state *State) error {
	val, err := state.CompileValue(string(sv))
	if err != nil {
		return err
	}
	state.SetValue(val)
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
	state.SetValue(newValue)
	return nil
}
