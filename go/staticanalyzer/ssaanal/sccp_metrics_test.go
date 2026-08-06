package ssaanal

import "testing"

func TestSCCPAggregateMetrics_NilSafety(t *testing.T) {
	// All Inc methods should be no-ops on nil receiver (no panic).
	var m *SCCPAggregateMetrics
	m.IncTotalInvocations()
	m.IncFailClose()
	m.IncFailOpen()
	m.IncFailUnknown()
	m.IncNonSuppressionReached()
	m.IncNonSuppressionPass()
	m.IncNonSuppressionFail()
	m.IncAttributionReached()
	m.IncAttributionPass()
	m.IncAttributionFail()
	m.IncDataAttrCount()
	m.IncDataAttrPass()
	m.IncDataAttrFail()
	m.IncControlAttrCount()
	m.IncControlAttrPass()
	m.IncControlAttrFail()
	m.IncInverseReached()
	m.IncInversePass()
	m.IncInverseFail()

	m.IncCFGPathsAnalyzed()

	if m.AsMap() != nil {
		t.Error("AsMap() on nil receiver should return nil")
	}
}

func TestSCCPAggregateMetrics_AsMap(t *testing.T) {
	m := &SCCPAggregateMetrics{}
	m.IncTotalInvocations()
	m.IncTotalInvocations()
	m.IncFailClose()
	m.IncFailOpen()
	m.IncNonSuppressionReached()
	m.IncNonSuppressionPass()

	result := m.AsMap()
	if result == nil {
		t.Fatal("AsMap() on non-nil receiver should not return nil")
	}
	if got := result["sccp_total_invocations"]; got != 2 {
		t.Errorf("sccp_total_invocations = %v, want 2", got)
	}
	if got := result["sccp_failclose_count"]; got != 1 {
		t.Errorf("sccp_failclose_count = %v, want 1", got)
	}
	if got := result["sccp_failopen_count"]; got != 1 {
		t.Errorf("sccp_failopen_count = %v, want 1", got)
	}

	// Verify all 19 expected keys are present.
	expectedKeys := []string{
		"sccp_total_invocations",
		"sccp_failclose_count", "sccp_failopen_count", "sccp_failunknown_count",
		"sccp_nonsuppression_reached", "sccp_nonsuppression_pass", "sccp_nonsuppression_fail",
		"sccp_attribution_reached", "sccp_attribution_pass", "sccp_attribution_fail",
		"sccp_data_attr_count", "sccp_data_attr_pass", "sccp_data_attr_fail",
		"sccp_control_attr_count", "sccp_control_attr_pass", "sccp_control_attr_fail",
		"sccp_inverse_reached", "sccp_inverse_pass", "sccp_inverse_fail",
		"sccp_cfg_paths_analyzed",
	}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("AsMap() missing key %q", key)
		}
	}
}
