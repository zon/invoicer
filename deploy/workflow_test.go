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
	If    string         `yaml:"if"`
	Steps []workflowStep `yaml:"steps"`
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

func loadWorkflow(t *testing.T) workflow {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".github", "workflows", "build.yaml")
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

func TestWorkflowRunsOnlyOnVersionTags(t *testing.T) {
	workflow := loadWorkflow(t)
	push := pushTriggerOf(t, workflow)

	if !slices.Contains(push.Tags, "v*") {
		t.Fatalf("workflow push tags: got %v, want v*", push.Tags)
	}
	if len(push.Branches) != 0 {
		t.Fatalf("workflow push branches: got %v, want none", push.Branches)
	}
	if _, ok := workflow.On["pull_request"]; ok {
		t.Fatal("workflow runs on pull requests, want only version tags")
	}
}

func TestWorkflowRunsFromTheReleaseWorkflow(t *testing.T) {
	if _, ok := loadWorkflow(t).On["workflow_call"]; !ok {
		t.Fatal("workflow is not callable by the release workflow")
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

func TestWorkflowRunsTests(t *testing.T) {
	for _, job := range loadWorkflow(t).Jobs {
		if stepIndexRunning(job.Steps, "go test ./...") >= 0 {
			return
		}
	}
	t.Fatal("workflow does not run go test ./...")
}

func TestWorkflowTestsBeforePublishing(t *testing.T) {
	for jobName, job := range loadWorkflow(t).Jobs {
		testIndex := stepIndexRunning(job.Steps, "go test ./...")
		for index, step := range job.Steps {
			if !isPublishing(step) {
				continue
			}
			if testIndex < 0 {
				t.Fatalf("job %s publishes without running go test ./...", jobName)
			}
			if index < testIndex {
				t.Fatalf("job %s publishes before running go test ./...", jobName)
			}
		}
	}
}

func TestWorkflowStopsBeforePublishingWhenTestsFail(t *testing.T) {
	for jobName, job := range loadWorkflow(t).Jobs {
		for _, step := range job.Steps {
			if !isPublishing(step) {
				continue
			}
			if runsAfterFailure(step.If) || runsAfterFailure(job.If) {
				t.Fatalf("job %s can publish after a failed test", jobName)
			}
		}
	}
}

func TestWorkflowBuildsImageAndLintsChart(t *testing.T) {
	job := pipelineJob(t, loadWorkflow(t))

	for _, check := range []struct {
		name  string
		match func(workflowStep) bool
	}{
		{name: "build the image", match: buildsImage},
		{name: "lint the chart", match: lintsChart},
	} {
		index := stepIndexMatching(job.Steps, check.match)
		if index < 0 {
			t.Fatalf("workflow does not %s:\n%s", check.name, renderSteps(job.Steps))
		}
		if job.If != "" || job.Steps[index].If != "" {
			t.Fatalf("%s step is conditional, so the build skips it", check.name)
		}
	}
}

func TestWorkflowBuildsImageWithPodman(t *testing.T) {
	workflow := loadWorkflow(t)
	steps := pipelineSteps(t, workflow)

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("workflow does not build the image with Podman from deploy/container/Containerfile")
	}

	if image := workflow.Env["IMAGE"]; image == "" {
		t.Fatal("workflow does not name the image in IMAGE")
	}

	tag := builtImageTag(steps[build].Run)
	if tag == "" {
		t.Fatalf("build step tags no image:\n%s", steps[build].Run)
	}
	if strings.HasSuffix(tag, ":latest") {
		t.Fatalf("build step tags the image latest: %s", tag)
	}
	if !referencesImage(steps[build].Run) {
		t.Fatalf("build step does not use the workflow IMAGE:\n%s", steps[build].Run)
	}
}

func TestWorkflowScansTheBuiltImage(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	build := stepIndexMatching(steps, buildsImage)
	save := stepIndexMatching(steps, savesImage)
	scan := stepIndexMatching(steps, scansImage)
	if save < 0 {
		t.Fatalf("workflow does not save the built image:\n%s", renderSteps(steps))
	}
	if scan < 0 {
		t.Fatalf("workflow does not scan the image with Trivy:\n%s", renderSteps(steps))
	}
	if build > save || save > scan {
		t.Fatalf("workflow builds at %d, saves at %d, scans at %d, want build then save then scan", build, save, scan)
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

func TestWorkflowPinsTrivyVersion(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	install := stepIndexMatching(steps, installsTrivy)
	if install < 0 {
		t.Fatalf("workflow does not install Trivy:\n%s", renderSteps(steps))
	}
	scan := stepIndexMatching(steps, scansImage)
	if scan < 0 {
		t.Fatalf("workflow does not scan the image with Trivy:\n%s", renderSteps(steps))
	}
	if install > scan {
		t.Fatalf("workflow scans at %d before it installs Trivy at %d", scan, install)
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

func TestWorkflowStopsOnHighOrCritical(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	scan := stepIndexMatching(steps, scansImage)
	if scan < 0 {
		t.Fatal("workflow does not scan the image with Trivy")
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

func TestWorkflowScansBeforePublishing(t *testing.T) {
	for jobName, job := range loadWorkflow(t).Jobs {
		scanIndex := stepIndexMatching(job.Steps, scansImage)
		for index, step := range job.Steps {
			if !isPublishing(step) {
				continue
			}
			if scanIndex < 0 {
				t.Fatalf("job %s publishes without scanning the image", jobName)
			}
			if index < scanIndex {
				t.Fatalf("job %s publishes before scanning the image", jobName)
			}
		}
	}
}

func TestWorkflowStopsBeforePublishingWhenScanFails(t *testing.T) {
	for jobName, job := range loadWorkflow(t).Jobs {
		for _, step := range job.Steps {
			if !isPublishing(step) {
				continue
			}
			if runsAfterFailure(step.If) || runsAfterFailure(job.If) {
				t.Fatalf("job %s can publish after a failed scan", jobName)
			}
		}
	}
}

func TestWorkflowSetsTheAppVersion(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	index := stepIndexMatching(steps, setsAppVersion)
	if index < 0 {
		t.Fatalf("workflow does not set APP_VERSION:\n%s", renderSteps(steps))
	}
	run := steps[index].Run
	if input := steps[index].Env["INPUT_VERSION"]; !strings.Contains(input, "inputs.version") {
		t.Fatalf("version step does not read the release version, got %q", input)
	}
	if !strings.Contains(run, "INPUT_VERSION") {
		t.Fatalf("version step does not use the release version:\n%s", run)
	}
	if !strings.Contains(run, "scripts/version.sh") {
		t.Fatalf("version step does not fall back to the tag version:\n%s", run)
	}

	for _, match := range []func(workflowStep) bool{buildsImage, checksChartAppVersion, isReleaseImagePublishStep} {
		step := stepIndexMatching(steps, match)
		if step < 0 || step < index {
			t.Fatalf("step at %d uses the app version before the version step at %d", step, index)
		}
	}
}

func TestWorkflowPublishesTheReleaseImage(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	publish := stepIndexMatching(steps, isReleaseImagePublishStep)
	if publish < 0 {
		t.Fatalf("workflow does not publish the release image:\n%s", renderSteps(steps))
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
		t.Fatal("workflow does not build the image")
	}
	if tag := builtImageTag(steps[build].Run); !strings.Contains(step.Run, tag) {
		t.Fatalf("release image publish does not use the built image %s:\n%s", tag, step.Run)
	}
}

func TestWorkflowLogsInForPublish(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	login := stepIndexMatching(steps, logsInToRegistry)
	publish := stepIndexMatching(steps, isReleaseImagePublishStep)
	if login < 0 || publish < 0 || login > publish {
		t.Fatalf("workflow logs in to the registry at %d but publishes the release at %d", login, publish)
	}
	if steps[login].If != "" {
		t.Fatalf("registry login is conditional: %q", steps[login].If)
	}
}

func TestWorkflowChecksChartAppVersion(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	check := stepIndexMatching(steps, checksChartAppVersion)
	if check < 0 {
		t.Fatalf("workflow does not check the chart app version:\n%s", renderSteps(steps))
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
			t.Fatalf("workflow publishes at %d before it checks the chart app version at %d", index, check)
		}
	}
}

func TestWorkflowStampsAppVersionIntoImage(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("workflow does not build the image")
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
		t.Fatalf("workflow does not publish the release image:\n%s", renderSteps(steps))
	}
	if !usesAppVersion(steps[publish].Run) {
		t.Fatalf("release image publish does not use the same app version:\n%s", steps[publish].Run)
	}
}

func TestWorkflowLabelsTheImage(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("workflow does not build the image")
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

func TestWorkflowPublishesNoLatestTag(t *testing.T) {
	for _, job := range loadWorkflow(t).Jobs {
		for _, step := range job.Steps {
			if strings.Contains(strings.ToLower(step.Run), "latest") {
				t.Fatalf("step publishes or tags a latest image:\n%s", step.Run)
			}
		}
	}
}

func TestWorkflowPublishesTheChart(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	publish := stepIndexMatching(steps, publishesChart)
	if publish < 0 {
		t.Fatalf("workflow does not publish the chart:\n%s", renderSteps(steps))
	}
	step := steps[publish]
	if !strings.Contains(step.Run, filepath.Join("deploy", "chart")) {
		t.Fatalf("chart publish does not use deploy/chart:\n%s", step.Run)
	}
	if !strings.Contains(step.Run, "oci://ghcr.io/zon/charts") {
		t.Fatalf("chart publish does not use oci://ghcr.io/zon/charts:\n%s", step.Run)
	}
}

func TestWorkflowLogsInToChartRegistry(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	login := stepIndexMatching(steps, logsInToChartRegistry)
	if login < 0 {
		t.Fatalf("workflow does not log in to the chart registry:\n%s", renderSteps(steps))
	}
	if !strings.Contains(steps[login].Run, "secrets.GITHUB_TOKEN") {
		t.Fatalf("chart registry login does not use the workflow GITHUB_TOKEN:\n%s", steps[login].Run)
	}

	publish := stepIndexMatching(steps, publishesChart)
	if publish < 0 || login > publish {
		t.Fatalf("workflow logs in to the chart registry at %d but publishes at %d", login, publish)
	}
}

func TestWorkflowGrantsPackageWrite(t *testing.T) {
	permissions := loadWorkflow(t).Permissions

	if got := permissions["packages"]; got != "write" {
		t.Fatalf("workflow package permission: got %q, want write", got)
	}
	if got := permissions["contents"]; got != "read" {
		t.Fatalf("workflow contents permission: got %q, want read", got)
	}
}

func TestWorkflowPublishesWithGitHubToken(t *testing.T) {
	steps := pipelineSteps(t, loadWorkflow(t))

	login := stepIndexMatching(steps, logsInToRegistry)
	if login < 0 {
		t.Fatalf("workflow does not log in to the registry:\n%s", renderSteps(steps))
	}
	publish := stepIndexMatching(steps, isPublishing)
	if publish < 0 || login > publish {
		t.Fatalf("workflow logs in at %d but publishes at %d", login, publish)
	}
	if !strings.Contains(steps[login].Run, "secrets.GITHUB_TOKEN") {
		t.Fatalf("registry login does not use the workflow GITHUB_TOKEN:\n%s", steps[login].Run)
	}

	secret := regexp.MustCompile(`secrets\.([A-Za-z0-9_]+)`)
	for _, step := range steps {
		for _, match := range secret.FindAllStringSubmatch(step.Uses+"\n"+step.Run, -1) {
			if match[1] != "GITHUB_TOKEN" {
				t.Fatalf("step uses secret %s instead of the workflow GITHUB_TOKEN", match[1])
			}
		}
	}
}

// pipelineJob returns the job that builds the image.
func pipelineJob(t *testing.T, workflow workflow) workflowJob {
	t.Helper()
	for _, job := range workflow.Jobs {
		if stepIndexMatching(job.Steps, buildsImage) >= 0 {
			return job
		}
	}
	t.Fatal("workflow does not build the image with Podman")
	return workflowJob{}
}

// pipelineSteps returns the steps of the job that builds the image.
func pipelineSteps(t *testing.T, workflow workflow) []workflowStep {
	t.Helper()
	return pipelineJob(t, workflow).Steps
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

func setsAppVersion(step workflowStep) bool {
	return strings.Contains(step.Run, "APP_VERSION=") &&
		strings.Contains(step.Run, "GITHUB_ENV")
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

func stepIndexRunning(steps []workflowStep, command string) int {
	for index, step := range steps {
		if strings.Contains(step.Run, command) {
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
