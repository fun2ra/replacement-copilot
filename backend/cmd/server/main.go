// Command server is the data plane: an HTTP API + SSE stream for the React
// UI. Each "conversation" is one Managed Agents session.
//
//	go run ./cmd/server          (listens on :8080)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"replacement-copilot/internal/agentrun"
	"replacement-copilot/internal/bus"
	"replacement-copilot/internal/config"
	"replacement-copilot/internal/domain"
	"replacement-copilot/internal/rag"
	"replacement-copilot/internal/tools"
)

// hub fans out a session's trace to any number of browser tabs and keeps the
// backlog so a page refresh shows the full history.
type hub struct {
	mu      sync.Mutex
	history []agentrun.Trace
	subs    map[chan agentrun.Trace]struct{}
}

func newHub() *hub { return &hub{subs: map[chan agentrun.Trace]struct{}{}} }

func (h *hub) publish(t agentrun.Trace) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t.Seq = len(h.history) + 1
	if t.At.IsZero() {
		t.At = time.Now()
	}
	h.history = append(h.history, t)
	for ch := range h.subs {
		select {
		case ch <- t:
		default: // slow client; it can refresh to get the backlog
		}
	}
}

func (h *hub) subscribe() ([]agentrun.Trace, chan agentrun.Trace, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan agentrun.Trace, 256)
	h.subs[ch] = struct{}{}
	backlog := append([]agentrun.Trace(nil), h.history...)
	return backlog, ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

type conv struct {
	sess   *agentrun.Session
	hub    *hub
	caseID string
}

type server struct {
	ctx      context.Context
	client   anthropic.Client
	cfg      config.AgentConfig
	deps     *tools.Deps
	bus      *bus.Bus
	approver *agentrun.UIApprover

	mu    sync.Mutex
	convs map[string]*conv
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	idx, err := rag.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	b := bus.New()
	// A second "service" consuming the domain event, e.g. billing or the
	// partner notification service in a real event-driven setup.
	b.Subscribe(func(e bus.Event) {
		raw, _ := json.Marshal(e.Payload)
		log.Printf("[bus] %s consumed by partner-notification-svc: %s", e.Type, raw)
	})

	s := &server{
		ctx: ctx, client: anthropic.NewClient(), cfg: cfg,
		deps: &tools.Deps{Store: domain.NewStore(), Index: idx},
		bus:  b, approver: agentrun.NewUIApprover(), convs: map[string]*conv{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/cases", s.listCases)
	mux.HandleFunc("GET /api/reservations", s.listReservations)
	mux.HandleFunc("POST /api/sessions", s.createSession)
	mux.HandleFunc("POST /api/sessions/{id}/messages", s.sendMessage)
	mux.HandleFunc("GET /api/sessions/{id}/events", s.streamEvents)
	mux.HandleFunc("POST /api/approvals/{id}", s.resolveApproval)

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() { <-ctx.Done(); srv.Close() }()
	log.Printf("agent %s v%d, environment %s", cfg.AgentID, cfg.AgentVersion, cfg.EnvironmentID)
	log.Println("listening on http://localhost:8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func (s *server) listCases(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Store.Cases())
}

func (s *server) listReservations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Store.Reservations())
}

func (s *server) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CaseID string `json:"case_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CaseID == "" {
		http.Error(w, "case_id required", http.StatusBadRequest)
		return
	}
	h := newHub()
	// s.ctx, not r.Context(): the session must outlive this request.
	sess, err := agentrun.Start(s.ctx, s.client, s.cfg.AgentID, s.cfg.EnvironmentID,
		"Replacement case "+body.CaseID, s.deps, s.bus, s.approver, h.publish)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.mu.Lock()
	s.convs[sess.ID] = &conv{sess: sess, hub: h, caseID: body.CaseID}
	s.mu.Unlock()

	kickoff := fmt.Sprintf("New replacement request for case %s. Find the right replacement vehicle and propose it.", body.CaseID)
	if err := sess.Send(s.ctx, kickoff); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id":  sess.ID,
		"console_url": "https://platform.claude.com/workspaces/default/sessions/" + sess.ID,
	})
}

func (s *server) sendMessage(w http.ResponseWriter, r *http.Request) {
	c := s.conv(r.PathValue("id"))
	if c == nil {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	if err := c.sess.Send(s.ctx, body.Text); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// streamEvents is plain SSE to the browser (EventSource).
func (s *server) streamEvents(w http.ResponseWriter, r *http.Request) {
	c := s.conv(r.PathValue("id"))
	if c == nil {
		http.NotFound(w, r)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	backlog, ch, cancel := c.hub.subscribe()
	defer cancel()
	write := func(t agentrun.Trace) {
		raw, _ := json.Marshal(t)
		fmt.Fprintf(w, "data: %s\n\n", raw)
	}
	for _, t := range backlog {
		write(t)
	}
	fl.Flush()
	for {
		select {
		case t := <-ch:
			write(t)
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *server) resolveApproval(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Approved bool   `json:"approved"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if err := s.approver.Resolve(r.PathValue("id"), body.Approved, "ops-agent (UI)", body.Note); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) conv(id string) *conv {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.convs[id]
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
