// Package tools defines the agent's custom tools and executes them in Go.
//
// Design choice: the agent gets NO sandbox tools (no bash, no web). Every
// action goes through these typed, validated functions, so the backend - not
// the model - owns data access and business rules.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"replacement-copilot/internal/domain"
	"replacement-copilot/internal/rag"
)

const SystemPrompt = `You are Replacement Copilot, an assistant for the Car Replacement operations team of a car rental company. When a partner's customer (insurance or leasing) has their car in repair, you find and propose the right replacement vehicle.

How to work a case:
1. Call get_case to load the case.
2. Call search_policies for the partner's rules that matter for this case (category, duration, transmission/equipment, co-payment, delivery). Base decisions only on retrieved text.
3. Call search_vehicles in the case city for the start date. Start with the nearest branch's options but consider the whole city.
4. Call propose_replacement with your best option. The backend enforces hard business rules; if it returns violations, fix the proposal and try again (at most 2 retries), otherwise explain what blocks the case.
5. A human operations agent approves or rejects every proposal. Report the outcome.

Rules:
- Never invent vehicles, prices, policy rules or case data. Use only tool results.
- Never downgrade a customer. Never upgrade above the covered category at the partner's cost; that requires customer co-payment with explicit consent, which you must not assume - ask for it or present it as an option.
- If the repair estimate exceeds the covered days, propose the maximum covered days and flag an extension request.
- Cite policy text by chunk id in square brackets, for example [nordinsure-standard#rental-duration].

Final answer for the operations agent (business users, keep it short and scannable):
**Decision** - one line. **Vehicle** - model, id, branch, dates. **Why** - 2-4 bullets with citations. **Open points** - follow-ups such as extension requests or customer consent, or "None".`

func Definitions() []anthropic.BetaManagedAgentsCustomToolParams {
	cats := domain.CategoryNames()
	return []anthropic.BetaManagedAgentsCustomToolParams{
		{
			Name:        "get_case",
			Description: "Load a car replacement case: customer, partner and policy id, the customer's own car and category, transmission needs, city, nearest branch, start date, estimated repair days and customer notes.",
			InputSchema: anthropic.BetaManagedAgentsCustomToolInputSchemaParam{
				Properties: map[string]any{
					"case_id": map[string]any{"type": "string", "description": "Case id, e.g. RC-1001"},
				},
				Required: []string{"case_id"},
			},
		},
		{
			Name:        "search_policies",
			Description: "Search partner contracts and the operations playbook. Returns up to 4 short text chunks, each with an id you must cite. Ask focused questions, e.g. 'maximum rental days extension'.",
			InputSchema: anthropic.BetaManagedAgentsCustomToolInputSchemaParam{
				Properties: map[string]any{
					"query":   map[string]any{"type": "string", "description": "What you need to know"},
					"partner": map[string]any{"type": "string", "description": "Optional partner name to restrict results, e.g. NordInsure"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "search_vehicles",
			Description: "Find unreserved vehicles in a city that are available on a date. Optional filters: category range and automatic transmission. Returns id, branch, model, category, transmission, seats and available_from.",
			InputSchema: anthropic.BetaManagedAgentsCustomToolInputSchemaParam{
				Properties: map[string]any{
					"city":         map[string]any{"type": "string"},
					"available_on": map[string]any{"type": "string", "description": "ISO date YYYY-MM-DD"},
					"min_category": map[string]any{"type": "string", "enum": cats},
					"max_category": map[string]any{"type": "string", "enum": cats},
					"automatic":    map[string]any{"type": "boolean"},
				},
				Required: []string{"city", "available_on"},
			},
		},
		{
			Name:        "propose_replacement",
			Description: "Propose a replacement booking. The backend validates it against hard business rules and returns violations if any. If valid, the proposal goes to a human operations agent, and this call returns their decision (CONFIRMED with a reservation id, or REJECTED with a note).",
			InputSchema: anthropic.BetaManagedAgentsCustomToolInputSchemaParam{
				Properties: map[string]any{
					"case_id":        map[string]any{"type": "string"},
					"vehicle_id":     map[string]any{"type": "string"},
					"start_date":     map[string]any{"type": "string", "description": "ISO date YYYY-MM-DD"},
					"days":           map[string]any{"type": "integer", "minimum": 1},
					"customer_copay": map[string]any{"type": "boolean", "description": "true only if the customer explicitly agreed to pay the difference for a category above the policy cap"},
					"justification":  map[string]any{"type": "string", "description": "1-3 sentences for the human approver, citing policy chunk ids"},
				},
				Required: []string{"case_id", "vehicle_id", "start_date", "days", "customer_copay", "justification"},
			},
		},
	}
}

// ForCreate / ForUpdate wrap the definitions in the two union types the SDK
// uses for agents.New and agents.Update.
func ForCreate() []anthropic.BetaAgentNewParamsToolUnion {
	var out []anthropic.BetaAgentNewParamsToolUnion
	for _, d := range Definitions() {
		d := d
		out = append(out, anthropic.BetaAgentNewParamsToolUnion{OfCustom: &d})
	}
	return out
}

func ForUpdate() []anthropic.BetaAgentUpdateParamsToolUnion {
	var out []anthropic.BetaAgentUpdateParamsToolUnion
	for _, d := range Definitions() {
		d := d
		out = append(out, anthropic.BetaAgentUpdateParamsToolUnion{OfCustom: &d})
	}
	return out
}

type Deps struct {
	Store *domain.Store
	Index *rag.Index
}

// Result of one tool execution. Proposal is set when propose_replacement
// passed the guardrails and now needs a human decision.
type Result struct {
	Output     any
	IsError    bool
	Violations []string
	Proposal   *domain.ProposalInput
}

func (d *Deps) Execute(_ context.Context, name string, in map[string]any) Result {
	switch name {
	case "get_case":
		c, ok := d.Store.Case(str(in, "case_id"))
		if !ok {
			return errorf("case %q not found", str(in, "case_id"))
		}
		return Result{Output: c}

	case "search_policies":
		q := str(in, "query")
		if q == "" {
			return errorf("query is required")
		}
		hits := d.Index.Search(q, str(in, "partner"), 4)
		if len(hits) == 0 {
			return Result{Output: map[string]any{"results": []any{}, "note": "no matching policy text; rephrase the query"}}
		}
		return Result{Output: map[string]any{"results": hits}}

	case "search_vehicles":
		q := domain.VehicleQuery{City: str(in, "city"), AvailableOn: str(in, "available_on")}
		if q.City == "" || q.AvailableOn == "" {
			return errorf("city and available_on are required")
		}
		if s := str(in, "min_category"); s != "" {
			c, ok := domain.ParseCategory(s)
			if !ok {
				return errorf("unknown min_category %q", s)
			}
			q.MinCategory = &c
		}
		if s := str(in, "max_category"); s != "" {
			c, ok := domain.ParseCategory(s)
			if !ok {
				return errorf("unknown max_category %q", s)
			}
			q.MaxCategory = &c
		}
		if v, ok := in["automatic"].(bool); ok {
			q.Automatic = &v
		}
		vs := d.Store.SearchVehicles(q)
		return Result{Output: map[string]any{"count": len(vs), "vehicles": vs}}

	case "propose_replacement":
		p := domain.ProposalInput{
			CaseID:        strings.ToUpper(str(in, "case_id")),
			VehicleID:     strings.ToUpper(str(in, "vehicle_id")),
			StartDate:     str(in, "start_date"),
			Days:          num(in, "days"),
			CustomerCopay: in["customer_copay"] == true,
			Justification: str(in, "justification"),
		}
		if v := d.Store.CheckProposal(p); len(v) > 0 {
			return Result{
				IsError:    true,
				Violations: v,
				Output:     map[string]any{"status": "REJECTED_BY_GUARDRAILS", "violations": v},
			}
		}
		return Result{Proposal: &p}
	}
	return errorf("unknown tool %q", name)
}

func errorf(format string, a ...any) Result {
	return Result{IsError: true, Output: map[string]any{"error": fmt.Sprintf(format, a...)}}
}

func str(in map[string]any, k string) string {
	s, _ := in[k].(string)
	return strings.TrimSpace(s)
}

func num(in map[string]any, k string) int {
	switch v := in[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}
