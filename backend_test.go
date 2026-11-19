package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testHandler(t *testing.T, a *App) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	a.routes(mux)
	return a.protect(mux)
}
func post(h http.Handler, path string, auth, header bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", base+path, nil)
	if auth {
		r.SetBasicAuth("admin", "test")
	}
	if header {
		r.Header.Set("X-Doco-UI", "1")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestWriteBoundary(t *testing.T) {
	a := &App{password: "test"}
	h := testHandler(t, a)
	for _, tc := range []struct {
		auth, header, write bool
		want                int
	}{{false, true, true, 401}, {true, true, false, 403}, {true, false, true, 403}, {true, true, true, 404}} {
		a.write = tc.write
		if w := post(h, "/actions/containers/test/delete", tc.auth, tc.header); w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
}

func TestPollUsesSelectedSourceAndFollowsJob(t *testing.T) {
	for _, terminal := range []string{"succeeded", "failed", "skipped"} {
		t.Run(terminal, func(t *testing.T) {
			var calls atomic.Int32
			release := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") != "secret" {
					t.Error("missing backend key")
				}
				switch r.URL.Path {
				case "/v1/api/runs":
					io.WriteString(w, `{"content":[]}`)
				case "/v1/api/poll/run":
					calls.Add(1)
					var items []map[string]string
					if err := json.NewDecoder(r.Body).Decode(&items); err != nil || len(items) != 1 || items[0]["url"] != "https://example.org/b.git" || items[0]["reference"] != "stage" || items[0]["target"] != "cluster" || r.URL.Query().Get("wait") != "false" {
						t.Errorf("unexpected poll: %v", items)
					}
					w.WriteHeader(202)
					io.WriteString(w, `{"job_id":"job-b","content":"poll jobs started"}`)
				case "/v1/api/run/job-b":
					<-release
					respond(w, map[string]any{"content": map[string]string{"job_id": "job-b", "status": terminal}})
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			a := &App{password: "test", write: true, poll: true, key: "secret", endpoint: upstream.URL, cd: upstream.Client(), sources: []Source{{ID: "first", URL: "https://example.org/a.git"}, {ID: "second", URL: "https://example.org/b.git", Reference: "stage", Target: "cluster"}}}
			h := testHandler(t, a)
			first := post(h, "/actions/sources/second/sync", true, true)
			duplicate := post(h, "/actions/sources/second/sync", true, true)
			close(release)
			if first.Code != 200 || !strings.Contains(first.Body.String(), "accepted") || duplicate.Code != 409 {
				t.Fatalf("unexpected responses: %v %v", first, duplicate)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				if a.trackedRuns()["second"].Run.Status == terminal && calls.Load() == 1 {
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatalf("did not track %s: %v", terminal, a.trackedRuns())
		})
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestInventoryWithOfflineDocoAndMixedManagement(t *testing.T) {
	id := strings.Repeat("a", 64)
	other := strings.Repeat("b", 64)
	var posts atomic.Int32
	a := &App{password: "test", write: true, control: true, docker: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		var data string
		switch r.URL.Path {
		case "/v1.44/containers/json":
			if r.URL.Query().Get("all") != "1" {
				t.Error("stopped containers omitted")
			}
			data = `[{"Id":"` + id + `","Names":["/worker"],"State":"exited","Labels":{"com.docker.compose.project":"demo","cd.doco.metadata.manager":"doco-cd"}},{"Id":"` + other + `","Labels":{"com.docker.compose.project":"demo"}}]`
		case "/v1.44/networks":
			data = `[{"Id":"network","Name":"default"}]`
		case "/v1.44/containers/" + id + "/start":
			posts.Add(1)
		default:
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})}}
	inv, e := a.loadInventory()
	if e != nil || len(inv.Networks) != 1 || len(inv.Containers) != 2 || !inv.Containers[0].Managed || inv.Containers[1].Managed || inv.Containers[0].Name != "worker" {
		t.Fatalf("unexpected inventory: %+v, %v", inv, e)
	}
	cd, _ := a.loadDeployments()
	if cd.RunsError == "" || cd.ProjectsError == "" {
		t.Fatal("missing source error")
	}
	h := testHandler(t, a)
	if w := post(h, "/actions/projects/demo/stop", true, true); w.Code != 409 {
		t.Fatal(w)
	}
	if w := post(h, "/actions/containers/"+id+"/start", true, true); w.Code != 200 || posts.Load() != 1 {
		t.Fatal(w)
	}
}
func TestLogsStripDockerFraming(t *testing.T) {
	a := &App{docker: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		data := `{"Config":{"Tty":false}}`
		if strings.HasSuffix(r.URL.Path, "/logs") {
			data = "\x01\x00\x00\x00\x00\x00\x00\x03one\x02\x00\x00\x00\x00\x00\x00\x03two"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})}}
	r := httptest.NewRequest("GET", "/", nil)
	r.SetPathValue("id", strings.Repeat("a", 64))
	w := httptest.NewRecorder()
	a.logs(w, r)
	if w.Body.String() != "onetwo" {
		t.Fatal(w.Body.String())
	}
}
func TestSharedMetrics(t *testing.T) {
	var calls atomic.Int32
	a := &App{docker: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		data := `{"cpu_stats":{"cpu_usage":{"total_usage":500},"system_cpu_usage":3000,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":250},"system_cpu_usage":2000},"memory_stats":{"usage":12345}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})}, samples: newMetrics()}
	s := a.samples.acquire("c")
	v, e := a.sample("c", s)
	if e != nil || *v[0].CPU != 50 || v[0].Bytes != 12345 {
		t.Fatalf("bad sample %v", v)
	}
	a.sample("c", a.samples.acquire("c"))
	if calls.Load() != 1 {
		t.Fatal("sample not shared")
	}
	for i := range 63 {
		a.samples.acquire(string(rune(i)))
	}
	if a.samples.acquire("overflow") != nil {
		t.Fatal("unbounded retention")
	}
}

func TestRecreateRequiresKnownServiceAndManagedProject(t *testing.T) {
	var actions atomic.Int32
	cd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/projects":
			io.WriteString(w, `{"content":[{"Name":"demo","Status":"running(1)"}]}`)
		case "/v1/api/runs":
			io.WriteString(w, `{"content":[]}`)
		case "/v1/api/project/demo/recreate":
			if r.Method != "POST" || r.URL.Query().Get("context") != "default" {
				t.Error("incorrect project scope")
			}
			if service := r.URL.Query().Get("service"); service != "" && service != "web" {
				t.Error("incorrect service")
			}
			actions.Add(1)
			io.WriteString(w, `{"content":"completed"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer cd.Close()
	a := &App{password: "test", write: true, control: true, endpoint: cd.URL, cd: cd.Client(), docker: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		data := "[]"
		if strings.HasSuffix(r.URL.Path, "/containers/json") {
			data = `[{"Labels":{"com.docker.compose.project":"demo","com.docker.compose.service":"web","cd.doco.metadata.manager":"doco-cd"}}]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})}}
	h := testHandler(t, a)
	for _, suffix := range []string{"", "?service=web"} {
		if w := post(h, "/actions/projects/demo/recreate"+suffix, true, true); w.Code != 200 {
			t.Fatal(w)
		}
	}
	if w := post(h, "/actions/projects/demo/recreate?service=other", true, true); w.Code != 409 || actions.Load() != 2 {
		t.Fatal("unknown service accepted")
	}
}

func TestProjectAndMemberActionsConflict(t *testing.T) {
	id := strings.Repeat("a", 64)
	project := "/actions/projects/demo/stop"
	member := "/actions/containers/" + id + "/restart"
	for _, tc := range []struct{ first, second string }{{project, member}, {member, project}} {
		t.Run(tc.first, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var posts atomic.Int32
			client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				data := "[]"
				switch {
				case r.Method == "POST":
					if posts.Add(1) == 1 {
						close(entered)
						<-release
					}
				case strings.HasSuffix(r.URL.Path, "/containers/json"):
					data = `[{"Id":"` + id + `","State":"running","Labels":{"com.docker.compose.project":"demo","cd.doco.metadata.manager":"doco-cd"}}]`
				case strings.HasSuffix(r.URL.Path, "/projects"):
					data = `{"content":[{"Name":"demo"}]}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
			})}
			a := &App{docker: client, cd: client, endpoint: "http://cd", password: "test", write: true, control: true}
			h := testHandler(t, a)
			done := make(chan int, 1)
			go func() { done <- post(h, tc.first, true, true).Code }()
			select {
			case <-entered:
			case code := <-done:
				t.Fatalf("first action returned early: %d", code)
			case <-time.After(time.Second):
				close(release)
				t.Fatal("first action did not reach upstream")
			}
			code := post(h, tc.second, true, true).Code
			close(release)
			if first := <-done; first != 200 || code != 409 || posts.Load() != 1 {
				t.Fatalf("responses %d, %d; forwarded actions %d", first, code, posts.Load())
			}
			if got := post(h, tc.second, true, true).Code; got != 200 {
				t.Fatalf("action after release: %d", got)
			}
		})
	}
}

func TestRunHistoryIndependentOfProjectDiscovery(t *testing.T) {
	runsRequested := make(chan struct{})
	a := &App{endpoint: "http://cd", cd: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/projects") {
			select {
			case <-runsRequested:
			case <-time.After(time.Second):
				t.Error("project lookup blocked run lookup")
			}
			return &http.Response{StatusCode: 503, Body: http.NoBody}, nil
		}
		close(runsRequested)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"content":[{"job_id":"run"}]}`))}, nil
	})}}
	d, _ := a.loadDeployments()
	if d.ProjectsError == "" || d.RunsError != "" || len(d.Runs) != 1 || d.Runs[0].ID != "run" {
		t.Fatalf("unexpected deployments: %+v", d)
	}
}

type failingLogWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *failingLogWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func (w *failingLogWriter) FlushError() error { return io.ErrClosedPipe }

func TestLogsBoundWritesAndStopOnFlushFailure(t *testing.T) {
	a := &App{docker: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		data := `{"Config":{"Tty":true}}`
		if strings.HasSuffix(r.URL.Path, "/logs") {
			data = "log content"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})}}
	r := httptest.NewRequest("GET", "/", nil)
	r.SetPathValue("id", strings.Repeat("a", 64))
	w := &failingLogWriter{ResponseRecorder: httptest.NewRecorder()}
	a.logs(w, r)
	if w.deadline.IsZero() || time.Until(w.deadline) > 65*time.Second || w.Body.Len() != 0 {
		t.Fatalf("deadline %v, body %q", w.deadline, w.Body)
	}
}
