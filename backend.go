package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const base = "/api/targets/local"

var identifier = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$")
var containerID = regexp.MustCompile("^[a-f0-9]{64}$")

type Source struct{ ID, Name, URL, Reference, Target, Scope string }
type App struct {
	docker, cd              *http.Client
	endpoint, key, password string
	write, control, poll    bool
	sources                 []Source
	inventory               cache[Inventory]
	deployments             cache[Deployments]
	locks                   sync.Map
	jobs                    sync.Map
	samples                 *metrics
}

func setting(name string) (string, error) {
	if path := os.Getenv(name + "_FILE"); path != "" {
		b, e := os.ReadFile(path)
		return strings.TrimSpace(string(b)), e
	}
	return os.Getenv(name), nil
}
func newApp() (*App, error) {
	a := &App{
		endpoint: strings.TrimRight(os.Getenv("DOCO_URL"), "/"),
		write:    os.Getenv("UI_WRITE") == "true",
		control:  os.Getenv("DOCO_CONTROL") == "true",
		poll:     os.Getenv("DOCO_POLL") == "true",
		samples:  newMetrics(),
	}
	var err error
	if a.password, err = setting("UI_PASSWORD"); err != nil || a.password == "" {
		return nil, errors.New("UI_PASSWORD oder UI_PASSWORD_FILE erforderlich")
	}
	if a.key, err = setting("DOCO_KEY"); err != nil {
		return nil, err
	}
	raw, err := setting("GITOPS_SOURCES")
	if err != nil {
		return nil, err
	}
	if raw != "" {
		if err = json.Unmarshal([]byte(raw), &a.sources); err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	for _, s := range a.sources {
		u, e := url.Parse(s.URL)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !identifier.MatchString(s.ID) || seen[s.ID] || s.Scope == "" || s.Name == "" {
			return nil, errors.New("ungültige GitOps-Quelle (ID, Name, HTTPS-URL und Scope erforderlich)")
		}
		seen[s.ID] = true
	}
	socket := os.Getenv("DOCKER_SOCKET")
	if socket == "" {
		socket = "/var/run/docker.sock"
	}
	a.docker = &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}, ResponseHeaderTimeout: 55 * time.Second}, Timeout: 65 * time.Second}
	a.cd = &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return a, nil
}
func (a *App) protect(next http.Handler) http.Handler {
	pass := sha256.Sum256([]byte(a.password))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, p, ok := r.BasicAuth()
		h := sha256.Sum256([]byte(p))
		if !ok || user != "admin" || subtle.ConstantTimeCompare(pass[:], h[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="doco-ui", charset="UTF-8"`)
			http.Error(w, "Anmeldung erforderlich", 401)
			return
		}
		if r.Method == "POST" && (!a.write || r.Header.Get("X-Doco-UI") != "1") {
			http.Error(w, "Schreibzugriff gesperrt", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

var errNoProjects = errors.New("keine Compose-Projekte")

type statusError int

func (e statusError) Error() string { return fmt.Sprintf("Datenquelle meldet HTTP %d", e) }
func (a *App) request(ctx context.Context, cd bool, method, path string, body any) (*http.Response, error) {
	endpoint, client := "http://docker/v1.44", a.docker
	if cd {
		if a.endpoint == "" {
			return nil, errors.New("doco-cd ist nicht konfiguriert")
		}
		endpoint, client = a.endpoint+"/v1/api", a.cd
	}
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		reader = bytes.NewReader(b)
	}
	r, e := http.NewRequestWithContext(ctx, method, endpoint+path, reader)
	if e != nil {
		return nil, e
	}
	if cd {
		r.Header.Set("x-api-key", a.key)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	res, e := client.Do(r)
	if e != nil {
		return nil, errors.New("Datenquelle nicht erreichbar")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		defer res.Body.Close()
		if cd && path == "/projects?all=true" && res.StatusCode == 404 {
			var reply struct{ Error string }
			if json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&reply) == nil && reply.Error == "no projects found" {
				return nil, errNoProjects
			}
		}

		return nil, statusError(res.StatusCode)
	}
	return res, nil
}
func (a *App) call(ctx context.Context, cd bool, method, path string, body, out any) error {
	res, e := a.request(ctx, cd, method, path, body)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(out)
	}
	return nil
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type cache[T any] struct {
	mu    sync.Mutex
	at    time.Time
	value T
	err   error
}

func (c *cache[T]) get(load func() (T, error)) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.at) > 4*time.Second {
		c.value, c.err = load()
		c.at = time.Now()
	}
	return c.value, c.err
}
func (c *cache[T]) clear() { c.mu.Lock(); defer c.mu.Unlock(); c.at = time.Time{} }

type Mount struct {
	Type, Source, Destination string
	RW                        bool
}
type Port struct {
	IP, Type                string
	PrivatePort, PublicPort int
}
type Endpoint struct{ EndpointID, IPAddress, GlobalIPv6Address string }
type Container struct {
	ID, Name, Project, Service, State, Status, Image string
	Managed                                          bool
	Mounts                                           []Mount
	Ports                                            []Port
	Networks                                         map[string]Endpoint
}
type Network struct {
	Name, ID, Driver string
	IPAM             struct{ Config []struct{ Subnet string } }
}
type Inventory struct {
	Containers []Container
	Networks   []Network
}

func (a *App) loadInventory() (Inventory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var raw []struct {
		Container
		Names           []string
		Labels          map[string]string
		NetworkSettings struct{ Networks map[string]Endpoint }
	}
	inv := Inventory{Containers: []Container{}}
	if e := a.call(ctx, false, "GET", "/containers/json?all=1", nil, &raw); e != nil {
		return inv, e
	}
	for _, item := range raw {
		c := item.Container
		if len(item.Names) > 0 {
			c.Name = strings.TrimPrefix(item.Names[0], "/")
		}
		c.Managed = item.Labels["cd.doco.metadata.manager"] == "doco-cd"
		c.Project = item.Labels["com.docker.compose.project"]
		c.Service = item.Labels["com.docker.compose.service"]
		c.Networks = item.NetworkSettings.Networks
		inv.Containers = append(inv.Containers, c)
	}
	err := a.call(ctx, false, "GET", "/networks", nil, &inv.Networks)
	return inv, err
}

type Run struct {
	ID                          string `json:"job_id"`
	Status, Repository, Trigger string
	Created                     string `json:"created_at"`
	Deployments                 []struct{ Stack, Context string }
}
type envelope[T any] struct {
	Content T
	ID      string `json:"job_id"`
}
type Project struct{ Name, Status string }
type Deployments struct {
	Projects                 []Project
	Runs                     []Run
	ProjectsError, RunsError string
}

func (a *App) loadDeployments() (Deployments, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var p envelope[[]Project]
	var runs envelope[[]Run]
	var d Deployments
	e := a.call(ctx, true, "GET", "/projects?all=true", nil, &p)
	if e != nil && e != errNoProjects {
		d.ProjectsError = e.Error()
	}
	if e = a.call(ctx, true, "GET", "/runs?limit=30", nil, &runs); e != nil {
		d.RunsError = e.Error()
	}
	d.Projects, d.Runs = p.Content, runs.Content
	return d, nil
}
func (a *App) routes(m *http.ServeMux) {
	m.HandleFunc("GET "+base+"/jobs", func(w http.ResponseWriter, r *http.Request) { respond(w, a.trackedRuns()) })
	m.HandleFunc("GET "+base+"/inventory", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.inventory.get(a.loadInventory)
		if e != nil {
			http.Error(w, e.Error(), 502)
			return
		}
		respond(w, v)
	})
	m.HandleFunc("GET "+base+"/deployments", func(w http.ResponseWriter, r *http.Request) {
		v, _ := a.deployments.get(a.loadDeployments)
		respond(w, v)
	})
	m.HandleFunc("GET "+base+"/settings", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]any{"write": a.write, "control": a.write && a.control, "poll": a.write && a.poll, "sources": a.sources})
	})
	m.HandleFunc("POST "+base+"/actions/{kind}/{id}/{action}", a.action)
	m.HandleFunc("GET "+base+"/containers/{id}/logs", a.logs)
	m.HandleFunc("GET "+base+"/containers/{id}/metrics", a.streamMetrics)
	m.HandleFunc("GET "+base+"/containers/{id}/health", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !containerID.MatchString(id) {
			http.NotFound(w, r)
			return
		}
		var info inspect
		if e := a.call(r.Context(), false, "GET", "/containers/"+id+"/json", nil, &info); e != nil {
			http.Error(w, e.Error(), 502)
			return
		}
		respond(w, info.State.Health)
	})
}
