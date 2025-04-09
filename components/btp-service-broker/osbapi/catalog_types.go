package osbapi

import "fmt"

type Catalog struct {
	Services []Service `json:"services"`
}

func (c Catalog) GetServiceByOfferingId(offeringId string) (Service, error) {
	for _, s := range c.Services {
		if s.Id == offeringId {
			return s, nil
		}
	}

	return Service{}, fmt.Errorf("service with offering ID %q not found", offeringId)
}

type Service struct {
	Name           string   `json:"name"`
	Id             string   `json:"id"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
	Requires       []string `json:"requires"`
	Bindable       bool     `json:"bindable"`
	Metadata       any      `json:"metadata"`
	PlanUpdateable bool     `json:"plan_updateable"`
	Plans          []Plan   `json:"plans"`
}

func (s Service) GetPlanById(planId string) (Plan, error) {
	for _, p := range s.Plans {
		if p.Id == planId {
			return p, nil
		}
	}

	return Plan{}, fmt.Errorf("plan %q not found for service %q", planId, s.Id)
}

type Plan struct {
	Id          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Metadata    PlanMetadata `json:"metadata"`
	Free        bool         `json:"free"`
	Bindable    bool         `json:"bindable"`
	Schemas     Schemas      `json:"schemas"`
}

type PlanMetadata struct {
	Bullets     []string `json:"bullets"`
	Costs       []any    `json:"costs"`
	DisplayName string   `json:"displayName"`
}

type Schemas struct {
	ServiceInstance ServiceInstanceSchema `json:"service_instance"`
	ServiceBinding  ServiceBindingSchema  `json:"service_binding"`
}

type ServiceInstanceSchema struct {
	Create InputParametersSchema `json:"create"`
	Update InputParametersSchema `json:"update"`
}

type ServiceBindingSchema struct {
	Create InputParametersSchema `json:"create"`
}

type InputParametersSchema struct {
	parameters map[string]any `json:"parameters"`
}
