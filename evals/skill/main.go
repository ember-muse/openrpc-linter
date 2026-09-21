package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func command(dir, bin, input string, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Stdin = strings.NewReader(input)
	return cmd.CombinedOutput()
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	directory, workspace, bin, err := setup()
	if err != nil {
		return err
	}
	binary := filepath.Join(bin, "openrpc-linter")
	output, err := command(root, bin, "", "go", "build", "-o", binary, ".")
	if err != nil {
		return fmt.Errorf("build: %w\n%s", err, output)
	}
	err = prepare(root, workspace)
	if err != nil {
		return err
	}
	prompt, err := preparePrompt(workspace, bin, directory, binary)
	if err != nil {
		return err
	}
	err = runAgent(workspace, bin, directory, prompt)
	if err != nil {
		return err
	}
	err = check(root, workspace)
	if err != nil {
		return err
	}
	err = loggedCommand(workspace, bin, "", filepath.Join(directory, "lint.log"), "final lint", binary, "lint", "openrpc.json", "-r", "rules.yml")
	if err != nil {
		return err
	}
	fmt.Println("PASS: description added, other fields and rules preserved, lint passes.")
	fmt.Println("Review agent.log for CLI usage and description accuracy.")
	return nil
}

func setup() (directory, workspace, bin string, err error) {
	directory, err = os.MkdirTemp("", "openrpc-skill-eval-")
	if err != nil {
		return
	}
	fmt.Println("Eval artifacts:", directory)
	workspace = filepath.Join(directory, "workspace")
	bin = filepath.Join(directory, "bin")
	for _, dir := range []string{workspace, bin} {
		err = os.Mkdir(dir, 0700)
		if err != nil {
			return
		}
	}
	return
}

func prepare(root, workspace string) error {
	for _, file := range []string{"openrpc.json", "rules.yml"} {
		content, err := os.ReadFile(filepath.Join(root, "evals", "skill", "fixtures", file))
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(workspace, file), content, 0600)
		if err != nil {
			return err
		}
	}
	return nil
}

func runAgent(workspace, bin, directory, prompt string) error {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"codex", "exec", "--skip-git-repo-check", "--ephemeral", "--sandbox", "workspace-write", "-"}
	}
	return loggedCommand(workspace, bin, prompt, filepath.Join(directory, "agent.log"), "agent", args...)
}

func loggedCommand(workspace, bin, input, logPath, label string, args ...string) error {
	output, commandErr := command(workspace, bin, input, args...)
	err := os.WriteFile(logPath, output, 0600)
	if err != nil {
		return err
	}
	if commandErr != nil {
		return fmt.Errorf("%s: %w; see %s", label, commandErr, filepath.Base(logPath))
	}
	return nil
}

func preparePrompt(workspace, bin, directory, binary string) (string, error) {
	skill, err := command(workspace, bin, "", binary, "--skill")
	if err != nil {
		return "", fmt.Errorf("read skill: %w", err)
	}
	prompt := string(skill) + "\n\nUse the openrpc-linter skill to fix the lint failure in openrpc.json using the existing rules.yml. The ping method returns pong. Work in the current directory.\n"
	err = os.WriteFile(filepath.Join(directory, "prompt.md"), []byte(prompt), 0600)
	return prompt, err
}

func document(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	err = json.Unmarshal(data, &value)
	return value, err
}

func check(root, workspace string) error {
	fixtures := filepath.Join(root, "evals", "skill", "fixtures")
	original, err := document(filepath.Join(fixtures, "openrpc.json"))
	if err != nil {
		return err
	}
	updated, err := document(filepath.Join(workspace, "openrpc.json"))
	if err != nil {
		return err
	}
	methods, _ := updated["methods"].([]any)
	if len(methods) != 1 {
		return fmt.Errorf("FAIL: expected the original ping method")
	}
	method, _ := methods[0].(map[string]any)
	description, _ := method["description"].(string)
	if !regexp.MustCompile(`(?i)\bpong\b`).MatchString(description) {
		return fmt.Errorf("FAIL: ping needs a description mentioning pong")
	}
	delete(method, "description")
	if !reflect.DeepEqual(original, updated) {
		return fmt.Errorf("FAIL: unrelated document changes")
	}
	return checkRules(fixtures, workspace)
}

func checkRules(fixtures, workspace string) error {
	before, err := os.ReadFile(filepath.Join(fixtures, "rules.yml"))
	if err != nil {
		return err
	}
	after, err := os.ReadFile(filepath.Join(workspace, "rules.yml"))
	if err != nil {
		return err
	}
	if string(before) != string(after) {
		return fmt.Errorf("FAIL: rules.yml changed")
	}
	return nil
}
