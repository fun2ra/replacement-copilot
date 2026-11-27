package domain

// All data below is fictional demo data. Partners, customers and vehicles are
// invented; the scenarios are designed to exercise the guardrails:
//   RC-1001  nearest branch has no like-for-like car -> agent must look at the
//            other Lisbon branch instead of downgrading or upgrading.
//   RC-1002  automatic required + family -> manual cars are rejected.
//   RC-1003  customer wants an SUV for 12 days on a policy capped at COMPACT
//            and 10 days -> co-pay + extension request, never a silent upgrade.

var seedPolicies = []Policy{
	{ID: "NI-STD", Partner: "NordInsure", MaxCategory: Compact, MaxDays: 10, DeliveryAllowed: false},
	{ID: "FL-PREM", Partner: "FleetLease", MaxCategory: SUV, MaxDays: 21, DeliveryAllowed: true},
}

var seedCases = []Case{
	{
		ID: "RC-1001", Customer: "Ana Ferreira", Partner: "NordInsure", PolicyID: "NI-STD",
		OwnCar: "VW Golf", OwnCategory: Compact, NeedsAutomatic: false,
		City: "Lisbon", NearestBranch: "LIS-APT", StartDate: "2026-12-01", EstRepairDays: 6,
		CustomerNote: "Car was hit while parked. Needs a car to commute to work.",
	},
	{
		ID: "RC-1002", Customer: "Jonas Weber", Partner: "FleetLease", PolicyID: "FL-PREM",
		OwnCar: "Skoda Superb Combi", OwnCategory: Standard, NeedsAutomatic: true,
		City: "Munich", NearestBranch: "MUC-HBF", StartDate: "2026-12-02", EstRepairDays: 14,
		CustomerNote: "Family of four, needs room for two child seats. Only drives automatic.",
	},
	{
		ID: "RC-1003", Customer: "Rui Costa", Partner: "NordInsure", PolicyID: "NI-STD",
		OwnCar: "Fiat 500", OwnCategory: Mini, NeedsAutomatic: false,
		City: "Lisbon", NearestBranch: "LIS-CTR", StartDate: "2026-12-01", EstRepairDays: 12,
		CustomerNote: "Customer asks for an SUV because of a family trip to the Algarve.",
	},
}

var seedVehicles = []Vehicle{
	// Lisbon
	{ID: "LIS-APT-01", Branch: "LIS-APT", City: "Lisbon", Model: "Renault Clio", Category: Economy, Automatic: false, Seats: 5, AvailableFrom: "2026-11-28"},
	{ID: "LIS-APT-02", Branch: "LIS-APT", City: "Lisbon", Model: "VW Golf", Category: Compact, Automatic: false, Seats: 5, AvailableFrom: "2026-12-04"},
	{ID: "LIS-APT-03", Branch: "LIS-APT", City: "Lisbon", Model: "Peugeot 308", Category: Intermediate, Automatic: true, Seats: 5, AvailableFrom: "2026-11-29"},
	{ID: "LIS-CTR-01", Branch: "LIS-CTR", City: "Lisbon", Model: "Seat Leon", Category: Compact, Automatic: false, Seats: 5, AvailableFrom: "2026-11-30"},
	{ID: "LIS-CTR-02", Branch: "LIS-CTR", City: "Lisbon", Model: "Fiat 500", Category: Mini, Automatic: false, Seats: 4, AvailableFrom: "2026-11-27"},
	{ID: "LIS-CTR-03", Branch: "LIS-CTR", City: "Lisbon", Model: "Peugeot 3008", Category: SUV, Automatic: true, Seats: 5, AvailableFrom: "2026-11-30"},
	// Munich
	{ID: "MUC-HBF-01", Branch: "MUC-HBF", City: "Munich", Model: "VW Passat Variant", Category: Standard, Automatic: false, Seats: 5, AvailableFrom: "2026-11-30"},
	{ID: "MUC-HBF-02", Branch: "MUC-HBF", City: "Munich", Model: "BMW X1", Category: SUV, Automatic: true, Seats: 5, AvailableFrom: "2026-12-01"},
	{ID: "MUC-APT-01", Branch: "MUC-APT", City: "Munich", Model: "Mercedes E-Class", Category: Premium, Automatic: true, Seats: 5, AvailableFrom: "2026-11-30"},
	{ID: "MUC-APT-02", Branch: "MUC-APT", City: "Munich", Model: "Skoda Octavia Combi", Category: Standard, Automatic: true, Seats: 5, AvailableFrom: "2026-12-05"},
	{ID: "MUC-APT-03", Branch: "MUC-APT", City: "Munich", Model: "VW Touran", Category: Standard, Automatic: true, Seats: 7, AvailableFrom: "2026-12-02"},
}
