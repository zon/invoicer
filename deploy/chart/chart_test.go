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
	Schedule struct {
		Enabled bool   `yaml:"enabled"`
		Cron    string `yaml:"cron"`
	} `yaml:"schedule"`
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

	equal(t, "schedule.enabled", v.Schedule.Enabled, true)
	equal(t, "schedule.cron", v.Schedule.Cron, "0 9 1 * *")

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

func deploymentPodSpec(t *testing.T, manifests []map[string]any) map[string]any {
	t.Helper()
	deployment := findKind(t, manifests, "Deployment")
	return deployment["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
}

func webContainer(t *testing.T, manifests []map[string]any) map[string]any {
	t.Helper()
	containers, _ := deploymentPodSpec(t, manifests)["containers"].([]any)
	if len(containers) != 1 {
		t.Fatalf("deployment runs %d containers, want one", len(containers))
	}
	return containers[0].(map[string]any)
}

func TestConfigMountIsReadOnly(t *testing.T) {
	const configPath = "/home/invoicer/.invoicer/config.yaml"
	manifests := renderChart(t)
	configMap := findKind(t, manifests, "ConfigMap")

	configName, _ := configMap["metadata"].(map[string]any)["name"].(string)
	if configName == "" {
		t.Fatal("config map has no name")
	}

	podSpec := deploymentPodSpec(t, manifests)
	mounts, _ := webContainer(t, manifests)["volumeMounts"].([]any)

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

type secretFile struct {
	mountPath string
	itemPath  string
	key       string
}

func TestSecretMounts(t *testing.T) {
	assertSecretMounts(t, renderChart(t), "invoicer-oidc", "client-secret", "cookie-key")
}

func TestSecretMountsUseValues(t *testing.T) {
	manifests := renderChart(t,
		"--set", "access.secretName=example-oidc",
		"--set", "access.clientSecretKey=oidc-secret",
		"--set", "access.cookieKeyKey=session-key",
	)
	assertSecretMounts(t, manifests, "example-oidc", "oidc-secret", "session-key")
}

func assertSecretMounts(t *testing.T, manifests []map[string]any, secretName, clientSecretKey, cookieKey string) {
	t.Helper()
	podSpec := deploymentPodSpec(t, manifests)
	container := webContainer(t, manifests)

	assertSecretFile(t, podSpec, container, secretName, secretFile{
		mountPath: "/run/secrets/oidc-client-secret",
		itemPath:  "oidc-client-secret",
		key:       clientSecretKey,
	})
	assertSecretFile(t, podSpec, container, secretName, secretFile{
		mountPath: "/run/secrets/cookie-key",
		itemPath:  "cookie-key",
		key:       cookieKey,
	})
}

func assertSecretFile(t *testing.T, podSpec, container map[string]any, secretName string, file secretFile) {
	t.Helper()
	mounts, _ := container["volumeMounts"].([]any)

	var volumeName string
	for _, value := range mounts {
		mount := value.(map[string]any)
		if mount["mountPath"] != file.mountPath {
			continue
		}
		if mount["readOnly"] != true {
			t.Fatalf("%s is not mounted read-only", file.mountPath)
		}
		volumeName, _ = mount["name"].(string)
	}
	if volumeName == "" {
		t.Fatalf("deployment mounts no secret at %s", file.mountPath)
	}

	volumes, _ := podSpec["volumes"].([]any)
	for _, value := range volumes {
		volume := value.(map[string]any)
		if volume["name"] != volumeName {
			continue
		}
		secret, _ := volume["secret"].(map[string]any)
		if secret["secretName"] != secretName {
			t.Fatalf("volume %s reads secret %v, want %s", volumeName, secret["secretName"], secretName)
		}
		items, _ := secret["items"].([]any)
		for _, value := range items {
			item := value.(map[string]any)
			if item["key"] == file.key && item["path"] == file.itemPath {
				return
			}
		}
		t.Fatalf("volume %s does not mount key %s at %s", volumeName, file.key, file.itemPath)
	}
	t.Fatalf("deployment has no volume named %s", volumeName)
}

func TestChartRendersNoSecretValues(t *testing.T) {
	const value = "secret-value-that-must-not-render"
	manifests := renderChart(t,
		"--set", "access.clientSecret="+value,
		"--set", "access.cookieKey="+value,
	)

	for _, manifest := range manifests {
		if manifest["kind"] == "Secret" {
			t.Fatal("chart renders a Secret instead of mounting the existing one")
		}
		rendered, err := yaml.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(rendered), value) {
			t.Fatalf("%s renders a secret value", manifest["kind"])
		}
	}

	cfg := decodeRenderedConfig(t, configFile(t, manifests))
	equal(t, "oidc.clientSecretFile", cfg.OIDC.ClientSecretFile, "/run/secrets/oidc-client-secret")
	equal(t, "oidc.cookieKeyFile", cfg.OIDC.CookieKeyFile, "/run/secrets/cookie-key")
}

func pvcName(t *testing.T, manifests []map[string]any) string {
	t.Helper()
	claim := findKind(t, manifests, "PersistentVolumeClaim")
	name, _ := claim["metadata"].(map[string]any)["name"].(string)
	if name == "" {
		t.Fatal("persistent volume claim has no name")
	}
	return name
}

func assertDataMount(t *testing.T, manifests []map[string]any, claimName string) {
	t.Helper()
	podSpec := deploymentPodSpec(t, manifests)
	mounts, _ := webContainer(t, manifests)["volumeMounts"].([]any)

	var volumeName string
	for _, value := range mounts {
		mount := value.(map[string]any)
		if mount["mountPath"] != "/data" {
			continue
		}
		if mount["readOnly"] == true {
			t.Fatal("data volume is mounted read-only")
		}
		volumeName, _ = mount["name"].(string)
	}
	if volumeName == "" {
		t.Fatal("deployment mounts no volume at /data")
	}

	volumes, _ := podSpec["volumes"].([]any)
	for _, value := range volumes {
		volume := value.(map[string]any)
		if volume["name"] != volumeName {
			continue
		}
		source, _ := volume["persistentVolumeClaim"].(map[string]any)
		if source["claimName"] != claimName {
			t.Fatalf("volume %s reads claim %v, want %s", volumeName, source["claimName"], claimName)
		}
		return
	}
	t.Fatalf("deployment has no volume named %s", volumeName)
}

func TestPersistentVolumeClaim(t *testing.T) {
	manifests := renderChart(t)
	claim := findKind(t, manifests, "PersistentVolumeClaim")
	spec := claim["spec"].(map[string]any)

	accessModes, _ := spec["accessModes"].([]any)
	if len(accessModes) != 1 || accessModes[0] != "ReadWriteOnce" {
		t.Fatalf("access modes: got %v, want [ReadWriteOnce]", accessModes)
	}
	equal(t, "storageClassName", spec["storageClassName"], "reliable")

	requests, _ := spec["resources"].(map[string]any)["requests"].(map[string]any)
	equal(t, "storage", requests["storage"], "1Gi")

	assertDataMount(t, manifests, pvcName(t, manifests))
}

func TestPersistentVolumeClaimUsesValues(t *testing.T) {
	manifests := renderChart(t,
		"--set", "data.size=5Gi",
		"--set", "data.storageClass=fast",
	)
	claim := findKind(t, manifests, "PersistentVolumeClaim")
	spec := claim["spec"].(map[string]any)

	equal(t, "storageClassName", spec["storageClassName"], "fast")
	requests, _ := spec["resources"].(map[string]any)["requests"].(map[string]any)
	equal(t, "storage", requests["storage"], "5Gi")

	assertDataMount(t, manifests, pvcName(t, manifests))
}

func TestPersistentVolumeClaimReusesExistingClaim(t *testing.T) {
	manifests := renderChart(t, "--set", "data.existingClaim=shared-data")

	for _, manifest := range manifests {
		if manifest["kind"] == "PersistentVolumeClaim" {
			t.Fatal("chart renders a claim instead of reusing the existing one")
		}
	}

	assertDataMount(t, manifests, "shared-data")
}

func TestDeploymentRecreateStrategy(t *testing.T) {
	manifests := renderChart(t)
	deployment := findKind(t, manifests, "Deployment")
	spec := deployment["spec"].(map[string]any)

	equal(t, "replicas", spec["replicas"], 1)
	strategy, _ := spec["strategy"].(map[string]any)
	equal(t, "strategy.type", strategy["type"], "Recreate")
}

func TestWebContainerPortAndProbes(t *testing.T) {
	container := webContainer(t, renderChart(t))

	ports, _ := container["ports"].([]any)
	if len(ports) != 1 {
		t.Fatalf("container exposes %d ports, want one", len(ports))
	}
	assertPort(t, "containerPort", ports[0].(map[string]any), "containerPort", 8080)

	for _, name := range []string{"readinessProbe", "livenessProbe"} {
		probe, ok := container[name].(map[string]any)
		if !ok {
			t.Fatalf("container has no %s", name)
		}
		socket, ok := probe["tcpSocket"].(map[string]any)
		if !ok {
			t.Fatalf("%s is not a TCP probe", name)
		}
		assertPort(t, name, socket, "port", 8080)
	}
}

func assertPort(t *testing.T, name string, source map[string]any, key string, want int) {
	t.Helper()
	port, ok := source[key].(int)
	if !ok || port != want {
		t.Fatalf("%s: got %v, want %d", name, source[key], want)
	}
}

func TestWebPodGracePeriod(t *testing.T) {
	podSpec := deploymentPodSpec(t, renderChart(t))
	grace, ok := podSpec["terminationGracePeriodSeconds"].(int)
	if !ok {
		t.Fatal("web pod sets no termination grace period")
	}
	if grace <= 10 {
		t.Fatalf("termination grace period %ds does not cover the 10s shutdown timeout", grace)
	}
}

func TestWebPodSecurityContext(t *testing.T) {
	manifests := renderChart(t)
	podSpec := deploymentPodSpec(t, manifests)

	security, _ := podSpec["securityContext"].(map[string]any)
	equal(t, "runAsUser", security["runAsUser"], 1000)
	equal(t, "runAsGroup", security["runAsGroup"], 1000)
	equal(t, "fsGroup", security["fsGroup"], 1000)

	containerSecurity, _ := webContainer(t, manifests)["securityContext"].(map[string]any)
	equal(t, "readOnlyRootFilesystem", containerSecurity["readOnlyRootFilesystem"], true)
}

func appVersion(t *testing.T) string {
	t.Helper()
	var chart chartMetadata
	decode(t, filepath.Join(chartDir(t), "Chart.yaml"), &chart)
	return chart.AppVersion
}

func containerImage(t *testing.T, container map[string]any) string {
	t.Helper()
	image, ok := container["image"].(string)
	if !ok {
		t.Fatalf("container has no image")
	}
	return image
}

func TestImageDefaultsToChartAppVersion(t *testing.T) {
	manifests := renderChart(t)
	want := "ghcr.io/zon/invoicer:" + appVersion(t)

	equal(t, "web image", containerImage(t, webContainer(t, manifests)), want)
}

func TestImageUsesTagValue(t *testing.T) {
	manifests := renderChart(t, "--set", "image.tag=v1.2.3")

	equal(t, "web image", containerImage(t, webContainer(t, manifests)), "ghcr.io/zon/invoicer:v1.2.3")
}

func pullSecretNames(t *testing.T, podSpec map[string]any) []any {
	t.Helper()
	secrets, _ := podSpec["imagePullSecrets"].([]any)

	var names []any
	for _, value := range secrets {
		name, _ := value.(map[string]any)["name"]
		names = append(names, name)
	}
	return names
}

func TestImagePullSecrets(t *testing.T) {
	manifests := renderChart(t,
		"--set", "image.pullSecrets={registry-a,registry-b}",
	)
	want := []any{"registry-a", "registry-b"}

	equalsAny(t, "web imagePullSecrets", pullSecretNames(t, deploymentPodSpec(t, manifests)), want)
}

func TestNoImagePullSecretsByDefault(t *testing.T) {
	manifests := renderChart(t)

	equalsAny(t, "web imagePullSecrets", pullSecretNames(t, deploymentPodSpec(t, manifests)), nil)
}

func TestDeploymentRestartsWhenConfigChanges(t *testing.T) {
	annotations := func(manifests []map[string]any) map[string]any {
		t.Helper()
		deployment := findKind(t, manifests, "Deployment")
		template := deployment["spec"].(map[string]any)["template"].(map[string]any)
		result, _ := template["metadata"].(map[string]any)["annotations"].(map[string]any)
		return result
	}

	before := annotations(renderChart(t))
	checksum, _ := before["checksum/config"].(string)
	if checksum == "" {
		t.Fatal("pod template has no config checksum annotation")
	}

	after := annotations(renderChart(t, "--set", "config.vendor=Grace Hopper"))
	if after["checksum/config"] == checksum {
		t.Fatal("config checksum does not change with the config")
	}
}

func assertService(t *testing.T, manifests []map[string]any, port int) {
	t.Helper()
	service := findKind(t, manifests, "Service")
	spec := service["spec"].(map[string]any)

	equal(t, "service type", spec["type"], "ClusterIP")

	ports, _ := spec["ports"].([]any)
	if len(ports) != 1 {
		t.Fatalf("service exposes %d ports, want one", len(ports))
	}
	servicePort := ports[0].(map[string]any)
	assertPort(t, "service port", servicePort, "port", port)
	assertPort(t, "service targetPort", servicePort, "targetPort", 8080)

	selector, _ := spec["selector"].(map[string]any)
	deployment := findKind(t, manifests, "Deployment")
	labels, _ := deployment["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["labels"].(map[string]any)
	for key, value := range selector {
		if labels[key] != value {
			t.Fatalf("service selector %s: got %v, want %v", key, value, labels[key])
		}
	}
	if len(selector) == 0 {
		t.Fatal("service has no selector")
	}
}

func TestServiceForwardsToWebPort(t *testing.T) {
	assertService(t, renderChart(t), 80)
}

func TestServiceUsesValues(t *testing.T) {
	assertService(t, renderChart(t, "--set", "service.port=8088"), 8088)
}

func ingressTLS(t *testing.T, ingress map[string]any) map[string]any {
	t.Helper()
	spec := ingress["spec"].(map[string]any)
	tls, _ := spec["tls"].([]any)
	if len(tls) != 1 {
		t.Fatalf("ingress has %d TLS entries, want one", len(tls))
	}
	return tls[0].(map[string]any)
}

func findManifest(t *testing.T, manifests []map[string]any, apiVersion, kind string) map[string]any {
	t.Helper()
	for _, manifest := range manifests {
		if manifest["apiVersion"] == apiVersion && manifest["kind"] == kind {
			return manifest
		}
	}
	t.Fatalf("chart renders no %s %s", apiVersion, kind)
	return nil
}

func TestIngressRoutesHostnameToService(t *testing.T) {
	manifests := renderChart(t)
	ingress := findManifest(t, manifests, "networking.k8s.io/v1", "Ingress")
	spec := ingress["spec"].(map[string]any)

	equal(t, "ingressClassName", spec["ingressClassName"], "traefik")

	rules, _ := spec["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("ingress has %d rules, want one", len(rules))
	}
	rule := rules[0].(map[string]any)
	equal(t, "rule host", rule["host"], "invoices.haralovich.com")

	paths, _ := rule["http"].(map[string]any)["paths"].([]any)
	if len(paths) != 1 {
		t.Fatalf("ingress has %d paths, want one", len(paths))
	}
	path := paths[0].(map[string]any)
	equal(t, "path", path["path"], "/")
	equal(t, "pathType", path["pathType"], "Prefix")

	backend := path["backend"].(map[string]any)["service"].(map[string]any)
	serviceName, _ := findKind(t, manifests, "Service")["metadata"].(map[string]any)["name"].(string)
	backendName, _ := backend["name"].(string)
	equal(t, "backend service name", backendName, serviceName)

	port, _ := backend["port"].(map[string]any)
	assertPort(t, "backend service port", port, "number", 80)

	tls := ingressTLS(t, ingress)
	equalsAny(t, "tls hosts", tls["hosts"], []any{"invoices.haralovich.com"})
	equal(t, "tls secretName", tls["secretName"], "invoicer-tls")
}

func TestIngressUsesValues(t *testing.T) {
	manifests := renderChart(t,
		"--set", "hostname=billing.example.com",
		"--set", "ingress.className=nginx",
		"--set", "service.port=8088",
	)
	ingress := findManifest(t, manifests, "networking.k8s.io/v1", "Ingress")
	spec := ingress["spec"].(map[string]any)

	equal(t, "ingressClassName", spec["ingressClassName"], "nginx")
	rule := spec["rules"].([]any)[0].(map[string]any)
	equal(t, "rule host", rule["host"], "billing.example.com")
	path := rule["http"].(map[string]any)["paths"].([]any)[0].(map[string]any)
	assertPort(t, "backend service port", path["backend"].(map[string]any)["service"].(map[string]any)["port"].(map[string]any), "number", 8088)

	tls := ingressTLS(t, ingress)
	equalsAny(t, "tls hosts", tls["hosts"], []any{"billing.example.com"})
}

func TestIngressDisabled(t *testing.T) {
	manifests := renderChart(t, "--set", "ingress.enabled=false")
	for _, manifest := range manifests {
		if manifest["kind"] == "Ingress" {
			t.Fatal("chart renders an Ingress when disabled")
		}
	}
}

func TestCertificate(t *testing.T) {
	certificate := findManifest(t, renderChart(t), "cert-manager.io/v1", "Certificate")
	spec := certificate["spec"].(map[string]any)

	equal(t, "name", certificate["metadata"].(map[string]any)["name"], "invoicer")
	equal(t, "secretName", spec["secretName"], "invoicer-tls")
	equalsAny(t, "dnsNames", spec["dnsNames"], []any{"invoices.haralovich.com"})

	issuer := spec["issuerRef"].(map[string]any)
	equal(t, "issuerRef.name", issuer["name"], "lets-encrypt")
	equal(t, "issuerRef.kind", issuer["kind"], "ClusterIssuer")
}

func TestCertificateUsesValues(t *testing.T) {
	certificate := findManifest(t, renderChart(t,
		"--set", "certificate.name=billing",
		"--set", "certificate.issuer=staging",
		"--set", "hostname=billing.example.com",
	), "cert-manager.io/v1", "Certificate")
	spec := certificate["spec"].(map[string]any)

	equal(t, "name", certificate["metadata"].(map[string]any)["name"], "billing")
	equal(t, "secretName", spec["secretName"], "billing-tls")
	equalsAny(t, "dnsNames", spec["dnsNames"], []any{"billing.example.com"})
	equal(t, "issuerRef.name", spec["issuerRef"].(map[string]any)["name"], "staging")
}

func cronWorkflow(t *testing.T, manifests []map[string]any) map[string]any {
	t.Helper()
	return findManifest(t, manifests, "argoproj.io/v1alpha1", "CronWorkflow")
}

func cronWorkflowSpec(t *testing.T, manifests []map[string]any) map[string]any {
	t.Helper()
	spec, ok := cronWorkflow(t, manifests)["spec"].(map[string]any)
	if !ok {
		t.Fatal("CronWorkflow has no spec")
	}
	return spec
}

func TestScheduleRendersOneCronWorkflow(t *testing.T) {
	manifests := renderChart(t)

	count := 0
	for _, manifest := range manifests {
		if manifest["kind"] == "CronWorkflow" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("chart renders %d CronWorkflows, want one", count)
	}

	workflow := cronWorkflow(t, manifests)
	metadata := workflow["metadata"].(map[string]any)
	equal(t, "namespace", metadata["namespace"], "invoicer")
	if name, _ := metadata["name"].(string); name == "" {
		t.Fatal("CronWorkflow has no name")
	}
}

func TestScheduleUsesDefaultCron(t *testing.T) {
	spec := cronWorkflowSpec(t, renderChart(t))

	equal(t, "schedule", spec["schedule"], "0 9 1 * *")
	equal(t, "timezone", spec["timezone"], "UTC")
	equal(t, "concurrencyPolicy", spec["concurrencyPolicy"], "Forbid")
}

func TestScheduleUsesCronValue(t *testing.T) {
	spec := cronWorkflowSpec(t, renderChart(t, "--set", "schedule.cron=30 6 2 * *"))

	equal(t, "schedule", spec["schedule"], "30 6 2 * *")
	equal(t, "timezone", spec["timezone"], "UTC")
}

func TestScheduleDisabledRendersNoCronWorkflow(t *testing.T) {
	for _, manifest := range renderChart(t, "--set", "schedule.enabled=false") {
		if manifest["kind"] == "CronWorkflow" {
			t.Fatal("chart renders a CronWorkflow when disabled")
		}
	}
}

func TestCertificateDisabled(t *testing.T) {
	manifests := renderChart(t, "--set", "certificate.enabled=false")

	for _, manifest := range manifests {
		if manifest["kind"] == "Certificate" {
			t.Fatal("chart renders a Certificate when disabled")
		}
	}

	ingress := findManifest(t, manifests, "networking.k8s.io/v1", "Ingress")
	spec := ingress["spec"].(map[string]any)
	if tls, ok := spec["tls"]; ok && tls != nil {
		t.Fatalf("ingress has TLS configuration: %v", tls)
	}
}

func equalsAny(t *testing.T, name string, got any, want []any) {
	t.Helper()
	values, ok := got.([]any)
	if !ok || len(values) != len(want) {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
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
