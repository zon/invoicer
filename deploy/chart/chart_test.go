package chart_test

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func chartDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Dir(file)
}

func decode(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

type chartMetadata struct {
	APIVersion  string `yaml:"apiVersion"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion"`
}

func TestChartMetadata(t *testing.T) {
	var chart chartMetadata
	decode(t, filepath.Join(chartDir(t), "Chart.yaml"), &chart)

	equal(t, "apiVersion", chart.APIVersion, "v2")
	equal(t, "name", chart.Name, "invoicer")
	equal(t, "type", chart.Type, "application")
	if chart.Description == "" {
		t.Fatal("Chart.yaml has no description")
	}
	if chart.Version == "" {
		t.Fatal("Chart.yaml has no version")
	}
	if chart.AppVersion == "" {
		t.Fatal("Chart.yaml has no appVersion")
	}
}

type values struct {
	Image struct {
		Repository  string   `yaml:"repository"`
		Tag         string   `yaml:"tag"`
		PullPolicy  string   `yaml:"pullPolicy"`
		PullSecrets []string `yaml:"pullSecrets"`
	} `yaml:"image"`
	Hostname  string `yaml:"hostname"`
	PublicURL string `yaml:"publicURL"`
	Config    struct {
		Vendor   string `yaml:"vendor"`
		Customer string `yaml:"customer"`
		Project  string `yaml:"project"`
		Rate     int    `yaml:"rate"`
		Hours    int    `yaml:"hours"`
		Database string `yaml:"database"`
	} `yaml:"config"`
	Access struct {
		Issuer          string `yaml:"issuer"`
		ClientID        string `yaml:"clientId"`
		AdminGroup      string `yaml:"adminGroup"`
		CustomerGroup   string `yaml:"customerGroup"`
		GroupsClaim     string `yaml:"groupsClaim"`
		SecretName      string `yaml:"secretName"`
		ClientSecretKey string `yaml:"clientSecretKey"`
		CookieKeyKey    string `yaml:"cookieKeyKey"`
	} `yaml:"access"`
	Data struct {
		Size          string `yaml:"size"`
		StorageClass  string `yaml:"storageClass"`
		ExistingClaim string `yaml:"existingClaim"`
	} `yaml:"data"`
	Service struct {
		Port int `yaml:"port"`
	} `yaml:"service"`
	Ingress struct {
		Enabled   bool   `yaml:"enabled"`
		ClassName string `yaml:"className"`
	} `yaml:"ingress"`
	Certificate struct {
		Enabled bool   `yaml:"enabled"`
		Name    string `yaml:"name"`
		Issuer  string `yaml:"issuer"`
	} `yaml:"certificate"`
	Resources struct {
		Requests struct {
			CPU    string `yaml:"cpu"`
			Memory string `yaml:"memory"`
		} `yaml:"requests"`
		Limits struct {
			CPU    string `yaml:"cpu"`
			Memory string `yaml:"memory"`
		} `yaml:"limits"`
	} `yaml:"resources"`
	CLI struct {
		Enabled bool     `yaml:"enabled"`
		Args    []string `yaml:"args"`
	} `yaml:"cli"`
}

func TestValuesDefaults(t *testing.T) {
	var v values
	decode(t, filepath.Join(chartDir(t), "values.yaml"), &v)

	equal(t, "image.repository", v.Image.Repository, "ghcr.io/zon/invoicer")
	equal(t, "image.tag", v.Image.Tag, "")
	equal(t, "image.pullPolicy", v.Image.PullPolicy, "IfNotPresent")
	if len(v.Image.PullSecrets) != 0 {
		t.Fatalf("image.pullSecrets: got %v, want empty", v.Image.PullSecrets)
	}

	equal(t, "hostname", v.Hostname, "invoices.haralovich.com")
	equal(t, "publicURL", v.PublicURL, "https://invoices.haralovich.com")

	equal(t, "config.vendor", v.Config.Vendor, "Jane Smith")
	equal(t, "config.customer", v.Config.Customer, "Acme Corp")
	equal(t, "config.project", v.Config.Project, "Website Redesign")
	equal(t, "config.rate", v.Config.Rate, 150)
	equal(t, "config.hours", v.Config.Hours, 40)
	equal(t, "config.database", v.Config.Database, "/data/invoices.db")

	equal(t, "access.issuer", v.Access.Issuer, "https://id.haralovich.org")
	equal(t, "access.clientId", v.Access.ClientID, "invoicer")
	equal(t, "access.adminGroup", v.Access.AdminGroup, "invoicer-admins")
	equal(t, "access.customerGroup", v.Access.CustomerGroup, "invoicer-customers")
	equal(t, "access.groupsClaim", v.Access.GroupsClaim, "groups")
	equal(t, "access.secretName", v.Access.SecretName, "invoicer-oidc")
	equal(t, "access.clientSecretKey", v.Access.ClientSecretKey, "client-secret")
	equal(t, "access.cookieKeyKey", v.Access.CookieKeyKey, "cookie-key")

	equal(t, "data.size", v.Data.Size, "1Gi")
	equal(t, "data.storageClass", v.Data.StorageClass, "reliable")
	equal(t, "data.existingClaim", v.Data.ExistingClaim, "")

	equal(t, "service.port", v.Service.Port, 80)

	equal(t, "ingress.enabled", v.Ingress.Enabled, true)
	equal(t, "ingress.className", v.Ingress.ClassName, "traefik")

	equal(t, "certificate.enabled", v.Certificate.Enabled, true)
	equal(t, "certificate.name", v.Certificate.Name, "invoicer")
	equal(t, "certificate.issuer", v.Certificate.Issuer, "lets-encrypt")

	equal(t, "resources.requests.cpu", v.Resources.Requests.CPU, "50m")
	equal(t, "resources.requests.memory", v.Resources.Requests.Memory, "64Mi")
	equal(t, "resources.limits.cpu", v.Resources.Limits.CPU, "500m")
	equal(t, "resources.limits.memory", v.Resources.Limits.Memory, "256Mi")

	equal(t, "cli.enabled", v.CLI.Enabled, false)
	if len(v.CLI.Args) != 0 {
		t.Fatalf("cli.args: got %v, want empty", v.CLI.Args)
	}
}

type renderedConfig struct {
	Vendor   string  `yaml:"vendor"`
	Customer string  `yaml:"customer"`
	Project  string  `yaml:"project"`
	Rate     float64 `yaml:"rate"`
	Hours    float64 `yaml:"hours"`
	Database string  `yaml:"database"`
	BaseURL  string  `yaml:"baseUrl"`
	OIDC     struct {
		Issuer           string `yaml:"issuer"`
		ClientID         string `yaml:"clientId"`
		ClientSecretFile string `yaml:"clientSecretFile"`
		CookieKeyFile    string `yaml:"cookieKeyFile"`
		AdminGroup       string `yaml:"adminGroup"`
		CustomerGroup    string `yaml:"customerGroup"`
		GroupsClaim      string `yaml:"groupsClaim"`
	} `yaml:"oidc"`
}

func renderChart(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed")
	}

	command := append([]string{"template", "invoicer", chartDir(t)}, args...)
	output, err := exec.Command(helm, command...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm %v: %v\n%s", command, err, output)
	}
	return parseManifests(t, string(output))
}

func parseManifests(t *testing.T, text string) []map[string]any {
	t.Helper()
	decoder := yaml.NewDecoder(strings.NewReader(text))
	var manifests []map[string]any
	for {
		var manifest map[string]any
		err := decoder.Decode(&manifest)
		if errors.Is(err, io.EOF) {
			return manifests
		}
		if err != nil {
			t.Fatalf("parsing rendered manifest: %v", err)
		}
		if len(manifest) > 0 {
			manifests = append(manifests, manifest)
		}
	}
}

func findKind(t *testing.T, manifests []map[string]any, kind string) map[string]any {
	t.Helper()
	for _, manifest := range manifests {
		if manifest["kind"] == kind {
			return manifest
		}
	}
	t.Fatalf("chart renders no %s", kind)
	return nil
}

func configFile(t *testing.T, manifests []map[string]any) string {
	t.Helper()
	configMap := findKind(t, manifests, "ConfigMap")
	data, ok := configMap["data"].(map[string]any)
	if !ok {
		t.Fatal("config map has no data")
	}
	if len(data) != 1 {
		t.Fatalf("config map holds %d files, want one", len(data))
	}
	text, ok := data["config.yaml"].(string)
	if !ok {
		t.Fatal("config map does not hold config.yaml")
	}
	return text
}

func decodeRenderedConfig(t *testing.T, text string) renderedConfig {
	t.Helper()
	var cfg renderedConfig
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil {
		t.Fatalf("parsing rendered config: %v", err)
	}
	return cfg
}

func TestConfigRendering(t *testing.T) {
	cfg := decodeRenderedConfig(t, configFile(t, renderChart(t)))

	equal(t, "vendor", cfg.Vendor, "Jane Smith")
	equal(t, "customer", cfg.Customer, "Acme Corp")
	equal(t, "project", cfg.Project, "Website Redesign")
	equal(t, "rate", cfg.Rate, 150)
	equal(t, "hours", cfg.Hours, 40)
	equal(t, "database", cfg.Database, "/data/invoices.db")
	equal(t, "baseUrl", cfg.BaseURL, "https://invoices.haralovich.com")

	equal(t, "oidc.issuer", cfg.OIDC.Issuer, "https://id.haralovich.org")
	equal(t, "oidc.clientId", cfg.OIDC.ClientID, "invoicer")
	equal(t, "oidc.clientSecretFile", cfg.OIDC.ClientSecretFile, "/run/secrets/oidc-client-secret")
	equal(t, "oidc.cookieKeyFile", cfg.OIDC.CookieKeyFile, "/run/secrets/cookie-key")
	equal(t, "oidc.adminGroup", cfg.OIDC.AdminGroup, "invoicer-admins")
	equal(t, "oidc.customerGroup", cfg.OIDC.CustomerGroup, "invoicer-customers")
	equal(t, "oidc.groupsClaim", cfg.OIDC.GroupsClaim, "groups")
}

func TestConfigRenderingUsesValues(t *testing.T) {
	manifests := renderChart(t,
		"--set", "config.vendor=Grace Hopper",
		"--set", "config.customer=Compilers Inc",
		"--set", "config.project=",
		"--set", "config.rate=200",
		"--set", "config.hours=32",
		"--set", "config.database=",
		"--set", "publicURL=https://billing.example.com",
		"--set", "access.issuer=https://login.example.com",
		"--set", "access.clientId=example",
		"--set", "access.adminGroup=billing-admins",
		"--set", "access.customerGroup=billing-customers",
		"--set", "access.groupsClaim=roles",
	)
	cfg := decodeRenderedConfig(t, configFile(t, manifests))

	equal(t, "vendor", cfg.Vendor, "Grace Hopper")
	equal(t, "customer", cfg.Customer, "Compilers Inc")
	equal(t, "project", cfg.Project, "")
	equal(t, "rate", cfg.Rate, 200)
	equal(t, "hours", cfg.Hours, 32)
	equal(t, "database", cfg.Database, "/data/invoices.db")
	equal(t, "baseUrl", cfg.BaseURL, "https://billing.example.com")
	equal(t, "oidc.issuer", cfg.OIDC.Issuer, "https://login.example.com")
	equal(t, "oidc.clientId", cfg.OIDC.ClientID, "example")
	equal(t, "oidc.adminGroup", cfg.OIDC.AdminGroup, "billing-admins")
	equal(t, "oidc.customerGroup", cfg.OIDC.CustomerGroup, "billing-customers")
	equal(t, "oidc.groupsClaim", cfg.OIDC.GroupsClaim, "roles")
}

func TestConfigMountIsReadOnly(t *testing.T) {
	const configPath = "/home/invoicer/.invoicer/config.yaml"
	manifests := renderChart(t)
	configMap := findKind(t, manifests, "ConfigMap")
	deployment := findKind(t, manifests, "Deployment")

	configName, _ := configMap["metadata"].(map[string]any)["name"].(string)
	if configName == "" {
		t.Fatal("config map has no name")
	}

	podSpec := deployment["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	if len(containers) != 1 {
		t.Fatalf("deployment runs %d containers, want one", len(containers))
	}
	mounts, _ := containers[0].(map[string]any)["volumeMounts"].([]any)

	var volumeName string
	for _, value := range mounts {
		mount := value.(map[string]any)
		if mount["mountPath"] != configPath {
			continue
		}
		if mount["readOnly"] != true {
			t.Fatal("config file is not mounted read-only")
		}
		volumeName, _ = mount["name"].(string)
	}
	if volumeName == "" {
		t.Fatalf("deployment mounts no file at %s", configPath)
	}

	volumes := podSpec["volumes"].([]any)
	for _, value := range volumes {
		volume := value.(map[string]any)
		if volume["name"] != volumeName {
			continue
		}
		source, _ := volume["configMap"].(map[string]any)
		if source["name"] != configName {
			t.Fatalf("volume %s reads config map %v, want %s", volumeName, source["name"], configName)
		}
		if source["defaultMode"] != 0o444 {
			t.Fatalf("config file mode: got %v, want 0444", source["defaultMode"])
		}
		return
	}
	t.Fatalf("deployment has no volume named %s", volumeName)
}

func TestHelmLintAndTemplate(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed")
	}

	for _, args := range [][]string{{"lint"}, {"template"}} {
		args = append(args, chartDir(t))
		cmd := exec.Command(helm, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("helm %v: %v\n%s", args, err, output)
		}
	}
}

func equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
}
