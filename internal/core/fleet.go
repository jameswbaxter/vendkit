// Fleet audit: read-only aggregation over consumer conformance documents
// (conformance spec §5). A scheduled external job clones each consumer and runs
// `conformance --json`; this aggregation folds the resulting interchange
// documents into one fleet report. Pure data reduction — no clone, no fetch, no
// network or SCM call (the audit never inverts the trust model).

package core

import "sort"

// FleetRow is one consumer's line in the aggregated fleet report: the
// per-consumer projection the spec names (slice, profile, pin, pin_lag,
// gap_count, worst_status).
type FleetRow struct {
	Slice       string  `json:"slice"`
	Profile     string  `json:"profile"`
	Pin         *PinDoc `json:"pin,omitempty"`
	PinLag      *int    `json:"pin_lag"`
	GapCount    int     `json:"gap_count"`
	WorstStatus string  `json:"worst_status"`
}

// FleetReport is the fleet-level interchange document: fleet size, a census of
// consumers by worst status, total gaps across the fleet, and the per-consumer
// rows sorted worst offenders first.
type FleetReport struct {
	TotalConsumers int            `json:"total_consumers"`
	ByWorstStatus  map[string]int `json:"by_worst_status"`
	TotalGaps      int            `json:"total_gaps"`
	Consumers      []*FleetRow    `json:"consumers"`
}

// AggregateFleet folds conformance documents into one fleet report. Rows are
// sorted worst first: by status rank desc, then gap count desc, then slice name
// asc for a stable order.
func AggregateFleet(docs []*ConformanceDoc) *FleetReport {
	report := &FleetReport{
		ByWorstStatus: map[string]int{},
		Consumers:     []*FleetRow{},
	}
	for _, d := range docs {
		worst := d.WorstStatus()
		report.TotalConsumers++
		report.TotalGaps += d.GapCount
		report.ByWorstStatus[worst]++
		report.Consumers = append(report.Consumers, &FleetRow{
			Slice:       d.Slice,
			Profile:     d.Profile,
			Pin:         d.Pin,
			PinLag:      d.PinLag,
			GapCount:    d.GapCount,
			WorstStatus: worst,
		})
	}
	sort.SliceStable(report.Consumers, func(i, j int) bool {
		a, b := report.Consumers[i], report.Consumers[j]
		if ra, rb := StatusRank(a.WorstStatus), StatusRank(b.WorstStatus); ra != rb {
			return ra > rb
		}
		if a.GapCount != b.GapCount {
			return a.GapCount > b.GapCount
		}
		return a.Slice < b.Slice
	})
	return report
}
