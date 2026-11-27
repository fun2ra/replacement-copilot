// Package agentrun orchestrates one Managed Agents session: it streams the
// session's events, executes custom tool calls in Go (with guardrails and a
// human approval step), sends the results back, and turns everything into a
// normalized Trace stream that the UI (or the eval harness) consumes.
package agentrun

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"replacement-copilot/internal/bus"
	"replacement-copilot/internal/domain"
	"replacement-copilot/internal/tools"
)

// Trace is the UI-facing event. One schema for everything the agent does, so
// the frontend can render a transparent timeline for business users.
type Trace struct {
	Seq        int       `json:"seq"`
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"` // user | agent_message | status | tool_call | tool_result | guardrail | approval_required | approval_resolved | domain_event | turn_end | error
	Text       string    `json:"text,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	ToolUseID  string    `json:"tool_use_id,omitempty"`
	Input      any       `json:"input,omitempty"`
	Output     any       `json:"output,omitempty"`
	IsError    bool      `json:"is_error,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	ApprovalID string    `json:"approval_id,omitempty"`
}

// Approver decides on proposals that passed the guardrails.
type Approver interface {
	Approve(ctx context.Context, sessionID, approvalID string, p domain.ProposalInput) (approved bool, by, note string)
}

type Session struct {
	ID       string
	client   anthropic.Client
	deps     *tools.Deps
	bus      *bus.Bus
	approver Approver
	emit     func(Trace)

	mu      sync.Mutex
	seen    map[string]bool // event ids already processed (dedupe across reconnects)
	handled map[string]bool // custom tool use ids already dispatched
}

// Start creates a session for an existing agent + environment and begins
// pumping its event stream in the background. ctx must outlive the HTTP
// request that triggered it.
func Start(ctx context.Context, client anthropic.Client, agentID, envID, title string,
	deps *tools.Deps, b *bus.Bus, approver Approver, emit func(Trace)) (*Session, error) {

	sess, err := client.Beta.Sessions.New(ctx, anthropic.BetaSessionNewParams{
		Agent:         anthropic.BetaSessionNewParamsAgentUnion{OfString: anthropic.String(agentID)},
		EnvironmentID: envID,
		Title:         anthropic.String(title),
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	s := &Session{
		ID: sess.ID, client: client, deps: deps, bus: b, approver: approver, emit: emit,
		seen: map[string]bool{}, handled: map[string]bool{},
	}
	ready := make(chan struct{})
	go s.pump(ctx, ready)
	<-ready // stream is open before the caller sends the first message
	return s, nil
}

func (s *Session) Send(ctx context.Context, text string) error {
	s.emit(Trace{Kind: "user", Text: text})
	_, err := s.client.Beta.Sessions.Events.Send(ctx, s.ID, anthropic.BetaSessionEventSendParams{
		Events: []anthropic.BetaManagedAgentsEventParamsUnion{{
			OfUserMessage: &anthropic.BetaManagedAgentsUserMessageEventParams{
				Content: []anthropic.BetaManagedAgentsUserMessageEventParamsContentUnion{{
					OfText: &anthropic.BetaManagedAgentsTextBlockParam{Text: text},
				}},
			},
		}},
	})
	return err
}

// pump keeps one SSE connection open and reconnects if it drops. On every
// (re)connect it opens the stream first, then lists history to catch up on
// anything missed - the SSE stream has no replay.
func (s *Session) pump(ctx context.Context, ready chan struct{}) {
	once := sync.Once{}
	signal := func() { once.Do(func() { close(ready) }) }
	defer signal()
	for attempt := 0; ctx.Err() == nil; attempt++ {
		if attempt > 0 {
			s.emit(Trace{Kind: "status", Text: "reconnecting"})
			time.Sleep(time.Duration(min(attempt, 5)) * time.Second)
		}
		if done := s.streamOnce(ctx, signal); done {
			return
		}
	}
}

func (s *Session) streamOnce(ctx context.Context, opened func()) (terminated bool) {
	stream := s.client.Beta.Sessions.Events.StreamEvents(ctx, s.ID, anthropic.BetaSessionEventStreamParams{})
	defer stream.Close()
	opened()

	// Catch up from history: collect first so we know which tool calls
	// already have results, then dispatch only the unresolved ones.
	var history []anthropic.BetaManagedAgentsSessionEventUnion
	it := s.client.Beta.Sessions.Events.ListAutoPaging(ctx, s.ID, anthropic.BetaSessionEventListParams{})
	for it.Next() {
		history = append(history, it.Current())
	}
	if err := it.Err(); err != nil {
		log.Printf("session %s: list history: %v", s.ID, err)
	}
	for _, ev := range history {
		if ev.Type == "user.custom_tool_result" {
			s.markHandled(ev.AsUserCustomToolResult().CustomToolUseID)
		}
	}
	for _, ev := range history {
		if !s.markSeen(ev.ID) {
			continue
		}
		switch ev.Type {
		case "agent.message":
			s.emitAgentText(ev.AsAgentMessage().Content)
		case "agent.custom_tool_use":
			tu := ev.AsAgentCustomToolUse()
			s.dispatch(ctx, tu.ID, tu.Name, tu.Input)
		}
	}

	for stream.Next() {
		cur := stream.Current()
		if !s.markSeen(cur.ID) {
			continue
		}
		switch ev := cur.AsAny().(type) {
		case anthropic.BetaManagedAgentsAgentMessageEvent:
			s.emitAgentText(ev.Content)
		case anthropic.BetaManagedAgentsAgentCustomToolUseEvent:
			s.dispatch(ctx, ev.ID, ev.Name, ev.Input)
		case anthropic.BetaManagedAgentsSessionStatusRunningEvent:
			s.emit(Trace{Kind: "status", Text: "running"})
		case anthropic.BetaManagedAgentsSessionStatusIdleEvent:
			// requires_action = waiting on our tool results; not the end of a turn.
			if ev.StopReason.Type == "requires_action" {
				s.emit(Trace{Kind: "status", Text: "waiting for tool results"})
			} else {
				s.emit(Trace{Kind: "turn_end", Text: ev.StopReason.Type})
			}
		case anthropic.BetaManagedAgentsSessionErrorEvent:
			s.emit(Trace{Kind: "error", Text: ev.Error.Message})
		case anthropic.BetaManagedAgentsSessionStatusTerminatedEvent:
			s.emit(Trace{Kind: "status", Text: "terminated"})
			return true
		}
	}
	if err := stream.Err(); err != nil && ctx.Err() == nil {
		log.Printf("session %s: stream: %v", s.ID, err)
	}
	return false
}

func (s *Session) emitAgentText(blocks []anthropic.BetaManagedAgentsAgentMessageEventContentUnion) {
	text := ""
	for _, b := range blocks {
		if b.Type == "text" {
			text += b.Text
		}
	}
	if text != "" {
		s.emit(Trace{Kind: "agent_message", Text: text})
	}
}

// dispatch runs a custom tool in its own goroutine so the stream keeps being
// read (parallel tool calls, and a slow human approval, must not block it).
func (s *Session) dispatch(ctx context.Context, id, name string, input map[string]any) {
	if !s.markHandled(id) {
		return
	}
	go func() {
		start := time.Now()
		s.emit(Trace{Kind: "tool_call", ToolUseID: id, Tool: name, Input: input})

		res := s.deps.Execute(ctx, name, input)
		if name == "propose_replacement" {
			s.emit(Trace{Kind: "guardrail", ToolUseID: id, Tool: name, IsError: len(res.Violations) > 0, Output: map[string]any{"passed": len(res.Violations) == 0, "violations": res.Violations}})
		}
		if res.Proposal != nil {
			res = s.humanInTheLoop(ctx, id, *res.Proposal)
		}

		body, _ := json.Marshal(res.Output)
		_, err := s.client.Beta.Sessions.Events.Send(ctx, s.ID, anthropic.BetaSessionEventSendParams{
			Events: []anthropic.BetaManagedAgentsEventParamsUnion{{
				OfUserCustomToolResult: &anthropic.BetaManagedAgentsUserCustomToolResultEventParams{
					CustomToolUseID: id,
					IsError:         anthropic.Bool(res.IsError),
					Content: []anthropic.BetaManagedAgentsUserCustomToolResultEventParamsContentUnion{{
						OfText: &anthropic.BetaManagedAgentsTextBlockParam{Text: string(body)},
					}},
				},
			}},
		})
		if err != nil {
			s.emit(Trace{Kind: "error", Text: "sending tool result: " + err.Error()})
		}
		s.emit(Trace{Kind: "tool_result", ToolUseID: id, Tool: name, Output: res.Output, IsError: res.IsError, DurationMs: time.Since(start).Milliseconds()})
	}()
}

func (s *Session) humanInTheLoop(ctx context.Context, toolUseID string, p domain.ProposalInput) tools.Result {
	approvalID := "APR-" + toolUseID[max(0, len(toolUseID)-8):]
	veh, _ := s.deps.Store.Vehicle(p.VehicleID)
	s.emit(Trace{Kind: "approval_required", ToolUseID: toolUseID, ApprovalID: approvalID, Output: map[string]any{"proposal": p, "vehicle": veh}})

	ok, by, note := s.approver.Approve(ctx, s.ID, approvalID, p)
	s.emit(Trace{Kind: "approval_resolved", ApprovalID: approvalID, Output: map[string]any{"approved": ok, "by": by, "note": note}})
	if !ok {
		return tools.Result{Output: map[string]any{"status": "REJECTED_BY_HUMAN", "by": by, "note": note}}
	}
	r, err := s.deps.Store.Confirm(p, by)
	if err != nil {
		return tools.Result{IsError: true, Output: map[string]any{"status": "FAILED", "error": err.Error()}}
	}
	ev := s.bus.Publish("ReplacementReserved", r)
	s.emit(Trace{Kind: "domain_event", Text: ev.Type, Output: r})
	return tools.Result{Output: map[string]any{"status": "CONFIRMED", "reservation": r, "note": note}}
}

func (s *Session) markSeen(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" || s.seen[id] {
		return id == "" // events without id are always processed
	}
	s.seen[id] = true
	return true
}

func (s *Session) markHandled(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handled[id] {
		return false
	}
	s.handled[id] = true
	return true
}
