package testdrive

import (
	"bufio"
	"io"
	"strings"
)

func readLines(in *bufio.Reader, end string, f func(line string)) (int, error) {
	lc := 0
	for {
		line, err := in.ReadString('\n')
		if err != nil && err != io.EOF {
			return lc, err
		}
		lc++
		line = strings.TrimSpace(line)
		if line == end {
			break
		}
		f(line)
		if err == io.EOF {
			break
		}
	}
	return lc, nil
}
