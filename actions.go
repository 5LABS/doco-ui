package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func (a *App) action(w http.ResponseWriter, r *http.Request) {
	kind, id, action := r.PathValue("kind"), r.PathValue("id"), r.PathValue("action")
	if !identifier.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	key := kind + "/" + id
	if _, busy := a.locks.LoadOrStore(key, true); busy {
		http.Error(w, "Aktion läuft bereits", 409)
		return
	}
	defer a.locks.Delete(key)
	// Finish an accepted operation even if its browser disconnects.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if kind == "sources" && action == "sync" && a.poll {
		for _, source := range a.sources {
			if source.ID == id {
				a.syncSource(ctx, w, source)
				return
			}
		}
	}
	if (action != "start" && action != "stop" && action != "restart" && action != "recreate") || (kind != "containers" && kind != "projects") {
		http.NotFound(w, r)
		return
	}
	inv, e := a.loadInventory()
	if e != nil {
		http.Error(w, e.Error(), 502)
		return
	}
	var path string
	cd := kind == "projects"
	if cd {
		if !a.control {
			http.Error(w, "Projektsteuerung deaktiviert", 403)
			return
		}
		service := r.URL.Query().Get("service")
		members := 0
		foundService := service == ""
		for _, c := range inv.Containers {
			if c.Project == id {
				members++
				memberKey := "containers/" + c.ID
				if _, busy := a.locks.LoadOrStore(memberKey, true); busy {
					http.Error(w, "Aktion läuft bereits", 409)
					return
				}
				defer a.locks.Delete(memberKey)
				if !c.Managed {
					http.Error(w, "Projekt nicht vollständig von doco-cd verwaltet", 409)
					return
				}
				if c.Service == service {
					foundService = true
				}
			}
		}
		projects, err := a.loadProjects(ctx)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		known := false
		for _, p := range projects {
			if p.Name == id {
				known = true
			}
		}
		if members == 0 || !known || !foundService || (service != "" && action != "recreate") {
			http.Error(w, "Projektaktion nicht verfügbar", 409)
			return
		}
		path = "/project/" + id + "/" + action + "?context=default&service=" + url.QueryEscape(service)
	} else {
		if !containerID.MatchString(id) || action == "recreate" {
			http.NotFound(w, r)
			return
		}
		found := false
		for _, c := range inv.Containers {
			if c.ID == id && containerActionAllowed(c.State, action) {
				if _, busy := a.locks.Load("projects/" + c.Project); busy {
					http.Error(w, "Aktion läuft bereits", 409)
					return
				}
				found = true
			}
		}
		if !found {
			http.Error(w, "Containeraktion im aktuellen Zustand nicht verfügbar", 409)
			return
		}
		path = "/containers/" + id + "/" + action
	}
	e = a.call(ctx, cd, "POST", path, nil, nil)
	a.inventory.clear()
	a.deployments.clear()
	if e != nil {
		http.Error(w, e.Error(), 502)
		return
	}
	respond(w, map[string]string{"status": "completed"})
}
func containerActionAllowed(state, action string) bool {
	if action == "start" {
		return state == "created" || state == "exited"
	}
	return state == "running" && (action == "restart" || action == "stop")
}

type tracked struct {
	Run   Run
	Error string
}

func (a *App) syncSource(ctx context.Context, w http.ResponseWriter, s Source) {
	if prev, ok := a.jobs.Load(s.ID); ok {
		status := prev.(tracked).Run.Status
		if status == "accepted" || status == "running" {
			http.Error(w, "Abgleich läuft bereits", 409)
			return
		}
	}
	// A poll is only useful to the operator when the key can also track its run.
	var recent envelope[[]Run]
	if err := a.call(ctx, true, "GET", "/runs?limit=1", nil, &recent); err != nil {
		http.Error(w, "Abgleich nicht verfügbar: "+err.Error(), http.StatusBadGateway)
		return
	}
	config := map[string]string{"url": s.URL}
	if s.Reference != "" {
		config["reference"] = s.Reference
	}
	if s.Target != "" {
		config["target"] = s.Target
	}
	var accepted envelope[any]
	if e := a.call(ctx, true, "POST", "/poll/run?wait=false", []map[string]string{config}, &accepted); e != nil {
		http.Error(w, e.Error(), 502)
		return
	}
	if !identifier.MatchString(accepted.ID) {
		http.Error(w, "Job-ID fehlt", 502)
		return
	}
	initial := tracked{Run: Run{ID: accepted.ID, Status: "accepted"}}
	a.jobs.Store(s.ID, initial)
	go a.track(s.ID, initial)
	respond(w, initial)
}
func (a *App) track(source string, t tracked) {
	for {
		var result envelope[Run]
		err := a.call(context.Background(), true, "GET", "/run/"+t.Run.ID, nil, &result)
		if err != nil {
			t.Error = err.Error()
			if err == statusError(404) {
				t.Run.Status = "unknown"
			}
		} else if result.Content.ID == t.Run.ID {
			t.Run = result.Content
			t.Error = ""
		}
		a.jobs.Store(source, t)
		switch t.Run.Status {
		case "succeeded", "failed", "skipped", "unknown":
			a.deployments.clear()
			return
		}
		time.Sleep(3 * time.Second)
	}
}

type inspect struct {
	Config struct{ Tty bool }
	State  struct{ Health *struct{ Status string } }
}

func (a *App) logs(w http.ResponseWriter, r *http.Request) {
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
	follow := r.URL.Query().Get("follow") == "1"
	res, e := a.request(r.Context(), false, "GET", fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&timestamps=1&tail=200&follow=%t", id, follow), nil)
	if e != nil {
		http.Error(w, e.Error(), 502)
		return
	}
	defer res.Body.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	ctrl := http.NewResponseController(w)
	ctrl.SetWriteDeadline(time.Now().Add(65 * time.Second))
	w.WriteHeader(200)
	if ctrl.Flush() != nil {
		return
	}
	remaining := int64(256 << 10)
	buf := make([]byte, 8192)
	for remaining > 0 {
		n := remaining
		if !info.Config.Tty {
			var hdr [8]byte
			if _, e = io.ReadFull(res.Body, hdr[:]); e != nil {
				return
			}
			n = min(n, int64(binary.BigEndian.Uint32(hdr[4:])))
		}
		for n > 0 {
			count, e := res.Body.Read(buf[:min(n, int64(len(buf)))])
			if count > 0 {
				if _, err := w.Write(buf[:count]); err != nil {
					return
				}
				if ctrl.Flush() != nil {
					return
				}
				n -= int64(count)
				remaining -= int64(count)
			}
			if e != nil {
				return
			}
		}
	}
	io.WriteString(w, "\n[Ausgabelimit erreicht]\n")
}

func (a *App) trackedRuns() map[string]tracked {
	result := map[string]tracked{}
	a.jobs.Range(func(k, v any) bool { result[k.(string)] = v.(tracked); return true })
	return result
}
