package nexus

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Event is one streamed article that matched the query.
type Event struct {
	Seq     int64   `json:"seq"`
	Article Article `json:"article"`
	Thread  Head    `json:"thread"`
}

// ErrReset is passed to the handler when the server cannot replay from the last id (more than
// 10 000 events behind). Re-read with Search, then the stream continues from now.
var ErrReset = errors.New("nexus: stream reset; re-read with Search")

// Stream follows /v1/stream?q= until ctx ends, calling fn for every event in order. It
// resumes after since (0 = from now), reconnects with Last-Event-ID after the server's 30-minute
// limit, a lag disconnect or a network error (with backoff), so fn sees every event once.
// fn is called with ErrReset (and a zero Event) when replay was impossible.
func (c *Client) Stream(ctx context.Context, q string, since int64, fn func(Event, error)) error {
	last := since
	backoff := time.Second
	client := &http.Client{Transport: c.http.Transport} // no overall timeout: streams are long
	for ctx.Err() == nil {
		got, err := c.streamOnce(ctx, client, q, &last, fn)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 429 {
			return err
		}
		if got {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, time.Minute)
	}
	return ctx.Err()
}

func (c *Client) streamOnce(ctx context.Context, client *http.Client, q string, last *int64, fn func(Event, error)) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/stream?"+url.Values{"q": {q}}.Encode(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "text/event-stream")
	if *last > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatInt(*last, 10))
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var env envelope
		_ = json.Unmarshal(raw, &env)
		return false, &Error{Status: resp.StatusCode, Code: env.Error, Message: env.ErrorMessage, Hint: env.Hint}
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	var event, id string
	var data strings.Builder
	got := false
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if event != "" || data.Len() > 0 {
				got = true
				if done := dispatch(event, id, data.String(), last, fn); done {
					return got, nil
				}
			}
			event, id = "", ""
			data.Reset()
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "id:"):
			id = strings.TrimSpace(line[3:])
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(line[5:], " "))
		}
	}
	return got, sc.Err()
}

// dispatch handles one SSE event; it reports whether the server is closing this connection.
func dispatch(event, id, data string, last *int64, fn func(Event, error)) bool {
	switch event {
	case "article":
		var ev Event
		if json.Unmarshal([]byte(data), &ev) != nil {
			return false
		}
		if ev.Seq <= *last {
			return false
		}
		fn(ev, nil)
		*last = ev.Seq
		if n, err := strconv.ParseInt(id, 10, 64); err == nil && n > *last {
			*last = n
		}
	case "reset":
		fn(Event{}, ErrReset)
		*last = 0
		return true
	case "end", "lagged":
		return true
	}
	return false
}
