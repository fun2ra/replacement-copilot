// Package domain holds the (mock) Car Replacement domain: cases, partner
// policies, fleet and reservations. In production each of these would be its
// own microservice; here they are an in-memory store so the demo runs offline.
package domain

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Category is an ordered vehicle class, so "no downgrade" and "max covered
// category" become simple integer comparisons in the guardrails.
type Category int

const (
	Mini Category = iota
	Economy
	Compact
	Intermediate
	Standard
	SUV
	Premium
)

var categoryNames = []string{"MINI", "ECONOMY", "COMPACT", "INTERMEDIATE", "STANDARD", "SUV", "PREMIUM"}

func (c Category) String() string { return categoryNames[c] }

func (c Category) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

func ParseCategory(s string) (Category, bool) {
	for i, n := range categoryNames {
		if strings.EqualFold(n, strings.TrimSpace(s)) {
			return Category(i), true
		}
	}
	return 0, false
}

func CategoryNames() []string { return append([]string(nil), categoryNames...) }

// Policy is the structured version of a partner contract. The prose version
// lives in internal/rag/policies and is what the agent reads (RAG); this
// struct is what the guardrails enforce. Keep them in sync.
type Policy struct {
	ID              string   `json:"id"`
	Partner         string   `json:"partner"`
	MaxCategory     Category `json:"max_category"`
	MaxDays         int      `json:"max_days"`
	DeliveryAllowed bool     `json:"delivery_allowed"`
}

type Case struct {
	ID             string   `json:"id"`
	Customer       string   `json:"customer"`
	Partner        string   `json:"partner"`
	PolicyID       string   `json:"policy_id"`
	OwnCar         string   `json:"own_car"`
	OwnCategory    Category `json:"own_category"`
	NeedsAutomatic bool     `json:"needs_automatic"`
	City           string   `json:"city"`
	NearestBranch  string   `json:"nearest_branch"`
	StartDate      string   `json:"start_date"`
	EstRepairDays  int      `json:"estimated_repair_days"`
	CustomerNote   string   `json:"customer_note,omitempty"`
}

type Vehicle struct {
	ID            string   `json:"id"`
	Branch        string   `json:"branch"`
	City          string   `json:"city"`
	Model         string   `json:"model"`
	Category      Category `json:"category"`
	Automatic     bool     `json:"automatic"`
	Seats         int      `json:"seats"`
	AvailableFrom string   `json:"available_from"` // ISO date, string compare is safe
	Reserved      bool     `json:"-"`
}

type Reservation struct {
	ID            string    `json:"id"`
	CaseID        string    `json:"case_id"`
	VehicleID     string    `json:"vehicle_id"`
	StartDate     string    `json:"start_date"`
	Days          int       `json:"days"`
	CustomerCopay bool      `json:"customer_copay"`
	Justification string    `json:"justification"`
	ApprovedBy    string    `json:"approved_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// ProposalInput is what the agent sends to propose_replacement.
type ProposalInput struct {
	CaseID        string `json:"case_id"`
	VehicleID     string `json:"vehicle_id"`
	StartDate     string `json:"start_date"`
	Days          int    `json:"days"`
	CustomerCopay bool   `json:"customer_copay"`
	Justification string `json:"justification"`
}

type Store struct {
	mu           sync.Mutex
	cases        map[string]Case
	policies     map[string]Policy
	vehicles     []*Vehicle
	reservations []Reservation
}

func NewStore() *Store {
	s := &Store{cases: map[string]Case{}, policies: map[string]Policy{}}
	for _, p := range seedPolicies {
		s.policies[p.ID] = p
	}
	for _, c := range seedCases {
		s.cases[c.ID] = c
	}
	for i := range seedVehicles {
		v := seedVehicles[i]
		s.vehicles = append(s.vehicles, &v)
	}
	return s
}

func (s *Store) Cases() []Case {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Case, 0, len(s.cases))
	for _, id := range []string{"RC-1001", "RC-1002", "RC-1003"} {
		if c, ok := s.cases[id]; ok {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) Case(id string) (Case, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[strings.ToUpper(strings.TrimSpace(id))]
	return c, ok
}

func (s *Store) Policy(id string) (Policy, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.policies[id]
	return p, ok
}

type VehicleQuery struct {
	City        string
	MinCategory *Category
	MaxCategory *Category
	Automatic   *bool
	AvailableOn string
}

func (s *Store) SearchVehicles(q VehicleQuery) []Vehicle {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Vehicle
	for _, v := range s.vehicles {
		switch {
		case v.Reserved:
		case q.City != "" && !strings.EqualFold(v.City, q.City):
		case q.MinCategory != nil && v.Category < *q.MinCategory:
		case q.MaxCategory != nil && v.Category > *q.MaxCategory:
		case q.Automatic != nil && v.Automatic != *q.Automatic:
		case q.AvailableOn != "" && v.AvailableFrom > q.AvailableOn:
		default:
			out = append(out, *v)
		}
	}
	return out
}

// CheckProposal is the deterministic guardrail. The LLM proposes, Go decides
// whether the proposal is even allowed. Returns human-readable violations.
func (s *Store) CheckProposal(in ProposalInput) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var v []string
	c, ok := s.cases[in.CaseID]
	if !ok {
		return []string{fmt.Sprintf("case %q does not exist", in.CaseID)}
	}
	p := s.policies[c.PolicyID]
	var car *Vehicle
	for _, x := range s.vehicles {
		if x.ID == in.VehicleID {
			car = x
		}
	}
	if car == nil {
		return []string{fmt.Sprintf("vehicle %q does not exist", in.VehicleID)}
	}
	if car.Reserved {
		v = append(v, "vehicle is already reserved")
	}
	if !strings.EqualFold(car.City, c.City) {
		v = append(v, fmt.Sprintf("vehicle is in %s but the customer is in %s (one-way/cross-city not covered)", car.City, c.City))
	}
	if in.StartDate < c.StartDate {
		v = append(v, fmt.Sprintf("start date %s is before the case start date %s", in.StartDate, c.StartDate))
	}
	if car.AvailableFrom > in.StartDate {
		v = append(v, fmt.Sprintf("vehicle is only available from %s", car.AvailableFrom))
	}
	if car.Category < c.OwnCategory {
		v = append(v, fmt.Sprintf("downgrade not allowed: customer's own car is %s, vehicle is %s", c.OwnCategory, car.Category))
	}
	if car.Category > p.MaxCategory && !in.CustomerCopay {
		v = append(v, fmt.Sprintf("%s exceeds the %s policy cap (%s); requires customer_copay=true with customer consent", car.Category, p.ID, p.MaxCategory))
	}
	if in.Days < 1 {
		v = append(v, "days must be >= 1")
	}
	if in.Days > p.MaxDays {
		v = append(v, fmt.Sprintf("%d days exceeds the %s maximum of %d days; propose the maximum and flag an extension request", in.Days, p.ID, p.MaxDays))
	}
	if c.NeedsAutomatic && !car.Automatic {
		v = append(v, "customer requires an automatic transmission")
	}
	if strings.TrimSpace(in.Justification) == "" {
		v = append(v, "justification is required (it is shown to the approver)")
	}
	return v
}

func (s *Store) Confirm(in ProposalInput, approvedBy string) (Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.vehicles {
		if x.ID == in.VehicleID {
			if x.Reserved {
				return Reservation{}, fmt.Errorf("vehicle %s already reserved", x.ID)
			}
			x.Reserved = true
			r := Reservation{
				ID:            fmt.Sprintf("RES-%04d", 5001+len(s.reservations)),
				CaseID:        in.CaseID,
				VehicleID:     in.VehicleID,
				StartDate:     in.StartDate,
				Days:          in.Days,
				CustomerCopay: in.CustomerCopay,
				Justification: in.Justification,
				ApprovedBy:    approvedBy,
				CreatedAt:     time.Now(),
			}
			s.reservations = append(s.reservations, r)
			return r, nil
		}
	}
	return Reservation{}, fmt.Errorf("vehicle %s not found", in.VehicleID)
}

func (s *Store) Reservations() []Reservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Reservation(nil), s.reservations...)
}

func (s *Store) Vehicle(id string) (Vehicle, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.vehicles {
		if v.ID == id {
			return *v, true
		}
	}
	return Vehicle{}, false
}
