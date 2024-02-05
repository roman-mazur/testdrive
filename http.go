package testdrive

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func ParseHTTP(prefix string, in *bufio.Reader) (Command, int, error) {
	parts := strings.Fields(prefix)
	if len(parts) != 2 {
		return nil, 0, fmt.Errorf("expected <VERB> <URL>")
	}
	verb, urlString := parts[0], parts[1]

	req, err := http.NewRequest(verb, urlString, nil)
	if err != nil {
		return nil, 0, err
	}

	// TODO: Parse header/body.
	return &httpCommand{r: req}, 0, nil
}

type httpCommand struct {
	r *http.Request
}

func (hc *httpCommand) Run(state *State) error {
	client := state.engine.HttpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(hc.r.Clone(state.Context))
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
	state.SetValue(val)
	return nil
}

type httpCommandResult struct {
	Status struct {
		Code int    `json:"code"`
		Line string `json:"line"`
	} `json:"status"`

	Headers http.Header `json:"headers"`

	Body any `json:"body"`
}
