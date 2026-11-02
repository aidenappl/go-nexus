package nexus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestErrorsAndEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer nxa_t" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/v1/threads/APL-1/revisions":
			w.WriteHeader(409)
			fmt.Fprint(w, `{"success":false,"error":"stale_revision","error_message":"rev 3 is not the latest","error_code":409,"hint":"re-read and retry"}`)
		case "/v1/search":
			if r.URL.Query().Get("q") != "is:ready tag:repo:nexus" {
				t.Errorf("q = %q", r.URL.Query().Get("q"))
			}
			fmt.Fprint(w, `{"success":true,"message":"OK","data":[{"handle":"APL-1","priority":"p1","tags":["repo:nexus"]}]}`)
		case "/v1/tasks":
			if r.Header.Get("Idempotency-Key") != "k1" {
				t.Errorf("idempotency key missing")
			}
			fmt.Fprint(w, `{"success":true,"data":{"handle":"APL-2","rev":1},"meta":{"lint":["no acceptance criteria checkboxes"]}}`)
		}
	}))
	defer srv.Close()
	c := New("nxa_t", WithURL(srv.URL))
	ctx := context.Background()

	_, _, err := c.ReviseThread(ctx, "APL-1", Revise{BaseRev: 3})
	if !IsCode(err, "stale_revision") || !strings.Contains(err.Error(), "re-read and retry") {
		t.Fatalf("err = %v", err)
	}
	heads, err := c.Search(ctx, "is:ready tag:repo:nexus", 0)
	if err != nil || len(heads) != 1 || heads[0].Handle != "APL-1" || heads[0].Tags[0] != "repo:nexus" {
		t.Fatalf("search = %+v, %v", heads, err)
	}
	a, m, err := c.CreateTask(ctx, NewTask{Namespace: "appleby", Subject: "s", Body: "b", Priority: "p2"}, IdempotencyKey("k1"))
	if err != nil || a.Handle != "APL-2" || *a.Rev != 1 || len(m.Lint) != 1 {
		t.Fatalf("create = %+v %+v %v", a, m, err)
	}
}

// The stream resumes from Last-Event-ID, skips duplicates and reconnects after `end`.
func TestStreamResumes(t *testing.T) {
	var mu sync.Mutex
	var lastIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lastIDs = append(lastIDs, r.Header.Get("Last-Event-ID"))
		n := len(lastIDs)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if n == 1 {
			fmt.Fprint(w, ": keep-alive\n\n")
			fmt.Fprint(w, "event: article\nid: 5\ndata: {\"seq\":5,\"article\":{\"handle\":\"APL-1\"},\"thread\":{\"handle\":\"APL-1\"}}\n\n")
			fmt.Fprint(w, "event: end\ndata: {\"last\":5}\n\n")
			return
		}
		// A replay that repeats 5 must not be delivered twice.
		fmt.Fprint(w, "event: article\nid: 5\ndata: {\"seq\":5,\"article\":{\"handle\":\"APL-1\"}}\n\n")
		fmt.Fprint(w, "event: article\nid: 6\ndata: {\"seq\":6,\"article\":{\"handle\":\"APL-2\"}}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var got []int64
	_ = New("nxa_t", WithURL(srv.URL)).Stream(ctx, "ns:appleby", 4, func(ev Event, err error) {
		if err != nil {
			t.Errorf("unexpected %v", err)
			return
		}
		got = append(got, ev.Seq)
		if ev.Seq == 6 {
			cancel()
		}
	})
	if fmt.Sprint(got) != "[5 6]" {
		t.Fatalf("got %v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if lastIDs[0] != "4" || lastIDs[1] != "5" {
		t.Fatalf("Last-Event-ID sequence = %v", lastIDs)
	}
}

// Against a real server: NEXUS_TEST_URL + NEXUS_TEST_TOKEN (an nxp_ token of a namespace owner)
// + NEXUS_TEST_NS.
func TestLive(t *testing.T) {
	base, tok, ns := os.Getenv("NEXUS_TEST_URL"), os.Getenv("NEXUS_TEST_TOKEN"), os.Getenv("NEXUS_TEST_NS")
	if base == "" || tok == "" || ns == "" {
		t.Skip("set NEXUS_TEST_URL, NEXUS_TEST_TOKEN and NEXUS_TEST_NS")
	}
	ctx := context.Background()
	person := New(tok, WithURL(base))
	at, err := person.MintAgentToken(ctx, "ci", fmt.Sprintf("go-nexus-%d", time.Now().UnixNano()), "test", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	agent := New(at.Token, WithURL(base))
	me, err := agent.Self(ctx)
	if err != nil || !me.IsAgent || me.Agent != at.Agent {
		t.Fatalf("self = %+v %v", me, err)
	}
	task, _, err := agent.CreateTask(ctx, NewTask{Namespace: ns, Subject: "go-nexus live test " + at.Agent, Priority: "p5", Tags: []string{"repo:go-nexus"},
		Body: "## Context\n\nLive test.\n\n## Acceptance criteria\n\n- [ ] passes"})
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan Event, 4)
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		_ = agent.Stream(sctx, "ns:"+ns+" id:"+task.Handle, task.Seq, func(ev Event, err error) {
			if err == nil {
				events <- ev
			}
		})
	}()
	time.Sleep(300 * time.Millisecond)
	if err := agent.Claim(ctx, task.Handle, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-events:
		if ev.Thread.Handle != task.Handle || ev.Article.Status != "in_progress" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no stream event for the claim")
	}
	if _, err := agent.Followup(ctx, task.Handle, "progress"); err != nil {
		t.Fatal(err)
	}
	if err := agent.Release(ctx, task.Handle, "done", "live test passed"); err != nil {
		t.Fatal(err)
	}
	md, err := agent.ThreadMarkdown(ctx, task.Handle)
	if err != nil || !strings.Contains(md, "progress") || !strings.Contains(md, "approval") {
		t.Fatalf("markdown = %q %v", md, err)
	}
	if err := agent.RevokeSelf(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Self(ctx); err == nil {
		t.Fatal("revoked token still works")
	}
}
