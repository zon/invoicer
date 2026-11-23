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
	Uses            string `yaml:"uses"`
	Run             string `yaml:"run"`
	If              string `yaml:"if"`
	ContinueOnError bool   `yaml:"continue-on-error"`
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

func TestWorkflowRunsOnPullRequests(t *testing.T) {
	workflow := loadWorkflow(t)

	if _, ok := workflow.On["pull_request"]; !ok {
		t.Fatal("workflow does not run on pull requests")
	}
}

func TestWorkflowRunsOnMainPushes(t *testing.T) {
	push := pushTriggerOf(t, loadWorkflow(t))

	if !slices.Contains(push.Branches, "main") {
		t.Fatalf("workflow push branches: got %v, want main", push.Branches)
	}
}

func TestWorkflowRunsOnVersionTags(t *testing.T) {
	push := pushTriggerOf(t, loadWorkflow(t))

	if !slices.Contains(push.Tags, "v*") {
		t.Fatalf("workflow push tags: got %v, want v*", push.Tags)
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

func TestPullRequestRunPublishesNothing(t *testing.T) {
	for jobName, job := range loadWorkflow(t).Jobs {
		for _, step := range job.Steps {
			if !isPublishing(step) {
				continue
			}
			if step.If == "" && job.If == "" {
				t.Fatalf("job %s publishes unconditionally", jobName)
			}
		}
	}
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

func TestPullRequestRunBuildsImageAndLintsChart(t *testing.T) {
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
			t.Fatalf("%s step is conditional, so pull requests skip it", check.name)
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

func TestWorkflowPublishesMainPushToRegistry(t *testing.T) {
	workflow := loadWorkflow(t)
	steps := pipelineSteps(t, workflow)

	publish := stepIndexMatching(steps, isPublishing)
	if publish < 0 {
		t.Fatalf("workflow does not publish the image:\n%s", renderSteps(steps))
	}
	step := steps[publish]
	if !strings.Contains(step.If, "github.event_name == 'push'") || !strings.Contains(step.If, "refs/heads/main") {
		t.Fatalf("publish step does not run only on main pushes: %q", step.If)
	}
	if image := workflow.Env["IMAGE"]; image != "ghcr.io/zon/invoicer" {
		t.Fatalf("workflow publishes to %q, want ghcr.io/zon/invoicer", image)
	}
	if !referencesImage(step.Run) {
		t.Fatalf("publish step does not use the workflow IMAGE:\n%s", step.Run)
	}
	if !strings.Contains(step.Run, "sha-") || !strings.Contains(step.Run, "GITHUB_SHA") {
		t.Fatalf("publish step does not tag the image with the short commit:\n%s", step.Run)
	}

	build := stepIndexMatching(steps, buildsImage)
	if build < 0 {
		t.Fatal("workflow does not build the image")
	}
	if tag := builtImageTag(steps[build].Run); !strings.Contains(step.Run, tag) {
		t.Fatalf("publish step does not use the built image %s:\n%s", tag, step.Run)
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
		{key: "org.opencontainers.image.version", uses: []string{"scripts/version.sh"}},
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
	} {
		if strings.Contains(text, command) {
			return true
		}
	}
	return false
}
