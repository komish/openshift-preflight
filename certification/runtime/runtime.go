// Package runtime contains the structs and definitions consumed by Preflight at
// runtime.
package runtime

import (
	"strings"
	"time"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/spf13/viper"
)

type Config struct {
	Image          string
	EnabledChecks  []string
	ResponseFormat string
	Mounted        bool
	Bundle         bool
	Scratch        bool
	LogFile        string
	// Container-Specific Fields
	CertificationProjectID string
	PyxisHost              string
	PyxisAPIToken          string
	DockerConfig           string
	Submit                 bool
	// Operator-Specific Fields
}

// storeContainerPolicyConfiguration reads container-policy-specific config
// items in viper, normalizes them, and stores them in Config.
func (c *Config) storeContainerPolicyConfiguration(vcfg viper.Viper) {
	c.PyxisAPIToken = vcfg.GetString("pyxis_api_token")
	c.DockerConfig = vcfg.GetString("dockerConfig")
	c.Submit = vcfg.GetBool("submit")
	c.PyxisHost = pyxisHostLookup(vcfg.GetString("pyxis_env"), vcfg.GetString("pyxis_host"))

	// Strip the ospid- prefix from the project ID if provided.
	certificationProjectID := vcfg.GetString("certification_project_id")
	if strings.HasPrefix(certificationProjectID, "ospid-") {
		certificationProjectID = strings.Split(certificationProjectID, "-")[1]
	}
	c.CertificationProjectID = certificationProjectID
}

// NewConfigFrom will return a runtime.Config based on the stored inputs in
// the provided viper.Viper
func NewConfigFrom(vcfg viper.Viper) (*Config, error) {
	cfg := Config{}
	cfg.LogFile = vcfg.GetString("logfile")
	cfg.storeContainerPolicyConfiguration(vcfg)

	return &cfg, nil
}

type Result struct {
	certification.Check
	ElapsedTime time.Duration
}

type Results struct {
	TestedImage       string
	PassedOverall     bool
	TestedOn          OpenshiftClusterVersion
	CertificationHash string
	Passed            []Result
	Failed            []Result
	Errors            []Result
}

type OpenshiftClusterVersion struct {
	Name    string
	Version string
}

func UnknownOpenshiftClusterVersion() OpenshiftClusterVersion {
	return OpenshiftClusterVersion{
		Name:    "unknown",
		Version: "unknown",
	}
}
