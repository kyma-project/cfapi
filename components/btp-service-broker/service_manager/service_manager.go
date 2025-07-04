package service_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyma-project/cfapi/osbapi"
)

type ServiceManager struct {
	SmToken      Token
	SmUrl        string
	TokenUrl     string
	ClientId     string
	ClientSecret string
}


type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	Jti         string `json:"jti"`

	dateCreated time.Time
}

func (r *Token) Expired() bool {
	if r.dateCreated.IsZero() {
		return true
	}

	d := time.Since(r.dateCreated)
	t := time.Duration((r.ExpiresIn - 10) * 1000000000)
	return d >= t
}

type ServiceOfferingResponse struct {
	Token    string            `json:"token"`
	NumItems int               `json:"num_items"`
	Items    []ServiceOffering `json:"items"`
}

type ServiceOffering struct {
	Id                   string   `json:"id"`
	Ready                bool     `json:"ready"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Bindable             bool     `json:"bindable"`
	InstancesRetrievable bool     `json:"instances_retrievable"`
	BindingsRetrievable  bool     `json:"bindings_retrievable"`
	PlanUpdateable       bool     `json:"plan_updateable"`
	AllowContextUpdates  bool     `json:"allow_context_updates"`
	Tags                 []string `json:"tags"`
	Metadata             struct {
		LongDescription     string `json:"longDescription"`
		DocumentationUrl    string `json:"documentationUrl"`
		ProviderDisplayName string `json:"providerDisplayName"`
		ServiceInventoryId  string `json:"serviceInventoryId"`
		DisplayName         string `json:"displayName"`
		ImageUrl            string `json:"imageUrl"`
		SupportUrl          string `json:"supportUrl"`
	} `json:"metadata"`
	BrokerId    string    `json:"broker_id"`
	CatalogId   string    `json:"catalog_id"`
	CatalogName string    `json:"catalog_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ServicePlanResponse struct {
	Token    string        `json:"token"`
	NumItems int           `json:"num_items"`
	Items    []ServicePlan `json:"items"`
}

type ServicePlan struct {
	Id          string `json:"id"`
	Ready       bool   `json:"ready"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CatalogId   string `json:"catalog_id"`
	CatalogName string `json:"catalog_name"`
	Free        bool   `json:"free"`
	Bindable    bool   `json:"bindable"`
	Metadata    struct {
		SupportsInstanceSharing bool     `json:"supportsInstanceSharing"`
		SupportedPlatforms      []string `json:"supportedPlatforms"`
		// SupportedMinOSBVersion  int      `json:"supportedMinOSBVersion,int"`
		SiblingResolution struct {
			ResolutionProperty string   `json:"resolution_property"`
			NamePaths          []string `json:"name_paths"`
			ValueRegexp        string   `json:"value_regexp"`
			Enabled            bool     `json:"enabled"`
		} `json:"sibling_resolution"`
		// SupportedMaxOSBVersion int      `json:"supportedMaxOSBVersion,int"`
	} `json:"metadata"`
	ServiceOfferingId string    `json:"service_offering_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Labels            struct {
		CommercialName []string `json:"commercial_name"`
	} `json:"labels"`
}

func (s *ServiceManager) request(url string, data interface{}) error {
	err := s.ensureToken()
	if err != nil {
		return err
	}

	request, err := http.NewRequest("GET", s.SmUrl+url, nil)
	if err != nil {
		return err
	}

	request.Header.Add("Authorization", "Bearer "+s.SmToken.AccessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errorDetails := map[string]any{}
		_ = json.NewDecoder(response.Body).Decode(&errorDetails)

		if response.StatusCode == http.StatusTooManyRequests {
			retryAfter := response.Header.Get("Retry-After")
			return fmt.Errorf("too many requests. retry-after %s: %v", retryAfter, errorDetails)

		}
		return fmt.Errorf("request failed. status code %d: %v", response.StatusCode, errorDetails)
	}

	err = json.NewDecoder(response.Body).Decode(data)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServiceManager) GetCatalog(ctx context.Context) (osbapi.Catalog, error) {
	logger := logr.FromContextOrDiscard(ctx).WithName("sm-client.getcatalog")
	offerings, err := s.getServiceOfferings()
	if err != nil {
		return osbapi.Catalog{}, err
	}
	logger.Info("service offerings", "count", len(offerings))

	plans, err := s.getServicePlans()
	if err != nil {
		return osbapi.Catalog{}, err
	}
	logger.Info("service plans", "count", len(plans))

	catalog := osbapi.Catalog{}
	for _, offering := range offerings {
		service := osbapi.Service{
			Name:           offering.Name,
			Id:             offering.Id,
			Description:    offering.Description,
			Tags:           offering.Tags,
			Requires:       []string{},
			Bindable:       offering.Bindable,
			Metadata:       offering.Metadata,
			PlanUpdateable: offering.PlanUpdateable,
		}

		for _, p := range plans[offering.Id] {
			service.Plans = append(service.Plans, osbapi.Plan{
				Id:          p.Id,
				Name:        p.Name,
				Description: p.Description,
				Metadata: osbapi.PlanMetadata{
					Bullets:     []string{},
					Costs:       []any{},
					DisplayName: p.Name,
				},
				Free:     p.Free,
				Bindable: p.Bindable,
			})
		}

		catalog.Services = append(catalog.Services, service)
	}

	return catalog, nil
}

func (s *ServiceManager) getServiceOfferings() ([]ServiceOffering, error) {
	offeringResponse := &ServiceOfferingResponse{}
	err := s.request("/v1/service_offerings", offeringResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to get offerings: %w", err)
	}

	return offeringResponse.Items, nil
}

// Returns map service-offering-id to service plans
func (s *ServiceManager) getServicePlans() (map[string][]ServicePlan, error) {
	plansResponse := &ServicePlanResponse{}
	err := s.request("/v1/service_plans", plansResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to get plans: %w", err)
	}

	result := map[string][]ServicePlan{}
	for _, plan := range plansResponse.Items {
		if _, ok := result[plan.ServiceOfferingId]; !ok {
			result[plan.ServiceOfferingId] = []ServicePlan{}
		}
		result[plan.ServiceOfferingId] = append(result[plan.ServiceOfferingId], plan)
	}

	return result, nil
}

func (s *ServiceManager) getAuthToken() (Token, error) {
	response, err := http.PostForm(s.TokenUrl+"/oauth/token", url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {s.ClientId},
		"client_secret": {s.ClientSecret},
	})
	if err != nil {
		return Token{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		return Token{}, fmt.Errorf("failed to get oauth token(status code: %d): %s", response.StatusCode, string(body))
	}

	token := Token{dateCreated: time.Now()}

	err = json.NewDecoder(response.Body).Decode(&token)
	if err != nil {
		return Token{}, err
	}

	return token, nil
}

func (s *ServiceManager) ensureToken() error {
	if !s.SmToken.Expired() {
		return nil
	}

	var err error
	s.SmToken, err = s.getAuthToken()
	return err
}
