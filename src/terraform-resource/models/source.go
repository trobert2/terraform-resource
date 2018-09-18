package models

import (
	"terraform-resource/authentication"
	"terraform-resource/storage"
)

type Source struct {
	VaultConfiguration authentication.VaultConfiguration `json:"vault"`
	Storage            storage.Model                     `json:"storage"`
	Terraform
}
