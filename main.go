package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	version   = "1.0.2"
	major     = 0
	minor     = 1
	increment = 2
)

type Config struct {
	RepoPath         string        `yaml:"repo_path"`
	DelayBetweenCmds time.Duration `yaml:"delay_between_cmds"`
}

func loadConfig() (Config, error) {
	var zero Config

	b, err := os.ReadFile("config.yaml")
	if err != nil {
		return zero, err
	}
	if len(b) == 0 {
		return zero, errors.New("config empty")
	}

	yamlRootKey := "agents_version_control"

	// Unmarshal into a map first to extract the specific key
	var raw map[string]interface{}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return zero, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Added to convert to typical snake_case for YAML keys
	yamlHeader := strings.ReplaceAll(yamlRootKey, "-", "_")
	// Extract the config for this agent's key
	agentConfigRaw, ok := raw[yamlHeader]
	if !ok {
		return zero, fmt.Errorf("config key '%s' not found in YAML", yamlRootKey)
	}

	// Marshal the extracted config back to YAML
	agentConfigBytes, err := yaml.Marshal(agentConfigRaw)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal agent config: %w", err)
	}

	// Unmarshal directly into the config type (no wrapper needed)
	var config Config
	if err := yaml.Unmarshal(agentConfigBytes, &config); err != nil {
		return zero, fmt.Errorf("failed to unmarshal into config type: %w", err)
	}

	return config, nil
}

func runGitDiff(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("sudo git", args...)
	cmd.Dir = repoPath

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("sudo git %v failed: %v: %s", args, err, stderr.String())
	}

	return out.String(), nil
}

func runGitCmd(repoPath string, args ...string) error {
	cmd := exec.Command("sudo git", args...)
	cmd.Dir = repoPath

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func incrementVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		fmt.Println("invalid version format: " + version)
	}

	increment, err := strconv.Atoi(parts[increment])
	if err != nil {
		fmt.Println("Str conversion error: ", err)
	}

	increment += 1
	newIncrementStr := strconv.Itoa(increment)
	newVersionStr := parts[major] + "." + parts[minor] + "." + newIncrementStr

	return newVersionStr
}

func main() {
	fmt.Println("loading config")
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("failed to load agent config: %w", err)
		return
	}
	fmt.Println("config loaded")
	// incrementedFiles keeps track of which files/agents have already been incremented
	// var incrementedFiles []string

	for {
		fmt.Println("Fetching")
		err := runGitCmd(config.RepoPath, "fetch")
		if err != nil {
			fmt.Errorf("Cmd error: ", err)
			return
		}
		fmt.Println("Fetched")

		diffCmdOutput, err := runGitDiff(config.RepoPath, "diff", "--name-only", "origin/main", "--", "$(hostname)/agent-config.yaml")
		fmt.Println("diffCmdOutput ", diffCmdOutput)
		if err != nil {
			fmt.Errorf("Cmd error: ", err)
			return
		}

		// only merge if file was modified
		if diffCmdOutput != "" {
			// git merge
			err := runGitCmd(config.RepoPath, "merge")
			if err != nil {
				fmt.Errorf("Cmd error: ", err)
				return
			}
			fmt.Println("merged main <- origin/main")
		}

		time.Sleep(config.DelayBetweenCmds * time.Second)
	}
}
