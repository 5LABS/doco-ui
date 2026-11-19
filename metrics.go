package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type cpu struct {
	Usage struct {
		Total uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	System uint64 `json:"system_cpu_usage"`
	Count  int    `json:"online_cpus"`
}
type stats struct {
	CPU      cpu                    `json:"cpu_stats"`
	Previous cpu                    `json:"precpu_stats"`
	Memory   struct{ Usage uint64 } `json:"memory_stats"`
}
type sample struct {
	Time  time.Time
	CPU   *float64
	Bytes uint64
}
type series struct {
	mu      sync.Mutex
	history []sample
	at      time.Time
	err     error
	viewers int
	expiry  *time.Timer
}
type metrics struct {
	mu     sync.Mutex
	series map[string]*series
}

func newMetrics() *metrics { return &metrics{series: map[string]*series{}} }
func (m *metrics) acquire(id string) *series {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.series[id]; s != nil {
		if s.expiry != nil {
			s.expiry.Stop()
			s.expiry = nil
		}
		s.viewers++
		return s
	}
	if len(m.series) >= 64 {
		old := ""
		var oldest time.Time
		for k, s := range m.series {
			if s.viewers == 0 && (old == "" || s.at.Before(oldest)) {
				old = k
				oldest = s.at
			}
		}
		if old == "" {
			return nil
		}
		if timer := m.series[old].expiry; timer != nil {
			timer.Stop()
		}
		delete(m.series, old)
	}
	s := &series{viewers: 1}
	m.series[id] = s
	return s
}
func (m *metrics) release(id string, s *series) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.viewers--
	if s.viewers == 0 {
		s.expiry = time.AfterFunc(5*time.Minute, func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.series[id] == s && s.viewers == 0 && time.Since(s.at) >= 5*time.Minute {
				delete(m.series, id)
			}
		})
	}
}
func (a *App) sample(id string, s *series) ([]sample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.at) >= 5*time.Second {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		var v stats
		s.err = a.call(ctx, false, "GET", "/containers/"+id+"/stats?stream=false", nil, &v)
		s.at = time.Now()
		if s.err == nil {
			p := sample{Time: s.at, Bytes: v.Memory.Usage}
			if v.Previous.System > 0 && v.CPU.System > v.Previous.System && v.CPU.Usage.Total >= v.Previous.Usage.Total && v.CPU.Count > 0 {
				pct := 100 * float64(v.CPU.Count) * float64(v.CPU.Usage.Total-v.Previous.Usage.Total) / float64(v.CPU.System-v.Previous.System)
				p.CPU = &pct
			}
			for len(s.history) > 0 && (len(s.history) >= 60 || s.at.Sub(s.history[0].Time) > 5*time.Minute) {
				s.history = s.history[1:]
			}
			s.history = append(s.history, p)
		}
	}
	return append([]sample(nil), s.history...), s.err
}
func (a *App) streamMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !containerID.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	s := a.samples.acquire(id)
	if s == nil {
		http.Error(w, "Messlimit erreicht", 429)
		return
	}
	defer a.samples.release(id, s)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		values, e := a.sample(id, s)
		event := "message"
		var payload any = values
		if e != nil {
			event = "unavailable"
			payload = e.Error()
		}
		b, _ := json.Marshal(payload)
		ctrl := http.NewResponseController(w)
		ctrl.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, e = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); e != nil {
			return
		}
		if ctrl.Flush() != nil {
			return
		}
		ctrl.SetWriteDeadline(time.Time{})
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
