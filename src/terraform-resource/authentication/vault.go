package authentication

import (
	"github.com/hashicorp/vault/api"
)

func GetAwsCredentials(v *VaultConfiguration) (AwsCredentials, error) {
	awsCreds := AwsCredentials{}

	// Setting the Address where Vault can be found
	conf := api.Config{
		Address: v.Address,
	}

	// Configuring the skip of TLS check
	tlsConfig := api.TLSConfig{
		// Insecure enables or disables SSL verification
		Insecure: true,
	}
	conf.ConfigureTLS(&tlsConfig)

	// Initialising the client
	client, err := api.NewClient(&conf)
	if err != nil {
		return awsCreds, err
	}

	// Setting the provided token
	client.SetToken(v.Token)

	// Writing the TTL
	secretData := map[string]interface{}{
		"ttl": v.TTL,
	}
	secret, err := client.Logical().Write(v.SecretPath, secretData)
	if err != nil {
		return awsCreds, err
	}

	if secret.Data["access_key"] != nil {
		awsCreds.AccessKeyID = secret.Data["access_key"].(string)
	}
	if secret.Data["secret_key"] != nil {
		awsCreds.SecretAccessKey = secret.Data["secret_key"].(string)
	}
	if secret.Data["security_token"] != nil {
		awsCreds.SessionToken = secret.Data["security_token"].(string)
	}

	return awsCreds, nil
}
