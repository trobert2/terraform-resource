package authentication

import (
	"fmt"
	"strings"
)

type VaultConfiguration struct {
	Address    string `json:"address"`
	SecretPath string `json:"secret_path"`
	Token      string `json:"token"`
	TTL        int    `json:"ttl"`
}

type AwsCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func (m VaultConfiguration) Validate() error {

	missingFields := []string{}
	fieldPrefix := "VaultConfig"
	if m.Address == "" {
		missingFields = append(missingFields, fmt.Sprintf("%s.address", fieldPrefix))
	}
	if m.SecretPath == "" {
		missingFields = append(missingFields, fmt.Sprintf("%s.secret_path", fieldPrefix))
	}
	if m.Token == "" {
		missingFields = append(missingFields, fmt.Sprintf("%s.token", fieldPrefix))
	}
	if m.TTL == 0 {
		missingFields = append(missingFields, fmt.Sprintf("%d.ttl", fieldPrefix))
	}

	if len(missingFields) > 0 {
		for i, value := range missingFields {
			missingFields[i] = fmt.Sprintf("'%s'", value)
		}
		return fmt.Errorf("Missing fields: %s", strings.Join(missingFields, ", "))
	}
	return nil
}
