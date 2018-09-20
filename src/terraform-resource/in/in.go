package in

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"terraform-resource/authentication"
	"terraform-resource/encoder"
	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

type Runner struct {
	OutputDir string
}

func (r Runner) Run(req models.InRequest) (models.InResponse, error) {
	if err := req.Version.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Invalid Version request: %s", err)
	}

	envName := req.Version.EnvName
	nameFilepath := path.Join(r.OutputDir, "name")
	if err := ioutil.WriteFile(nameFilepath, []byte(envName), 0644); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create name file at path '%s': %s", nameFilepath, err)
	}

	if req.Params.Action == models.DestroyAction {
		resp := models.InResponse{
			Version: req.Version,
		}
		return resp, nil
	}

	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-in")
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	// TODO: instead of copying this to "out", "check" and "in", abstract it and make it available to call.
	vaultAuthConfigModel := req.Source.VaultConfiguration
	credentials := authentication.AwsCredentials{}
	if vaultAuthConfigModel != (authentication.VaultConfiguration{}) {
		if err := vaultAuthConfigModel.Validate(); err != nil {
			return models.InResponse{}, fmt.Errorf("Failed to validate vault Config Model: %s", err)
		}

		credentials, err = authentication.GetAwsCredentials(&vaultAuthConfigModel)
		if err != nil {
			return models.InResponse{}, fmt.Errorf("Failed to grab AWS credentials from Vault: %s", err)
		}

		// Set the fetched credentials to be used in the storage driver
		req.Source.Storage.AccessKeyID = credentials.AccessKeyID
		req.Source.Storage.SecretAccessKey = credentials.SecretAccessKey
		req.Source.Storage.SessionToken = credentials.SessionToken
	}

	storageModel := req.Source.Storage
	if err = storageModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	stateFile := terraform.StateFile{
		LocalPath:     path.Join(tmpDir, "terraform.tfstate"),
		RemotePath:    fmt.Sprintf("%s.tfstate", req.Version.EnvName),
		StorageDriver: storageDriver,
	}
	existsAsTainted, err := stateFile.ExistsAsTainted()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to check for tainted state file: %s", err)
	}
	if existsAsTainted {
		stateFile = stateFile.ConvertToTainted()
	}

	stateFileExists, err := stateFile.Exists()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to check for existing state file: %s", err)
	}
	if !stateFileExists {
		if req.Version.IsPlan() {
			resp := models.InResponse{
				Version: req.Version,
			}
			return resp, nil
		}

		return models.InResponse{}, fmt.Errorf(
			"State file does not exist with key '%s'."+
				"\nIf you intended to run the `destroy` action, add `put.get_params.action: destroy`."+
				"\nThis is a temporary requirement until Concourse supports a `delete` step.",
			stateFile.RemotePath,
		)
	}

	terraformModel := models.Terraform{
		StateFileLocalPath:  stateFile.LocalPath,
		StateFileRemotePath: stateFile.RemotePath,
	}

	if credentials != (authentication.AwsCredentials{}) {
		// Set the fetched credentials to be used in the terraform environment
		envMap := make(map[string]string)

		envMap["AWS_ACCESS_KEY_ID"] = credentials.AccessKeyID
		envMap["TF_VAR_access_key"] = credentials.SessionToken
		envMap["AWS_SECRET_ACCESS_KEY"] = credentials.SecretAccessKey
		envMap["TF_VAR_secret_key"] = credentials.SessionToken
		if len(credentials.SessionToken) > 0 {
			envMap["AWS_SESSION_TOKEN"] = credentials.SessionToken
			envMap["TF_VAR_session_token"] = credentials.SessionToken
		}

		// Add the values to the Terraform Env
		if terraformModel.Env == nil {
			// No vaules were provided in the source. We directly add our map
			terraformModel.Env = envMap
		} else {
			// There are already Env values coming from source. We merge the maps
			for k, v := range envMap {
				terraformModel.Env[k] = v
			}
		}
	}

	if req.Params.OutputModule != "" {
		terraformModel.OutputModule = req.Params.OutputModule
	}

	if err = terraformModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.Client{
		Model:         terraformModel,
		StorageDriver: storageDriver,
	}

	storageVersion, err := stateFile.Download()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to download state file from storage backend: %s", err)
	}
	version := models.NewVersion(storageVersion)

	tfOutput, err := client.Output()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to parse terraform output.\nError: %s", err)
	}
	result := terraform.Result{
		Output: tfOutput,
	}

	outputFilepath := path.Join(r.OutputDir, "metadata")
	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create output file at path '%s': %s", outputFilepath, err)
	}

	if err := encoder.NewJSONEncoder(outputFile).Encode(result.RawOutput()); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to write output file: %s", err)
	}

	metadata := []models.MetadataField{}
	for key, value := range result.SanitizedOutput() {
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: value,
		})
	}

	tfVersion, err := client.Version()
	if err != nil {
		return models.InResponse{}, err
	}
	metadata = append(metadata, models.MetadataField{
		Name:  "terraform_version",
		Value: tfVersion,
	})

	if req.Params.OutputStatefile {
		stateFilePath := path.Join(r.OutputDir, "terraform.tfstate")
		stateContents, err := ioutil.ReadFile(terraformModel.StateFileLocalPath)
		if err != nil {
			return models.InResponse{}, err
		}
		err = ioutil.WriteFile(stateFilePath, stateContents, 0777)
		if err != nil {
			return models.InResponse{}, err
		}
	}

	resp := models.InResponse{
		Version:  version,
		Metadata: metadata,
	}
	return resp, nil
}
