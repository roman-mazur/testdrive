package testserver

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func CreateHandler() *http.ServeMux {
	mux := new(http.ServeMux)
	mux.HandleFunc("/time", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-type", "application/json")
		resp := struct {
			Now time.Time `json:"now"`
		}{
			Now: time.Now(),
		}
		_ = json.NewEncoder(rw).Encode(resp)
	})
	mux.HandleFunc("/echo", func(rw http.ResponseWriter, r *http.Request) {
		for name, values := range r.Header {
			rw.Header().Set(name, values[0])
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = io.Copy(rw, r.Body)
	})
	return mux
}
