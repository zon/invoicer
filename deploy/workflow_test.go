package deploy_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "..")
}

type workflow struct {
	Env         map[string]string      `yaml:"env"`
	On          map[string]yaml.Node   `yaml:"on"`
	Permissions map[string]string      `yaml:"permissions"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	If    string            `yaml:"if"`
	Needs []string          `yaml:"needs"`
	Env   map[string]string `yaml:"env"`
	Steps []workflowStep    `yaml:"steps"`
}

type workflowStep struct {
	Uses            string            `yaml:"uses"`
	Run             string            `yaml:"run"`
	If              string            `yaml:"if"`
	Env             map[string]string `yaml:"env"`
	ContinueOnError bool              `yaml:"continue-on-error"`
}

type pushTrigger struct {
	Branches []string `yaml:"branches"`
	Tags     []string `yaml:"tags"`
}

func loadWorkflow(t *testing.T, name string) workflow {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".github", "workflows", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var parsed workflow
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed
}

func containerWorkflow(t *testing.T) workflow {
	t.Helper()
	return loadWorkflow(t, "build-container.yaml")
}

func chartWorkflow(t *testing.T) workflow {
	t.Helper()
	return loadWorkflow(t, "build-chart.yaml")
}

func TestBuildWorkflowsRunOnMainPush(t *testing.T) {
	for _, name := range []string{"build-container.yaml", "build-chart.yaml"} {
		workflow := loadWorkflow(t, name)
		push := pushTriggerOf(t, workflow)

		if !slices.Contains(push.Branches, "main") {
			t.Fatalf("%s push branches: got %v, want main", name, push.Branches)
		}
		if len(push.Tags) != 0 {
			t.Fatalf("%s push tags: got %v, want none", name, push.Tags)
		}
		if _, ok := workflow.On["pull_request"]; ok {
			t.Fatalf("%s runs on pull requests, want only main pushes", name)
		}
		if _, ok := workflow.On["workflow_call"]; ok {
			t.Fatalf("%s is callable, want a standalone workflow", name)
		}
	}
}

func TestNoReleaseWorkflow(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "workflows", "release.yaml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("release workflow still exists at %s", path)
	}
}

func TestBuildWorkflowsBuildSeparateArtifacts(t *testing.T) {
	container := containerWorkflow(t)
	chart := chartWorkflow(t)

	if workflowHasStep(container, lintsChart) || workflowHasStep(container, publishesChart) {
		t.Fatal("container workflow builds the chart, want a separate chart workflow")
	}
	if workflowHasStep(chart, buildsImage) || workflowHasStep(chart, isImagePublishing) {
		t.Fatal("chart workflow builds the image, want a separate container workflow")
	}
}

func TestContainerWorkflowChecksTheImageVersion(t *testing.T) {
	workflow := containerWorkflow(t)
	check := jobNamed(t, workflow, "check")
	build := jobNamed(t, workflow, "build")

	version := stepIndexMatching(check.Steps, readsAppVersion)
	inspect := stepIndexMatching(check.Steps, inspectsImage)
	if version < 0 {
		t.Fatalf("check job does not read VERSION:\n%s", renderSteps(check.Steps))
	}
	if inspect < 0 || inspect < version {
		t.Fatalf("check job does not inspect the image after reading VERSION:\n%s", renderSteps(check.Steps))
	}
	if login := stepIndexMatching(check.Steps, logsInToRegistry); login < 0 || login > inspect {
		t.Fatalf("check job inspects the image before logging in:\n%s", renderSteps(check.Steps))
	}

	run := check.Steps[inspect].Run
	if !referencesImage(run) || !strings.Contains(run, "steps.version.outputs.version") {
		t.Fatalf("image check does not use the app version and IMAGE:\n%s", run)
	}
	if !slices.Contains(build.Needs, "check") {
		t.Fatalf("build job needs %v, want check", build.Needs)
	}
	if !strings.Contains(build.If, "needs.check.outputs.built") || !strings.Contains(build.If, "false") {
		t.Fatalf("build job condition %q does not skip a built image", build.If)
	}
}

func TestChartWorkflowChecksTheChartVersion(t *testing.T) {
	workflow := chartWorkflow(t)
	check := jobNamed(t, workflow, "check")
	build := jobNamed(t, workflow, "build")

	version := stepIndexMatching(check.Steps, readsChartVersion)
	inspect := stepIndexMatching(check.Steps, inspectsChart)
	if version < 0 {
		t.Fatalf("check job does not read the chart version:\n%s", renderSteps(check.Steps))
	}
	if inspect < 0 || inspect < version {
		t.Fatalf("check job does not inspect the chart after reading its version:\n%s", renderSteps(check.Steps))
	}
	if login := stepIndexMatching(check.Steps, logsInToChartRegistry); login < 0 || login > inspect {
		t.Fatalf("check job inspects the chart before logging in:\n%s", renderSteps(check.Steps))
	}

	run := check.Steps[inspect].Run
	if !strings.Contains(run, "steps.chart_version.outputs.version") {
		t.Fatalf("chart check does not use the chart version:\n%s", run)
	}
	if !strings.Contains(run, "oci://ghcr.io/zon/charts/invoicer") {
		t.Fatalf("chart check does not use the chart registry:\n%s", run)
	}
	if !slices.Contains(build.Needs, "check") {
		t.Fatalf("build job needs %v, want check", build.Needs)
	}
	if !strings.Contains(build.If, "needs.check.outputs.built") || !strings.Contains(build.If, "false") {
		t.Fatalf("build job condition %q does not skip a built chart", build.If)
	}
}

func TestBuildWorkflowsRunTestsBeforePublishing(t *testing.T) {
	for _, check := range []struct {
		name     string
		workflow workflow
	}{
		{name: "container", workflow: containerWorkflow(t)},
		{name: "chart", workflow: chartWorkflow(t)},
	} {
		build := jobNamed(t, check.workflow, "build")
		testIndex := stepIndexMatching(build.Steps, runsTests)
		if testIndex < 0 {
			t.Fatalf("%s build does not run go test ./...:\n%s", check.name, renderSteps(build.Steps))
		}

		for index, step := range build.Steps {
			if !isPublishing(step) {
				continue
			}
			if index < testIndex {
				t.Fatalf("%s build publishes before running tests", check.name)
			}
			if runsAfterFailure(step.If) || runsAfterFailure(build.If) {
				t.Fatalf("%s build can publish after a failed test", check.name)
			}
		}
	}
}

func TestContainerWorkflowBuildsImageWithPodman(t *testing.T) {
	workflow := containerWorkflow(t)
	build := jobNamed(t, workflow, "build")
	index := stepIndexMatching(build.Steps, buildsImage)
	if index < 0 {
		t.Fatalf("container build does not build the image with Podman from deploy/container/Containerfile:\n%s", renderSteps(build.Steps))
	}
	if build.Steps[index].If != "" {
		t.Fatalf("image build step is conditional: %q", build.Steps[index].If)
	}

	if image := workflow.Env["IMAGE"]; image == "" {
		t.Fatal("workflow does not name the image in IMAGE")
	}

	tag := builtImageTag(build.Steps[index].Run)
	if tag == "" {
		t.Fatalf("build step tags no image:\n%s", build.Steps[index].Run)
	}
	if strings.HasSuffix(tag, ":latest") {
		t.Fatalf("build step tags the image latest: %s", tag)
	}
	if !referencesImage(build.Steps[index].Run) {
		t.Fatalf("build step does not use the workflow IMAGE:\n%s", build.Steps[index].Run)
	}
}

func TestContainerWorkflowScansTheBuiltImage(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	build := stepIndexMatching(steps, buildsImage)
	save := stepIndexMatching(steps, savesImage)
	scan := stepIndexMatching(steps, scansImage)
	if save < 0 {
		t.Fatalf("container build does not save the built image:\n%s", renderSteps(steps))
	}
	if scan < 0 {
		t.Fatalf("container build does not scan the image with Trivy:\n%s", renderSteps(steps))
	}
	if build > save || save > scan {
		t.Fatalf("container build builds at %d, saves at %d, scans at %d, want build then save then scan", build, save, scan)
	}

	tag := builtImageTag(steps[build].Run)
	if !strings.Contains(steps[save].Run, tag) {
		t.Fatalf("save step does not save the built image %s:\n%s", tag, steps[save].Run)
	}

	archive := savedArchive(steps[save].Run)
	if archive == "" {
		t.Fatalf("save step names no archive:\n%s", steps[save].Run)
	}
	if input := flagValue(steps[scan].Run, "--input"); input != archive {
		t.Fatalf("scan reads %q but the save step wrote %q", input, archive)
	}
}

func TestContainerWorkflowPinsTrivyVersion(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	install := stepIndexMatching(steps, installsTrivy)
	if install < 0 {
		t.Fatalf("container build does not install Trivy:\n%s", renderSteps(steps))
	}
	scan := stepIndexMatching(steps, scansImage)
	if scan < 0 {
		t.Fatalf("container build does not scan the image with Trivy:\n%s", renderSteps(steps))
	}
	if install > scan {
		t.Fatalf("container build scans at %d before it installs Trivy at %d", scan, install)
	}

	release := regexp.MustCompile(`trivy/releases/download/(v\d+\.\d+\.\d+)/trivy_(\d+\.\d+\.\d+)_Linux-64bit\.tar\.gz`)
	match := release.FindStringSubmatch(steps[install].Run)
	if match == nil {
		t.Fatalf("Trivy install does not pin a release version:\n%s", steps[install].Run)
	}
	if match[1] != "v"+match[2] {
		t.Fatalf("Trivy install downloads %s but extracts %s", match[1], match[2])
	}
}

func TestContainerWorkflowStopsOnHighOrCritical(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	scan := stepIndexMatching(steps, scansImage)
	if scan < 0 {
		t.Fatal("container build does not scan the image with Trivy")
	}
	run := steps[scan].Run
	if !strings.Contains(run, "--severity") || !strings.Contains(run, "HIGH") || !strings.Contains(run, "CRITICAL") {
		t.Fatalf("scan step does not limit the scan to high and critical vulnerabilities:\n%s", run)
	}
	if !strings.Contains(run, "--exit-code 1") {
		t.Fatalf("scan step does not fail on a found vulnerability:\n%s", run)
	}
	if steps[scan].ContinueOnError {
		t.Fatal("scan step ignores a found vulnerability, so the job continues")
	}
}

func TestContainerWorkflowScansBeforePublishing(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	scanIndex := stepIndexMatching(steps, scansImage)
	for index, step := range steps {
		if !isImagePublishing(step) {
			continue
		}
		if scanIndex < 0 {
			t.Fatal("container build publishes an image without scanning it")
		}
		if index < scanIndex {
			t.Fatal("container build publishes an image before scanning it")
		}
		if runsAfterFailure(step.If) {
			t.Fatal("container build can publish an image after a failed scan")
		}
	}
}

func TestContainerWorkflowSetsTheAppVersion(t *testing.T) {
	build := jobNamed(t, containerWorkflow(t), "build")

	version := build.Env["APP_VERSION"]
	if !strings.Contains(version, "needs.check.outputs.version") {
		t.Fatalf("build job APP_VERSION %q does not come from the image check", version)
	}

	for _, match := range []func(workflowStep) bool{buildsImage, isReleaseImagePublishStep, tagsRelease} {
		index := stepIndexMatching(build.Steps, match)
		if index < 0 {
			t.Fatalf("container build does not use the app version:\n%s", renderSteps(build.Steps))
		}
		if !usesAppVersion(build.Steps[index].Run) {
			t.Fatalf("step does not use the app version:\n%s", build.Steps[index].Run)
		}
	}
}

func TestContainerWorkflowPublishesTheReleaseImage(t *testing.T) {
	workflow := containerWorkflow(t)
	steps := jobNamed(t, workflow, "build").Steps

	publish := stepIndexMatching(steps, isReleaseImagePublishStep)
	if publish < 0 {
		t.Fatalf("container build does not publish the release image:\n%s", renderSteps(steps))
	}
	step := steps[publish]
	if step.If != "" {
		t.Fatalf("release image publish is conditional: %q", step.If)
	}
	if !referencesImage(step.Run) {
		t.Fatalf("release image publish does not use the workflow IMAGE:\n%s", step.Run)
	}
	if !usesAppVersion(step.Run) {
		t.Fatalf("release image publish does not use the app version:\n%s", step.Run)
	}
	if !strings.Contains(step.Run, "GITHUB_SHA") {
		t.Fatalf("release image publish does not use the commit:\n%s", step.Run)
	}

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("container build does not build the image")
	}
	if tag := builtImageTag(steps[build].Run); !strings.Contains(step.Run, tag) {
		t.Fatalf("release image publish does not use the built image %s:\n%s", tag, step.Run)
	}
}

func TestContainerWorkflowTagsTheRelease(t *testing.T) {
	workflow := containerWorkflow(t)
	steps := jobNamed(t, workflow, "build").Steps

	tag := stepIndexMatching(steps, tagsRelease)
	if tag < 0 {
		t.Fatalf("container build does not tag the release:\n%s", renderSteps(steps))
	}
	step := steps[tag]
	if !usesAppVersion(step.Run) {
		t.Fatalf("release tag does not use the app version:\n%s", step.Run)
	}
	if !strings.Contains(step.Run, "git push origin") {
		t.Fatalf("release tag does not push the tag:\n%s", step.Run)
	}
	if !strings.Contains(step.Run, "git ls-remote") {
		t.Fatalf("release tag does not check for an existing tag:\n%s", step.Run)
	}

	publish := stepIndexMatching(steps, isReleaseImagePublishStep)
	if publish < 0 || tag < publish {
		t.Fatalf("container build tags the release at %d before it publishes the image at %d", tag, publish)
	}
	if got := workflow.Permissions["contents"]; got != "write" {
		t.Fatalf("container workflow contents permission: got %q, want write", got)
	}
}

func TestContainerWorkflowLogsInForPublish(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	login := stepIndexMatching(steps, logsInToRegistry)
	publish := stepIndexMatching(steps, isReleaseImagePublishStep)
	if login < 0 || publish < 0 || login > publish {
		t.Fatalf("container build logs in to the registry at %d but publishes the release at %d", login, publish)
	}
	if steps[login].If != "" {
		t.Fatalf("registry login is conditional: %q", steps[login].If)
	}
}

func TestContainerWorkflowStampsAppVersionIntoImage(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("container build does not build the image")
	}
	version := imageBuildArg(steps[build].Run, "VERSION")
	if version == "" {
		t.Fatalf("image build does not stamp the app version:\n%s", steps[build].Run)
	}
	if !usesAppVersion(version) {
		t.Fatalf("image build VERSION %q does not come from APP_VERSION", version)
	}

	publish := stepIndexMatching(steps, isReleaseImagePublishStep)
	if publish < 0 {
		t.Fatalf("container build does not publish the release image:\n%s", renderSteps(steps))
	}
	if !usesAppVersion(steps[publish].Run) {
		t.Fatalf("release image publish does not use the same app version:\n%s", steps[publish].Run)
	}
}

func TestContainerWorkflowLabelsTheImage(t *testing.T) {
	steps := jobNamed(t, containerWorkflow(t), "build").Steps

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("container build does not build the image")
	}
	labels := imageLabels(steps[build].Run)

	for _, check := range []struct {
		key  string
		uses []string
	}{
		{key: "org.opencontainers.image.version", uses: []string{"APP_VERSION"}},
		{key: "org.opencontainers.image.revision", uses: []string{"GITHUB_SHA"}},
		{key: "org.opencontainers.image.source", uses: []string{"GITHUB_SERVER_URL", "GITHUB_REPOSITORY"}},
		{key: "org.opencontainers.image.created", uses: []string{"date -u", "%Y-%m-%dT%H:%M:%SZ"}},
	} {
		value, ok := labels[check.key]
		if !ok {
			t.Fatalf("built image has no %s label:\n%s", check.key, steps[build].Run)
		}
		for _, use := range check.uses {
			if !strings.Contains(value, use) {
				t.Fatalf("%s label %q does not use %s", check.key, value, use)
			}
		}
	}
}

func TestChartWorkflowLintsTheChart(t *testing.T) {
	build := jobNamed(t, chartWorkflow(t), "build")

	index := stepIndexMatching(build.Steps, lintsChart)
	if index < 0 {
		t.Fatalf("chart build does not lint the chart:\n%s", renderSteps(build.Steps))
	}
	if build.Steps[index].If != "" {
		t.Fatalf("chart lint step is conditional: %q", build.Steps[index].If)
	}
}

func TestChartWorkflowSetsTheAppVersion(t *testing.T) {
	build := jobNamed(t, chartWorkflow(t), "build")

	version := build.Env["APP_VERSION"]
	if !strings.Contains(version, "needs.check.outputs.version") {
		t.Fatalf("build job APP_VERSION %q does not come from the chart check", version)
	}

	for _, match := range []func(workflowStep) bool{checksChartAppVersion} {
		index := stepIndexMatching(build.Steps, match)
		if index < 0 {
			t.Fatalf("chart build does not use the app version:\n%s", renderSteps(build.Steps))
		}
		if !usesAppVersion(build.Steps[index].Run) {
			t.Fatalf("step does not use the app version:\n%s", build.Steps[index].Run)
		}
	}
}

func TestChartWorkflowChecksChartAppVersion(t *testing.T) {
	steps := jobNamed(t, chartWorkflow(t), "build").Steps

	check := stepIndexMatching(steps, checksChartAppVersion)
	if check < 0 {
		t.Fatalf("chart build does not check the chart app version:\n%s", renderSteps(steps))
	}
	step := steps[check]
	if !strings.Contains(step.Run, filepath.Join("deploy", "chart")) {
		t.Fatalf("chart app version check does not use deploy/chart:\n%s", step.Run)
	}
	if !usesAppVersion(step.Run) {
		t.Fatalf("chart app version check does not use the app version:\n%s", step.Run)
	}

	for index, other := range steps {
		if isPublishing(other) && index < check {
			t.Fatalf("chart build publishes at %d before it checks the chart app version at %d", index, check)
		}
	}
}

func TestChartWorkflowPublishesTheChart(t *testing.T) {
	steps := jobNamed(t, chartWorkflow(t), "build").Steps

	publish := stepIndexMatching(steps, publishesChart)
	if publish < 0 {
		t.Fatalf("chart build does not publish the chart:\n%s", renderSteps(steps))
	}
	step := steps[publish]
	if !strings.Contains(step.Run, filepath.Join("deploy", "chart")) {
		t.Fatalf("chart publish does not use deploy/chart:\n%s", step.Run)
	}
	if !strings.Contains(step.Run, "oci://ghcr.io/zon/charts") {
		t.Fatalf("chart publish does not use oci://ghcr.io/zon/charts:\n%s", step.Run)
	}
}

func TestChartWorkflowLogsInToChartRegistry(t *testing.T) {
	steps := jobNamed(t, chartWorkflow(t), "build").Steps

	login := stepIndexMatching(steps, logsInToChartRegistry)
	if login < 0 {
		t.Fatalf("chart build does not log in to the chart registry:\n%s", renderSteps(steps))
	}
	if !strings.Contains(steps[login].Run, "secrets.GITHUB_TOKEN") {
		t.Fatalf("chart registry login does not use the workflow GITHUB_TOKEN:\n%s", steps[login].Run)
	}

	publish := stepIndexMatching(steps, publishesChart)
	if publish < 0 || login > publish {
		t.Fatalf("chart build logs in to the chart registry at %d but publishes at %d", login, publish)
	}
}

func TestChartWorkflowGrantsPackageWrite(t *testing.T) {
	permissions := chartWorkflow(t).Permissions

	if got := permissions["packages"]; got != "write" {
		t.Fatalf("chart workflow package permission: got %q, want write", got)
	}
	if got := permissions["contents"]; got != "read" {
		t.Fatalf("chart workflow contents permission: got %q, want read", got)
	}
}

func TestBuildWorkflowsPublishWithGitHubToken(t *testing.T) {
	secret := regexp.MustCompile(`secrets\.([A-Za-z0-9_]+)`)
	for _, workflow := range []workflow{containerWorkflow(t), chartWorkflow(t)} {
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				for _, match := range secret.FindAllStringSubmatch(step.Uses+"\n"+step.Run, -1) {
					if match[1] != "GITHUB_TOKEN" {
						t.Fatalf("step uses secret %s instead of the workflow GITHUB_TOKEN", match[1])
					}
				}
			}
		}
	}
}

func TestBuildWorkflowsPublishNoLatestTag(t *testing.T) {
	for _, workflow := range []workflow{containerWorkflow(t), chartWorkflow(t)} {
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if strings.Contains(strings.ToLower(step.Run), "latest") {
					t.Fatalf("step publishes or tags a latest image:\n%s", step.Run)
				}
			}
		}
	}
}

func pushTriggerOf(t *testing.T, workflow workflow) pushTrigger {
	t.Helper()
	node, ok := workflow.On["push"]
	if !ok {
		t.Fatal("workflow does not run on pushes")
	}

	var push pushTrigger
	if err := node.Decode(&push); err != nil {
		t.Fatal(err)
	}
	return push
}

func jobNamed(t *testing.T, workflow workflow, name string) workflowJob {
	t.Helper()
	job, ok := workflow.Jobs[name]
	if !ok {
		t.Fatalf("workflow has no %s job", name)
	}
	return job
}

func workflowHasStep(workflow workflow, match func(workflowStep) bool) bool {
	for _, job := range workflow.Jobs {
		if stepIndexMatching(job.Steps, match) >= 0 {
			return true
		}
	}
	return false
}

func buildsImage(step workflowStep) bool {
	return strings.Contains(step.Run, "podman build") &&
		strings.Contains(step.Run, filepath.Join("deploy", "container", "Containerfile"))
}

func lintsChart(step workflowStep) bool {
	return strings.Contains(step.Run, "helm lint") &&
		strings.Contains(step.Run, filepath.Join("deploy", "chart"))
}

func publishesChart(step workflowStep) bool {
	return strings.Contains(step.Run, filepath.Join("scripts", "publish-chart.sh"))
}

func checksChartAppVersion(step workflowStep) bool {
	return strings.Contains(step.Run, filepath.Join("scripts", "check-chart-app-version.sh"))
}

func isReleaseImagePublishStep(step workflowStep) bool {
	return strings.Contains(step.Run, filepath.Join("scripts", "publish-image.sh"))
}

func readsAppVersion(step workflowStep) bool {
	return strings.Contains(step.Run, "cat VERSION")
}

func readsChartVersion(step workflowStep) bool {
	return strings.Contains(step.Run, "Chart.yaml") && strings.Contains(step.Run, "version")
}

func inspectsImage(step workflowStep) bool {
	return strings.Contains(step.Run, "skopeo inspect")
}

func inspectsChart(step workflowStep) bool {
	return strings.Contains(step.Run, "helm show chart")
}

func tagsRelease(step workflowStep) bool {
	return strings.Contains(step.Run, "git tag") && strings.Contains(step.Run, "git push")
}

func runsTests(step workflowStep) bool {
	return strings.Contains(step.Run, "go test ./...")
}

func usesAppVersion(run string) bool {
	return strings.Contains(run, "$APP_VERSION") || strings.Contains(run, "${APP_VERSION}")
}

func logsInToChartRegistry(step workflowStep) bool {
	return strings.Contains(step.Run, "helm registry login") && strings.Contains(step.Run, "ghcr.io")
}

func savesImage(step workflowStep) bool {
	return strings.Contains(step.Run, "podman save")
}

func logsInToRegistry(step workflowStep) bool {
	return strings.Contains(step.Run, "podman login") && strings.Contains(step.Run, "ghcr.io")
}

func scansImage(step workflowStep) bool {
	return strings.Contains(step.Run, "trivy image")
}

func installsTrivy(step workflowStep) bool {
	return strings.Contains(step.Run, "trivy/releases/download/")
}

// builtImageTag returns the image tag a build command applies.
func builtImageTag(run string) string {
	for _, line := range strings.Split(run, "\n") {
		for _, flag := range []string{"--tag", "-t"} {
			if tag := flagValue(line, flag); tag != "" {
				return tag
			}
		}
	}
	return ""
}

// imageLabels returns the labels a build command applies, keyed by label name.
func imageLabels(run string) map[string]string {
	labels := make(map[string]string)
	label := regexp.MustCompile(`--label\s+(?:"([^"]*)"|'([^']*)'|(\S+))`)
	for _, match := range label.FindAllStringSubmatch(run, -1) {
		key, value, ok := strings.Cut(match[1]+match[2]+match[3], "=")
		if !ok {
			continue
		}
		labels[key] = value
	}
	return labels
}

// imageBuildArg returns the value of a build argument a build command applies.
func imageBuildArg(run, name string) string {
	arg := regexp.MustCompile(`--build-arg\s+(?:"([^"]*)"|'([^']*)'|(\S+))`)
	for _, match := range arg.FindAllStringSubmatch(run, -1) {
		key, value, ok := strings.Cut(match[1]+match[2]+match[3], "=")
		if ok && key == name {
			return value
		}
	}
	return ""
}

// savedArchive returns the archive path a save command writes.
func savedArchive(run string) string {
	for _, line := range strings.Split(run, "\n") {
		if !strings.Contains(line, "podman save") {
			continue
		}
		if path := flagValue(line, "-o"); path != "" {
			return path
		}
		if path := flagValue(line, "--output"); path != "" {
			return path
		}
	}
	return ""
}

// flagValue returns the argument after flag on one command line.
func flagValue(line, flag string) string {
	fields := strings.Fields(line)
	for index, field := range fields {
		if field != flag || index+1 >= len(fields) {
			continue
		}
		return strings.Trim(fields[index+1], `"'`)
	}
	return ""
}

func referencesImage(run string) bool {
	return strings.Contains(run, "$IMAGE") || strings.Contains(run, "${IMAGE}")
}

func renderSteps(steps []workflowStep) string {
	var out strings.Builder
	for index, step := range steps {
		out.WriteString(step.Run)
		if index < len(steps)-1 {
			out.WriteString("\n---\n")
		}
	}
	return out.String()
}

func stepIndexMatching(steps []workflowStep, match func(workflowStep) bool) int {
	for index, step := range steps {
		if match(step) {
			return index
		}
	}
	return -1
}

func runsAfterFailure(condition string) bool {
	condition = strings.ReplaceAll(strings.ToLower(condition), " ", "")
	for _, call := range []string{"always()", "failure()", "cancelled()", "!success()"} {
		if strings.Contains(condition, call) {
			return true
		}
	}
	return false
}

func isPublishing(step workflowStep) bool {
	text := strings.ToLower(step.Uses + "\n" + step.Run)
	for _, command := range []string{
		"podman push",
		"docker push",
		"helm push",
		"oras push",
		"crane push",
		"scripts/publish-chart.sh",
		"scripts/publish-image.sh",
	} {
		if strings.Contains(text, command) {
			return true
		}
	}
	return false
}

func isImagePublishing(step workflowStep) bool {
	text := strings.ToLower(step.Uses + "\n" + step.Run)
	for _, command := range []string{
		"podman push",
		"docker push",
		"oras push",
		"crane push",
		"scripts/publish-image.sh",
	} {
		if strings.Contains(text, command) {
			return true
		}
	}
	return false
}
