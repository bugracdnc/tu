package gcal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// linkWriter hands over the authorization link once Login prints it.
type linkWriter struct {
	once sync.Once
	link chan *url.URL
}

var linkRe = regexp.MustCompile(`https?://\S+`)

func (w *linkWriter) Write(p []byte) (int, error) {
	if m := linkRe.Find(p); m != nil {
		u, _ := url.Parse(string(m))
		w.once.Do(func() { w.link <- u })
	}
	return len(p), nil
}

func fakeTokenServer(t *testing.T, wantCode string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("code") != wantCode || r.Form.Get("code_verifier") == "" {
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "refresh_token": "rt", "expires_in": 3600})
	}))
	t.Cleanup(srv.Close)
	old := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = old })
}

func TestLoginRedirect(t *testing.T) {
	fakeTokenServer(t, "c0de")
	s := &State{ClientID: "id"}
	w := &linkWriter{link: make(chan *url.URL, 1)}
	go func() {
		q := (<-w.link).Query()
		resp, err := http.Get(q.Get("redirect_uri") + "?code=c0de&state=" + q.Get("state"))
		if err == nil {
			resp.Body.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	in, _ := io.Pipe()
	if err := s.Login(ctx, false, in, w); err != nil {
		t.Fatal(err)
	}
	if !s.LoggedIn() || s.Token.AccessToken != "at" {
		t.Errorf("got token %+v", s.Token)
	}
}

func TestLoginPaste(t *testing.T) {
	fakeTokenServer(t, "pasted")
	s := &State{ClientID: "id"}
	w := &linkWriter{link: make(chan *url.URL, 1)}
	in, paste := io.Pipe()
	go func() {
		q := (<-w.link).Query()
		io.WriteString(paste, "\n"+q.Get("redirect_uri")+"?state="+q.Get("state")+"&code=pasted&scope=x\n")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Login(ctx, false, in, w); err != nil {
		t.Fatal(err)
	}
	if !s.LoggedIn() {
		t.Error("not logged in")
	}
}

func TestLoginWrongState(t *testing.T) {
	fakeTokenServer(t, "c0de")
	s := &State{ClientID: "id"}
	w := &linkWriter{link: make(chan *url.URL, 1)}
	in, paste := io.Pipe()
	go func() {
		q := (<-w.link).Query()
		io.WriteString(paste, q.Get("redirect_uri")+"?state=other&code=c0de\n")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Login(ctx, false, in, w); err == nil || !strings.Contains(err.Error(), "different login") {
		t.Errorf("wanted a state error, got %v", err)
	}
}
