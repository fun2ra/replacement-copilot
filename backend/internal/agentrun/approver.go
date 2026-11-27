package agentrun

import (
	"context"
	"fmt"
	"sync"
	"time"

	"replacement-copilot/internal/domain"
)

type decision struct {
	approved bool
	by, note string
}

// UIApprover parks a proposal until a human clicks Approve/Reject in the UI.
// The tool call stays open meanwhile; the session idles with requires_action.
type UIApprover struct {
	mu      sync.Mutex
	pending map[string]chan decision
	Timeout time.Duration
}

func NewUIApprover() *UIApprover {
	return &UIApprover{pending: map[string]chan decision{}, Timeout: 15 * time.Minute}
}

func (a *UIApprover) Approve(ctx context.Context, _, approvalID string, _ domain.ProposalInput) (bool, string, string) {
	ch := make(chan decision, 1)
	a.mu.Lock()
	a.pending[approvalID] = ch
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.pending, approvalID)
		a.mu.Unlock()
	}()
	select {
	case d := <-ch:
		return d.approved, d.by, d.note
	case <-time.After(a.Timeout):
		return false, "system", "no human decision within the timeout"
	case <-ctx.Done():
		return false, "system", "server shutting down"
	}
}

func (a *UIApprover) Resolve(approvalID string, approved bool, by, note string) error {
	a.mu.Lock()
	ch, ok := a.pending[approvalID]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("no pending approval %s", approvalID)
	}
	ch <- decision{approved, by, note}
	return nil
}

// AutoApprover is used by the eval harness (no human in the loop).
type AutoApprover struct{}

func (AutoApprover) Approve(context.Context, string, string, domain.ProposalInput) (bool, string, string) {
	return true, "eval-harness", "auto-approved for evaluation"
}
