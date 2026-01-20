package bindings

import (
	"encoding/json"
	"errors"
	"fmt"
)

type bindingCredentialMetadata struct {
	CredentialProperties []credentialProperties `json:"credentialProperties"`
}

type credentialProperties struct {
	Name   string `json:"name"`
	Format string `json:"format"`
}

// CredentialsDecoder decodes BTP operator service binding secret
// BTP operator encodes credentials object into the BTP binding secret in its own manner,
// see https://github.com/SAP/sap-btp-service-operator/blob/d9ca02ab478110bef94cbe3a50147e2920e4e4c4/internal/utils/controller_util.go#L63
// This decoder is doing the revers so that we can restore the original credentials object
type CredentialsDecoder struct{}

func NewCredentialsDecoder() *CredentialsDecoder {
	return &CredentialsDecoder{}
}

func (c *CredentialsDecoder) DecodeBindingSecretData(secretData map[string][]byte) (any, error) {
	metadataBytes, ok := secretData[".metadata"]
	if !ok {
		return nil, errors.New("secret data does not contain .metadata key")
	}

	metadata := bindingCredentialMetadata{}
	err := json.Unmarshal(metadataBytes, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %v into metadata: %w", metadataBytes, err)
	}

	return decodeSecretKeys(secretData, metadata.CredentialProperties)
}

func decodeSecretKeys(secretData map[string][]byte, properties []credentialProperties) (any, error) {
	decodedObject := map[string]any{}
	for _, property := range properties {
		encodedSecretValue, ok := secretData[property.Name]
		if !ok {
			return nil, fmt.Errorf("missing credentials property with name %q", property.Name)
		}

		switch property.Format {
		case "json":
			var decodedValue any
			err := json.Unmarshal(encodedSecretValue, &decodedValue)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal credentials property %q: %w", property.Name, err)
			}
			decodedObject[property.Name] = decodedValue
		case "text":
			decodedObject[property.Name] = string(encodedSecretValue)
		default:
			return nil, fmt.Errorf("unsupported credentials property format %q for property %q", property.Format, property.Name)

		}
	}

	return decodedObject, nil
}
