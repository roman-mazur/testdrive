package testdrive

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var httpSyntaxError = fmt.Errorf("expected [^<END>] <VERB> <URL>")

// ParseHTTP is a parse for HTTP command.
// The first (that starts with an HTTP prefix) provides a method and a URL.
// Next lines provide headers.
// After a blank line, the request body follows.
// HTTP command produces the following value:
//
//	status: { code: int, line: string }
//	headers: [string]: [...string]
//	body: _
func ParseHTTP(prefix string, in *bufio.Reader) (Command, int, error) {
	parts := strings.Fields(prefix)

	var end, verb, urlString string
	switch {
	case len(parts) == 3 && strings.HasPrefix(parts[0], "^"):
		end, verb, urlString = parts[0][1:], parts[1], parts[2]
	case len(parts) == 2:
		verb, urlString = parts[0], parts[1]
	default:
		return nil, 0, httpSyntaxError
	}

	req, err := http.NewRequest(verb, urlString, nil)
	if err != nil {
		return nil, 0, err
	}

	var (
		headers []string
		body    bytes.Buffer
	)
	headersFinished := false
	lc, err := readLines(in, end, func(line string) {
		if line == "" {
			headersFinished = true
			return
		}
		if headersFinished {
			body.WriteString(line)
			body.WriteByte('\n')
		} else {
			headers = append(headers, line)
		}
	})
	if err != nil {
		return nil, lc, err
	}

	req.Body = io.NopCloser(&body)
	for _, headerLine := range headers {
		parts := strings.SplitN(headerLine, " ", 2)
		name, value := strings.TrimSuffix(parts[0], ":"), parts[1]
		req.Header.Set(name, value)
	}
	return &httpCommand{r: req}, 0, nil
}

type httpCommand struct {
	r *http.Request
}

func (hc *httpCommand) Run(state *State) error {
	client := state.engine.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	req := hc.r.Clone(state.Context)
	if state.engine.baseURL != "" && req.URL.Scheme == "" {
		var urlError error
		req.URL, urlError = url.Parse(state.engine.baseURL + req.URL.String())
		if urlError != nil {
			return urlError
		}
	}

	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		if len(body) > 0 {
			expandedBody, err := state.Expand(string(body))
			if err != nil {
				return err
			}
			req.Body = io.NopCloser(strings.NewReader(expandedBody))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var value httpCommandResult
	value.Status.Code = resp.StatusCode
	value.Status.Line = resp.Status
	value.Headers = resp.Header
	if strings.Contains(resp.Header.Get("content-type"), "json") {
		if err := json.NewDecoder(resp.Body).Decode(&value.Body); err != nil {
			return fmt.Errorf("failed to parse as JSON: %s", err)
		}
	} else {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response fully: %s", err)
		}
		value.Body = string(data)
	}

	val := state.cueCtx.Encode(&value)
	if err := val.Err(); err != nil {
		return err
	}
	state.PushValue(val)
	return nil
}

// Note that this structure is documented in ParseHTTP.
type httpCommandResult struct {
	Status struct {
		Code int    `json:"code"`
		Line string `json:"line"`
	} `json:"status"`

	Headers http.Header `json:"headers"`

	Body any `json:"body"`
}
