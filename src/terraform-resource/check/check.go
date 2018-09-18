package check

import (
	"fmt"
	"time"

	"terraform-resource/authentication"
	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

type Runner struct{}

func (r Runner) Run(req models.InRequest) ([]models.Version, error) {
	currentVersionTime := time.Time{}
	if req.Version.IsZero() == false {
		if err := req.Version.Validate(); err != nil {
			return nil, fmt.Errorf("Failed to validate provided version: %s", err)
		}
		currentVersionTime = req.Version.LastModifiedTime()
	}

	// TODO: instead of copying this to "out", "check" and "in", abstract it and make it available to call.
	vaultAuthConfigModel := req.Source.VaultConfiguration
	if vaultAuthConfigModel != (authentication.VaultConfiguration{}) {
		if err := vaultAuthConfigModel.Validate(); err != nil {
			return nil, fmt.Errorf("Failed to validate vault Config Model: %s", err)
		}

		credentials, err := authentication.GetAwsCredentials(&vaultAuthConfigModel)
		if err != nil {
			return nil, fmt.Errorf("Failed to grab AWS credentials from Vault: %s", err)
		}

		// Set the fetched credentials to be used in the storage driver
		req.Source.Storage.AccessKeyID = credentials.AccessKeyID
		req.Source.Storage.SecretAccessKey = credentials.SecretAccessKey
		req.Source.Storage.SessionToken = credentials.SessionToken
	}

	storageModel := req.Source.Storage
	if err := storageModel.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	stateFile := terraform.StateFile{
		StorageDriver: storageDriver,
	}

	storageVersion, err := stateFile.LatestVersion()
	if err != nil {
		return nil, fmt.Errorf("Failed to check storage backend for latest version: %s", err)
	}

	resp := []models.Version{}
	if storageVersion.IsZero() == false && storageVersion.LastModified.After(currentVersionTime) {
		version := models.NewVersion(storageVersion)
		resp = append(resp, version)
	}

	return resp, nil
}
