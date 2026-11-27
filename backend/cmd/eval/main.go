// Command eval runs every demo case end-to-end in a fresh session (with an
// auto-approver instead of a human) and checks the agent's behaviour with
// deterministic assertions over the trace and the resulting reservation.
//
//	go run ./cmd/eval            all cases
//	go run ./cmd/eval RC-1003    one case
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
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

type run struct {
	traces []agentrun.Trace
	store  *domain.Store
	took   time.Duration
}

func (r run) tools() []string {
	var out []string
	for _, t := range r.traces {
		if t.Kind == "tool_call" {
			out = append(out, t.Tool)
		}
	}
	return out
}

func (r run) finalText() string {
	last := ""
	for _, t := range r.traces {
		if t.Kind == "agent_message" {
			last = t.Text
		}
	}
	return last
}

func (r run) reservation() (domain.Reservation, domain.Vehicle, bool) {
	rs := r.store.Reservations()
	if len(rs) == 0 {
		return domain.Reservation{}, domain.Vehicle{}, false
	}
	v, _ := r.store.Vehicle(rs[0].VehicleID)
	return rs[0], v, true
}

type check struct {
	name string
	fn   func(run) bool
}

var citation = regexp.MustCompile(`\[[a-z0-9-]+#[a-z0-9-]+\]`)

var common = []check{
	{"loads the case first", func(r run) bool { t := r.tools(); return len(t) > 0 && t[0] == "get_case" }},
	{"reads policy before proposing", func(r run) bool {
		for _, t := range r.tools() {
			if t == "search_policies" {
				return true
			}
			if t == "propose_replacement" {
				return false
			}
		}
		return false
	}},
	{"ends with a confirmed reservation", func(r run) bool { _, _, ok := r.reservation(); return ok }},
	{"cites policy chunks in final answer", func(r run) bool { return citation.MatchString(r.finalText()) }},
}

var specific = map[string][]check{
	"RC-1001": {{"like-for-like COMPACT (no up/downgrade)", func(r run) bool {
		_, v, ok := r.reservation()
		return ok && v.Category == domain.Compact
	}}},
	"RC-1002": {{"automatic vehicle", func(r run) bool {
		_, v, ok := r.reservation()
		return ok && v.Automatic
	}}},
	"RC-1003": {
		{"caps duration at 10 days", func(r run) bool { res, _, ok := r.reservation(); return ok && res.Days <= 10 }},
		{"does not assume co-pay consent", func(r run) bool { res, _, ok := r.reservation(); return ok && !res.CustomerCopay }},
		{"flags the extension request", func(r run) bool { return strings.Contains(strings.ToLower(r.finalText()), "extension") }},
	},
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
	client := anthropic.NewClient()
	cases := []string{"RC-1001", "RC-1002", "RC-1003"}
	if len(os.Args) > 1 {
		cases = os.Args[1:]
	}

	passed, total := 0, 0
	for _, id := range cases {
		r, err := execute(client, cfg, idx, id)
		if err != nil {
			log.Printf("%s: %v", id, err)
			continue
		}
		rejections := 0
		for _, t := range r.traces {
			if t.Kind == "guardrail" && t.IsError {
				rejections++
			}
		}
		fmt.Printf("\n== %s  (%d tool calls, %d guardrail rejections, %s)\n", id, len(r.tools()), rejections, r.took.Round(time.Second))
		fmt.Printf("   tools: %s\n", strings.Join(r.tools(), " -> "))
		for _, c := range append(append([]check{}, common...), specific[id]...) {
			ok := c.fn(r)
			total++
			mark := "FAIL"
			if ok {
				passed++
				mark = "PASS"
			}
			fmt.Printf("   [%s] %s\n", mark, c.name)
		}
	}
	fmt.Printf("\n%d/%d checks passed\n", passed, total)
}

func execute(client anthropic.Client, cfg config.AgentConfig, idx *rag.Index, caseID string) (run, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	r := run{store: domain.NewStore()} // fresh fleet per scenario
	var mu sync.Mutex
	done := make(chan struct{})
	var once sync.Once
	emit := func(t agentrun.Trace) {
		mu.Lock()
		r.traces = append(r.traces, t)
		mu.Unlock()
		if t.Kind == "turn_end" || (t.Kind == "status" && t.Text == "terminated") {
			once.Do(func() { close(done) })
		}
	}

	start := time.Now()
	sess, err := agentrun.Start(ctx, client, cfg.AgentID, cfg.EnvironmentID, "eval "+caseID,
		&tools.Deps{Store: r.store, Index: idx}, bus.New(), agentrun.AutoApprover{}, emit)
	if err != nil {
		return r, err
	}
	if err := sess.Send(ctx, fmt.Sprintf("New replacement request for case %s. Find the right replacement vehicle and propose it.", caseID)); err != nil {
		return r, err
	}
	select {
	case <-done:
	case <-ctx.Done():
		return r, fmt.Errorf("timed out")
	}
	r.took = time.Since(start)
	mu.Lock()
	defer mu.Unlock()
	r.traces = append([]agentrun.Trace(nil), r.traces...)
	return r, nil
}
