package analytics

import (
	"io"
	"net/http"
)

// Response is a proxied analytics API result.
type Response struct {
	StatusCode int
	Body       []byte
}

// WriteJSON forwards the response to an http.ResponseWriter.
func (r *Response) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.StatusCode)
	_, _ = w.Write(r.Body)
}

func readResponse(resp *http.Response) (*Response, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{StatusCode: resp.StatusCode, Body: body}, nil
}
