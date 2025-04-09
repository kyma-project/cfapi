package osbapi

type BindRequest struct {
	Context      any          `json:"context"`
	ServiceId    string       `json:"service_id"`
	PlanId       string       `json:"plan_id"`
	BindResource BindResource `json:"bind_resource"`
	Parameters   any          `json:"parameters"`
}

type BindResource struct {
	AppGuid string `json:"app_guid"`
}

type ServiceBindingResponse struct {
	Metadata        BindingMetadata `json:"metadata"`
	Credentials     any             `json:"credentials"`
	SyslogDrainUrl  string          `json:"syslog_drain_url"`
	RouteServiceUrl string          `json:"route_service_url"`
	VolumeMounts    []VolumeMount   `json:"volume_mounts"`
	Endpoints       []Endpoint      `json:"endpoints"`
	Parameters      map[string]any  `json:"parameters"`
}

type BindingMetadata struct {
	ExpiresAt   string `json:"expires_at"`
	RenewBefore string `json:"renew_before"`
}

type VolumeMount struct {
	Driver       string `json:"driver"`
	ContainerDir string `json:"container_dir"`
	Mode         string `json:"mode"`
	DeviceType   string `json:"device_type"`
	Device       Device `json:"device"`
}

type Device struct {
	VolumeId    string `json:"volume_id"`
	MountConfig any    `json:"mount_config"`
}

type Endpoint struct {
	Host     string   `json:"host"`
	Ports    []string `json:"ports"`
	Protocol string   `json:"protocol"`
}
