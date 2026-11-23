package deploy_test

import (
	"os"
	"path/filepath"
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
	On   map[string]yaml.Node   `yaml:"on"`
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	If    string         `yaml:"if"`
	Steps []workflowStep `yaml:"steps"`
}

type workflowStep struct {
	Uses string `yaml:"uses"`
	Run  string `yaml:"run"`
	If   string `yaml:"if"`
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
