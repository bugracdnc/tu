package gcal

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var (
	authURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL  = "https://oauth2.googleapis.com/token"
	revokeURL = "https://oauth2.googleapis.com/revoke"
)

// Scopes allow managing events and reading the list of calendars.
const Scopes = "https://www.googleapis.com/auth/calendar.events https://www.googleapis.com/auth/calendar.readonly"

// ReadClientFile reads the client ID and secret from the JSON file that the
// Google Cloud console offers for download when creating an OAuth client.
func ReadClientFile(path string) (id, secret string, err error) {
	byt, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	type client struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	var f struct {
		Installed *client `json:"installed"`
		Web       *client `json:"web"`
	}
	if err := json.Unmarshal(byt, &f); err != nil {
		return "", "", fmt.Errorf("%s is not a Google client file: %w", path, err)
	}
	c := f.Installed
	if c == nil {
		c = f.Web
	}
	if c == nil || c.ClientID == "" {
		return "", "", fmt.Errorf("%s has no client_id, download it from the OAuth client's page", path)
	}
	return c.ClientID, c.ClientSecret, nil
}

// Login runs the OAuth flow. The browser is sent back to a server on the
// loopback interface; when that can't be reached (e.g. logging in on another
// device) the redirected address or the code can be pasted into `in`.
func (s *State) Login(ctx context.Context, openBrowser bool, in io.Reader, out io.Writer) error {
	if s.ClientID == "" {
		return errors.New("no OAuth client configured")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	redirect := fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)

	verifier := randomString(48)
	challenge := sha256.Sum256([]byte(verifier))
	state := randomString(16)
	link := authURL + "?" + url.Values{
		"client_id":             {s.ClientID},
		"redirect_uri":          {redirect},
		"response_type":         {"code"},
		"scope":                 {Scopes},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"access_type":           {"offline"},
		"prompt":                {"consent"},
	}.Encode()

	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)
	send := func(r result) {
		select {
		case results <- r:
		default:
		}
	}

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !q.Has("code") && !q.Has("error") {
			http.NotFound(w, r)
			return
		}
		code, err := codeFromQuery(q, state)
		if err != nil {
			http.Error(w, "tu: "+err.Error(), http.StatusBadRequest)
		} else {
			fmt.Fprintln(w, "tu is now connected to Google Calendar. You can close this tab.")
		}
		send(result{code, err})
	})}
	go srv.Serve(ln)
	defer srv.Close()

	go func() {
		sc := bufio.NewScanner(in)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			if !strings.Contains(line, "code=") {
				send(result{code: line})
				return
			}
			u, err := url.Parse(line)
			if err != nil {
				send(result{err: err})
				return
			}
			code, err := codeFromQuery(u.Query(), state)
			send(result{code, err})
			return
		}
	}()

	fmt.Fprintln(out, "Open this address to allow tu to use your Google Calendar:")
	fmt.Fprintln(out)
	fmt.Fprintln(out, link)
	fmt.Fprintln(out)
	if openBrowser && launchBrowser(link) == nil {
		fmt.Fprintln(out, "A browser window should have opened.")
	}
	fmt.Fprintln(out, "If the browser ends up on a page that can't be reached, copy its full address")
	fmt.Fprintln(out, "from the address bar and paste it here:")

	var r result
	select {
	case r = <-results:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.err != nil {
		return r.err
	}

	tok, err := s.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {r.code},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	})
	if err != nil {
		return err
	}
	s.Token = tok
	return nil
}

// Logout revokes the tokens (best effort) and forgets them.
func (s *State) Logout(ctx context.Context) {
	if s.Token != nil && s.Token.RefreshToken != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL,
			strings.NewReader(url.Values{"token": {s.Token.RefreshToken}}.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if resp, err := http.DefaultClient.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	}
	s.Token = nil
	s.Account = ""
}

func codeFromQuery(q url.Values, state string) (string, error) {
	if e := q.Get("error"); e != "" {
		return "", fmt.Errorf("google refused access: %s", e)
	}
	if q.Get("state") != state {
		return "", errors.New("the address belongs to a different login attempt, try again")
	}
	if q.Get("code") == "" {
		return "", errors.New("the address holds no code")
	}
	return q.Get("code"), nil
}

// accessToken returns a valid access token, refreshing it when needed.
func (s *State) accessToken(ctx context.Context, force bool) (string, error) {
	if !s.LoggedIn() {
		return "", ErrNotLoggedIn
	}
	if !force && s.Token.AccessToken != "" && time.Until(s.Token.Expiry) > time.Minute {
		return s.Token.AccessToken, nil
	}
	tok, err := s.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {s.Token.RefreshToken},
	})
	if err != nil {
		return "", err
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = s.Token.RefreshToken
	}
	s.Token = tok
	return tok.AccessToken, s.Save()
}

func (s *State) requestToken(ctx context.Context, form url.Values) (*Token, error) {
	form.Set("client_id", s.ClientID)
	if s.ClientSecret != "" {
		form.Set("client_secret", s.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("unexpected token response (%s): %w", resp.Status, err)
	}
	if body.Error != "" {
		if body.Error == "invalid_grant" && form.Get("grant_type") == "refresh_token" {
			return nil, errors.New("google access has expired or was revoked, run: tu google login")
		}
		return nil, fmt.Errorf("google token error: %s %s", body.Error, body.Description)
	}
	return &Token{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
	}, nil
}

func launchBrowser(link string) error {
	var cmds [][]string
	switch runtime.GOOS {
	case "darwin":
		cmds = [][]string{{"open"}}
	case "windows":
		cmds = [][]string{{"rundll32", "url.dll,FileProtocolHandler"}}
	default:
		cmds = [][]string{{"termux-open-url"}, {"xdg-open"}, {"wslview"}}
	}
	for _, c := range cmds {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		cmd := exec.Command(c[0], append(c[1:], link)...)
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return nil
		}
	}
	return errors.New("no browser opener found")
}

func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
