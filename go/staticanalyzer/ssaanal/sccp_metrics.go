package ssaanal

// SCCPAggregateMetrics accumulates metrics across multiple SCCP invocations.
// Passed via SCCPOptions.Metrics; each AnalyzeCallBySCCPWithOptions call
// updates it in place. Thread-safety is not needed (single-threaded driver).
type SCCPAggregateMetrics struct {
	TotalInvocations int

	// Result breakdown
	FailCloseCount   int
	FailOpenCount    int
	FailUnknownCount int

	// Non-suppression phase: per-return counts summed across invocations
	NonSuppressionReached int
	NonSuppressionPass    int
	NonSuppressionFail    int

	// Attribution phase: reached only when non-suppression passes
	AttributionReached int
	AttributionPass    int
	AttributionFail    int

	// Attribution sub-breakdown
	DataAttrCount    int
	DataAttrPass     int
	DataAttrFail     int
	ControlAttrCount int
	ControlAttrPass  int
	ControlAttrFail  int

	// Inverse phase: reached only when forward passes + InverseCheck enabled
	InverseReached int
	InversePass    int
	InverseFail    int

	// CFG exploration: return paths that reached phase 2/3 analysis.
	CFGPathsAnalyzed int
}

// AsMap returns the metrics as map[string]float64 for RunMetrics integration.
// All keys are prefixed with "sccp_".
func (m *SCCPAggregateMetrics) AsMap() map[string]float64 {
	if m == nil {
		return nil
	}
	return map[string]float64{
		"sccp_total_invocations": float64(m.TotalInvocations),

		"sccp_failclose_count":   float64(m.FailCloseCount),
		"sccp_failopen_count":    float64(m.FailOpenCount),
		"sccp_failunknown_count": float64(m.FailUnknownCount),

		"sccp_nonsuppression_reached": float64(m.NonSuppressionReached),
		"sccp_nonsuppression_pass":    float64(m.NonSuppressionPass),
		"sccp_nonsuppression_fail":    float64(m.NonSuppressionFail),

		"sccp_attribution_reached": float64(m.AttributionReached),
		"sccp_attribution_pass":    float64(m.AttributionPass),
		"sccp_attribution_fail":    float64(m.AttributionFail),

		"sccp_data_attr_count": float64(m.DataAttrCount),
		"sccp_data_attr_pass":  float64(m.DataAttrPass),
		"sccp_data_attr_fail":  float64(m.DataAttrFail),

		"sccp_control_attr_count": float64(m.ControlAttrCount),
		"sccp_control_attr_pass":  float64(m.ControlAttrPass),
		"sccp_control_attr_fail":  float64(m.ControlAttrFail),

		"sccp_inverse_reached": float64(m.InverseReached),
		"sccp_inverse_pass":    float64(m.InversePass),
		"sccp_inverse_fail":    float64(m.InverseFail),

		"sccp_cfg_paths_analyzed": float64(m.CFGPathsAnalyzed),
	}
}

// Increment methods — all nil-safe (no-op on nil receiver).

func (m *SCCPAggregateMetrics) IncTotalInvocations() {
	if m != nil {
		m.TotalInvocations++
	}
}
func (m *SCCPAggregateMetrics) IncFailClose() {
	if m != nil {
		m.FailCloseCount++
	}
}
func (m *SCCPAggregateMetrics) IncFailOpen() {
	if m != nil {
		m.FailOpenCount++
	}
}
func (m *SCCPAggregateMetrics) IncFailUnknown() {
	if m != nil {
		m.FailUnknownCount++
	}
}
func (m *SCCPAggregateMetrics) IncNonSuppressionReached() {
	if m != nil {
		m.NonSuppressionReached++
	}
}
func (m *SCCPAggregateMetrics) IncNonSuppressionPass() {
	if m != nil {
		m.NonSuppressionPass++
	}
}
func (m *SCCPAggregateMetrics) IncNonSuppressionFail() {
	if m != nil {
		m.NonSuppressionFail++
	}
}
func (m *SCCPAggregateMetrics) IncAttributionReached() {
	if m != nil {
		m.AttributionReached++
	}
}
func (m *SCCPAggregateMetrics) IncAttributionPass() {
	if m != nil {
		m.AttributionPass++
	}
}
func (m *SCCPAggregateMetrics) IncAttributionFail() {
	if m != nil {
		m.AttributionFail++
	}
}
func (m *SCCPAggregateMetrics) IncDataAttrCount() {
	if m != nil {
		m.DataAttrCount++
	}
}
func (m *SCCPAggregateMetrics) IncDataAttrPass() {
	if m != nil {
		m.DataAttrPass++
	}
}
func (m *SCCPAggregateMetrics) IncDataAttrFail() {
	if m != nil {
		m.DataAttrFail++
	}
}
func (m *SCCPAggregateMetrics) IncControlAttrCount() {
	if m != nil {
		m.ControlAttrCount++
	}
}
func (m *SCCPAggregateMetrics) IncControlAttrPass() {
	if m != nil {
		m.ControlAttrPass++
	}
}
func (m *SCCPAggregateMetrics) IncControlAttrFail() {
	if m != nil {
		m.ControlAttrFail++
	}
}
func (m *SCCPAggregateMetrics) IncInverseReached() {
	if m != nil {
		m.InverseReached++
	}
}
func (m *SCCPAggregateMetrics) IncInversePass() {
	if m != nil {
		m.InversePass++
	}
}
func (m *SCCPAggregateMetrics) IncInverseFail() {
	if m != nil {
		m.InverseFail++
	}
}

func (m *SCCPAggregateMetrics) IncCFGPathsAnalyzed() {
	if m != nil {
		m.CFGPathsAnalyzed++
	}
}
