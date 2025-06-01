package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlewi/cloud-assistant/app/api"

	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Note: The application uses viper for configuration management. Viper merges configurations from various sources
//such as files, environment variables, and command line flags. After merging, viper unmarshals the configuration into the Configuration struct, which is then used throughout the application.

const (
	ConfigFlagName = "config"
	LevelFlagName  = "level"
	AppName        = "cloud-assistant"
	ConfigDir      = "." + AppName
)

var (
	// globalV is the global instance of viper
	globalV *viper.Viper
)

// TODO(jeremy): It might be better to put the datastructures defining the configuration into the API package.
// The reason being we might want to share those data structures withother parts of the API (e.g. RPCs).
// However, we should keep the api package free of other dpendencies (e.g. viper, cobra, etc.). So that might
// necessitate some refactoring. We might want to use a separate struct defined here as a wrapper around
// the underlying data structure.

// Config represents the persistent configuration data.
//
// Currently, the format of the data on disk and in memory is identical. In the future, we may modify this to simplify
// changes to the disk format and to store in-memory values that should not be written to disk. Could that be achieved
// by embedding it in a different struct which contains values that shouldn't be serialized?
type Config struct {
	APIVersion string           `json:"apiVersion" yaml:"apiVersion" yamltags:"required"`
	Kind       string           `json:"kind" yaml:"kind" yamltags:"required"`
	Metadata   api.Metadata     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Logging    Logging          `json:"logging" yaml:"logging"`
	Telemetry  *TelemetryConfig `json:"telemetry,omitempty" yaml:"telemetry,omitempty"`

	OpenAI *OpenAIConfig `json:"openai,omitempty" yaml:"openai,omitempty"`

	CloudAssistant  *CloudAssistantConfig  `json:"cloudAssistant,omitempty" yaml:"cloudAssistant,omitempty"`
	AssistantServer *AssistantServerConfig `json:"assistantServer,omitempty" yaml:"assistantServer,omitempty"`

	// WebAppConfig is the configuration for the web application.
	WebApp *WebAppConfig `json:"webApp,omitempty" yaml:"webApp,omitempty"`

	// IAMPolicy is the IAM policy for the service. It only matters if OIDC is enabled in the AssistantServerConfig.
	IAMPolicy *api.IAMPolicy `json:"iamPolicy,omitempty" yaml:"iamPolicy,omitempty"`

	// configFile is the configuration file used
	configFile string
}

type WebAppConfig struct {
	// Runner is the address of the Runme runner service to use
	Runner string `json:"runner,omitempty" yaml:"runner,omitempty"`
}

type CloudAssistantConfig struct {
	// VectorStores is the list of vector stores to use
	VectorStores []string `json:"vectorStores,omitempty" yaml:"vectorStores,omitempty"`
	// SessionCookie is the value of the cassie-session cookie used for evaluation
	SessionCookie string `json:"sessionCookie,omitempty" yaml:"sessionCookie,omitempty"`
}

type OpenAIConfig struct {
	// APIKeyFile is the file containing the OpenAI API key
	APIKeyFile string `json:"apiKeyFile,omitempty" yaml:"apiKeyFile,omitempty"`
}

type Logging struct {
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
	// Use JSON logging
	JSON bool `json:"json,omitempty" yaml:"json,omitempty"`

	LogDir string `json:"logDir,omitempty" yaml:"logDir,omitempty"`
	// Sinks is a list of sinks to write logs to.
	// Use stderr to write to stderr.
	// Use gcplogs:///projects/${PROJECT}/logs/${LOGNAME} to write to Google Cloud Logging
	Sinks []LogSink `json:"sinks,omitempty" yaml:"sinks,omitempty"`

	LogFields *LogFields `json:"logFields,omitempty" yaml:"logFields,omitempty"`
}

// LogFields is the fields to use when logging to structured logging
type LogFields struct {
	Level   string `json:"level,omitempty" yaml:"level,omitempty"`
	Time    string `json:"time,omitempty" yaml:"time,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

type LogSink struct {
	// Set to true to write logs in JSON format
	JSON bool `json:"json,omitempty" yaml:"json,omitempty"`
	// Path is the path to write logs to. Use "stderr" to write to stderr.
	// Use gcplogs:///projects/${PROJECT}/logs/${LOGNAME} to write to Google Cloud Logging
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

type TelemetryConfig struct {
	Honeycomb *HoneycombConfig `json:"honeycomb,omitempty" yaml:"honeycomb,omitempty"`
}

type HoneycombConfig struct {
	// APIKeyFile is the Honeycomb API key
	APIKeyFile string `json:"apiKeyFile" yaml:"apiKeyFile"`
}

func (c *Config) GetLogLevel() string {
	if c.Logging.Level == "" {
		return "info"
	}
	return c.Logging.Level
}

// GetConfigFile returns the configuration file
func (c *Config) GetConfigFile() string {
	if c.configFile == "" {
		c.configFile = DefaultConfigFile()
	}
	return c.configFile
}

// GetConfigDir returns the configuration directory
func (c *Config) GetConfigDir() string {
	configFile := c.GetConfigFile()
	if configFile != "" {
		return filepath.Dir(configFile)
	}

	// Since there is no config file we will use the default config directory.
	return binHome()
}

// IsValid validates the configuration and returns any errors.
func (c *Config) IsValid() []string {
	problems := make([]string, 0, 1)
	return problems
}

func (c *Config) UseHoneycomb() bool {
	if c.Telemetry == nil {
		return false
	}
	if c.Telemetry.Honeycomb == nil {
		return false
	}
	if c.Telemetry.Honeycomb.APIKeyFile == "" {
		return false
	}
	return true
}

// DeepCopy returns a deep copy.
func (c *Config) DeepCopy() Config {
	b, err := json.Marshal(c)
	if err != nil {
		log := zapr.NewLogger(zap.L())
		log.Error(err, "Failed to marshal config")
		panic(err)
	}
	var copy Config
	if err := json.Unmarshal(b, &copy); err != nil {
		log := zapr.NewLogger(zap.L())
		log.Error(err, "Failed to unmarshal config")
		panic(err)
	}
	return copy
}

// InitViper function is responsible for reading the configuration file and environment variables, if they are set.
// The results are stored in viper. To retrieve a configuration, use the GetConfig function.
// The function accepts a cmd parameter which allows binding to command flags.
func InitViper(cmd *cobra.Command) error {
	// N.B. we need to set globalV because the subsequent call GetConfig will use that viper instance.
	// Would it make sense to combine InitViper and Get into one command that returns a config object?
	// TODO(jeremy): Could we just use viper.GetViper() to get the global instance?
	globalV = viper.New()
	return InitViperInstance(globalV, cmd)
}

// InitViperInstance function is responsible for reading the configuration file and environment variables, if they are set.
// The results are stored in viper. To retrieve a configuration, use the GetConfig function.
// The function accepts a cmd parameter which allows binding to command flags.
func InitViperInstance(v *viper.Viper, cmd *cobra.Command) error {
	// Ref https://github.com/spf13/viper#establishing-defaults
	v.SetEnvPrefix(AppName)

	if v.ConfigFileUsed() == "" {
		// If ConfigFile isn't already set then configure the search parameters.
		// The most likely scenario for it already being set is tests.

		// name of config file (without extension)
		v.SetConfigName("config")
		// make home directory the first search path
		v.AddConfigPath("$HOME/." + AppName)
	}

	// Without the replacer overriding with environment variables doesn't work
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv() // read in environment variables that match

	// TODO: Set any default values here

	// We need to attach to the command line flag if it was specified.
	keyToflagName := map[string]string{
		ConfigFlagName:             ConfigFlagName,
		"logging." + LevelFlagName: LevelFlagName,
	}

	if cmd != nil {
		for key, flag := range keyToflagName {
			if err := v.BindPFlag(key, cmd.Flags().Lookup(flag)); err != nil {
				return err
			}
		}
	}

	// Ensure the path for the config file path is set
	// Required since we use viper to persist the location of the config file so can save to it.
	// This allows us to overwrite the config file location with the --config flag.
	cfgFile := v.GetString(ConfigFlagName)
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log := zapr.NewLogger(zap.L())
			log.Error(err, "config file not found", "file", cfgFile)
			return nil
		}
		if _, ok := err.(*fs.PathError); ok {
			log := zapr.NewLogger(zap.L())
			log.Error(err, "config file not found", "file", cfgFile)
			return nil
		}
		return err
	}
	return nil
}

// GetConfig returns a configuration created from the viper configuration.
func GetConfig() *Config {
	if globalV == nil {
		// TODO(jeremy): Using a global variable to pass state between InitViper and GetConfig is wonky.
		// It might be better to combine InitViper and GetConfig into a single command that returns a config object.
		// This would also make viper an implementation detail of the config.
		panic("globalV is nil; was InitViper called before calling GetConfig?")
	}
	// We do this as a way to load the configuration while still allowing values to be overwritten by viper
	cfg, err := getConfigFromViper(globalV)
	if err != nil {
		panic(err)
	}
	return cfg
}

func getConfigFromViper(v *viper.Viper) (*Config, error) {
	// We do this as a way to load the configuration while still allowing values to be overwritten by viper
	cfg := &Config{}

	if err := v.Unmarshal(cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal configuration; error %v", err)
	}

	// Set the configFileUsed
	cfg.configFile = v.ConfigFileUsed()
	return cfg, nil
}

func binHome() string {
	log := zapr.NewLogger(zap.L())
	usr, err := user.Current()
	homeDir := ""
	if err != nil {
		log.Error(err, "failed to get current user; falling back to temporary directory for homeDir", "homeDir", os.TempDir())
		homeDir = os.TempDir()
	} else {
		homeDir = usr.HomeDir
	}
	p := filepath.Join(homeDir, ConfigDir)

	return p
}

// Write saves the configuration to a file.
func (c *Config) Write(cfgFile string) error {
	log := zapr.NewLogger(zap.L())
	if cfgFile == "" {
		return errors.Errorf("no config file specified")
	}
	configDir := filepath.Dir(cfgFile)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		log.Info("creating config directory", "dir", configDir)
		if err := os.Mkdir(configDir, 0700); err != nil {
			return errors.Wrapf(err, "Ffailed to create config directory %s", configDir)
		}
	}

	f, err := os.Create(cfgFile)
	if err != nil {
		return err
	}

	return yaml.NewEncoder(f).Encode(c)
}

func DefaultConfigFile() string {
	return binHome() + "/config.yaml"
}

type AssistantServerConfig struct {
	// BindAddress is the address to bind to. Default is 0.0.0.0
	BindAddress string `json:"bindAddress" yaml:"bindAddress"`

	// Port is the port for the server
	Port int `json:"port" yaml:"port"`

	// HttpMaxReadTimeout is the max read duration.
	// Ref: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts
	HttpMaxReadTimeout time.Duration `json:"httpMaxReadTimeout" yaml:"httpMaxReadTimeout"`

	// HttpMaxWriteTimeout is the max write duration.
	HttpMaxWriteTimeout time.Duration `json:"httpMaxWriteTimeout" yaml:"httpMaxWriteTimeout"`

	// CorsOrigins is a list of allowed origins for CORS requests
	CorsOrigins []string `json:"corsOrigins" yaml:"corsOrigins"`

	StaticAssets string `json:"staticAssets" yaml:"staticAssets"`

	// RunnerService starts the Runme runner service if true otherwise it doesn't start the runner service.
	RunnerService bool `json:"runnerService" yaml:"runnerService"`

	// OIDC configuration
	OIDC *OIDCConfig `json:"oidc,omitempty" yaml:"oidc,omitempty"`

	// TLSConfig is the TLS configuration
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
}

// OIDCConfig contains configuration for OIDC authentication
type OIDCConfig struct {
	// Google contains Google-specific OIDC configuration
	Google *GoogleOIDCConfig `json:"google,omitempty" yaml:"google,omitempty"`

	// Generic contains generic OIDC configuration
	Generic *GenericOIDCConfig `json:"generic,omitempty" yaml:"generic,omitempty"`

	// ForceApproval is a flag to force the user to approve the app again
	ForceApproval bool `json:"forceApproval" yaml:"forceApproval"`
}

// GoogleOIDCConfig contains Google-specific OIDC configuration
type GoogleOIDCConfig struct {
	// ClientCredentialsFile is the path to the file containing the Google client credentials
	ClientCredentialsFile string `json:"clientCredentialsFile" yaml:"clientCredentialsFile"`
	// DiscoveryURL is the URL for the OpenID Connect discovery document
	DiscoveryURL string `json:"discoveryURL" yaml:"discoveryURL"`
}

// GenericOIDCConfig contains configuration for a generic OIDC provider
type GenericOIDCConfig struct {
	// ClientID is the OAuth2 client ID
	ClientID string `json:"clientID" yaml:"clientID"`
	// ClientSecret is the OAuth2 client secret
	ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
	// RedirectURL is the URL to redirect users to after login
	RedirectURL string `json:"redirectURL" yaml:"redirectURL"`
	// DiscoveryURL is the URL for the OpenID Connect discovery document
	DiscoveryURL string `json:"discoveryURL" yaml:"discoveryURL"`
	// Scopes are the OAuth2 scopes to request (optional, defaults to ["openid", "email"])
	Scopes []string `json:"scopes" yaml:"scopes"`
	// Issuer allows overwriting the URL for the OpenID Connect issuer
	Issuer string `json:"issuer" yaml:"issuer"`
}

type TLSConfig struct {
	// Generate is a flag to generate a self-signed certificate if true.
	// If CertFile and KeyFile are also specified then the key is only generated if one doesn't already exist
	Generate bool `json:"generate" yaml:"generate"`

	// CertFile is the path to the TLS certificate file
	CertFile string `json:"certFile" yaml:"certFile"`
	// KeyFile is the path to the TLS key file
	KeyFile string `json:"keyFile" yaml:"keyFile"`
}

// Add a helper method to get the discovery URL with a default
func (c *GoogleOIDCConfig) GetDiscoveryURL() string {
	if c.DiscoveryURL != "" {
		return c.DiscoveryURL
	}
	return "https://accounts.google.com/.well-known/openid-configuration"
}

// GetDiscoveryURL returns the discovery URL for the generic OIDC provider
func (c *GenericOIDCConfig) GetDiscoveryURL() string {
	return c.DiscoveryURL
}

func (c *AssistantServerConfig) GetBindAddress() string {
	if c.BindAddress == "" {
		return "0.0.0.0"
	}
	return c.BindAddress
}

func (c *AssistantServerConfig) GetPort() int {
	if c.Port <= 0 {
		if c.TLSConfig != nil {
			return 8443
		}
		return 8080
	}
	return c.Port
}

func (c *AssistantServerConfig) GetHttpMaxReadTimeout() time.Duration {
	if c.HttpMaxReadTimeout > 0 {
		return c.HttpMaxReadTimeout
	}
	// See https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts
	// If we start using really slow models we may need to bump these to avoid timeouts.
	// This is just a guess on how we should set the timeout.
	return 5 * time.Minute
}

func (c *AssistantServerConfig) GetHttpMaxWriteTimeout() time.Duration {
	if c.HttpMaxWriteTimeout > 0 {
		return c.HttpMaxWriteTimeout
	}
	// See https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts
	// If we start using really slow models we may need to bump these to avoid timeouts.
	// This is just a guess on how we should set the timeout.
	return 5 * time.Minute
}

func (c *Config) GetLogDir() string {
	if c.Logging.LogDir != "" {
		return c.Logging.LogDir
	}

	return filepath.Join(c.GetConfigDir(), "logs")
}
