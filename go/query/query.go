package query

// ProdServiceTrafficRecord is the record of a service's traffic in production.
type ProdServiceTrafficRecord struct {
	// ServiceName is the name of the service.
	ServiceName string `json:"uber_service"`
	// AppearedAsCaller indicates whether the service appeared as a caller in production for the past 30 days.
	AppearedAsCaller bool `json:"appeared_as_caller"`
	// AppearedAsCallee indicates whether the service appeared as a callee in production for the past 30 days.
	AppearedAsCallee bool `json:"appeared_as_callee"`
}
