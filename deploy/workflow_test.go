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
	Env  map[string]string      `yaml:"env"`
	On   map[string]yaml.Node   `yaml:"on"`
	Jobs map[string]workflowJob `yaml:"jobs"`
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

// pipelineSteps returns the steps of the job that builds the image.
func pipelineSteps(t *testing.T, workflow workflow) []workflowStep {
	t.Helper()
	for _, job := range workflow.Jobs {
		if stepIndexMatching(job.Steps, buildsImage) >= 0 {
			return job.Steps
		}
	}
	t.Fatal("workflow does not build the image with Podman")
	return nil
}

func buildsImage(step workflowStep) bool {
	return strings.Contains(step.Run, "podman build") &&
		strings.Contains(step.Run, filepath.Join("deploy", "container", "Containerfile"))
}

func savesImage(step workflowStep) bool {
	return strings.Contains(step.Run, "podman save")
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
