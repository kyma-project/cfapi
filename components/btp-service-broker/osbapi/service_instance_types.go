package osbapi

type ProvisionRequest struct {
	ServiceId        string          `json:"service_id"`
	PlanId           string          `json:"plan_id"`
	Context          any             `json:"context"`
	OrganizationGuid string          `json:"organization_guid"`
	SpaceGuid        string          `json:"space_guid"`
	Parameters       map[string]any  `json:"parameters"`
	MaintenanceInfo  MaintenanceInfo `json:"maintenance_info"`
}

type MaintenanceInfo struct {
	Version     string `json:"version"`
	Description string `json:"description"`
}
