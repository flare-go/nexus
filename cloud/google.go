package cloud

// GoogleConfig defines the configuration for Google Cloud
type GoogleConfig struct {

	// ServiceAccountKeyPath is the path to the service account key
	ServiceAccountKeyPath string `yaml:"service_account_key_path"`

	// StorageBucket is the storage bucket
	StorageBucket string `yaml:"storage_bucket"`

	// OAuth defines the OAuth configuration
	OAuth *GoogleOAuthConfig `yaml:"oauth"`
}

// GoogleOAuthConfig defines the configuration for Google OAuth
type GoogleOAuthConfig struct {
	// Web contains the OAuth credentials for web applications
	Web *GoogleOAuthWebConfig `yaml:"web"`
}

// GoogleOAuthWebConfig defines the web-specific OAuth configuration
type GoogleOAuthWebConfig struct {
	ClientID                string   `yaml:"client_id"`
	ProjectID               string   `yaml:"project_id"`
	AuthURI                 string   `yaml:"auth_uri"`
	TokenURI                string   `yaml:"token_uri"`
	AuthProviderX509CertURL string   `yaml:"auth_provider_x509_cert_url"`
	ClientSecret            string   `yaml:"client_secret"`
	RedirectURIs            []string `yaml:"redirect_uris"`
	JavaScriptOrigins       []string `yaml:"javascript_origins"`
}
