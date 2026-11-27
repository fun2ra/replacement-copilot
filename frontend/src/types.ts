// Mirrors the Go structs in backend/internal/{agentrun,domain}.

export type TraceKind =
  | "user"
  | "agent_message"
  | "status"
  | "tool_call"
  | "tool_result"
  | "guardrail"
  | "approval_required"
  | "approval_resolved"
  | "domain_event"
  | "turn_end"
  | "error";

export interface Trace {
  seq: number;
  at: string;
  kind: TraceKind;
  text?: string;
  tool?: string;
  tool_use_id?: string;
  input?: Record<string, unknown>;
  output?: unknown;
  is_error?: boolean;
  duration_ms?: number;
  approval_id?: string;
}

export interface Case {
  id: string;
  customer: string;
  partner: string;
  policy_id: string;
  own_car: string;
  own_category: string;
  needs_automatic: boolean;
  city: string;
  nearest_branch: string;
  start_date: string;
  estimated_repair_days: number;
  customer_note?: string;
}

export interface Vehicle {
  id: string;
  branch: string;
  model: string;
  category: string;
  automatic: boolean;
  seats: number;
  available_from: string;
}

export interface Proposal {
  case_id: string;
  vehicle_id: string;
  start_date: string;
  days: number;
  customer_copay: boolean;
  justification: string;
}

export interface Reservation extends Proposal {
  id: string;
  approved_by: string;
  created_at: string;
}

export interface PolicyHit {
  id: string;
  doc_title: string;
  section: string;
  text: string;
  score: number;
}
