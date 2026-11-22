package chart_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
