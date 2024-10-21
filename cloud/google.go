package cloud

// GoogleConfig defines the configuration for Google Cloud
type GoogleConfig struct {

	// ServiceAccountKeyPath is the path to the service account key
	ServiceAccountKeyPath string `yaml:"service_account_key_path"`

	// StorageBucket is the storage bucket
	StorageBucket string `yaml:"storage_bucket"`
}
