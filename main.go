package main

import (
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

	b, err := os.ReadFile("/etc/default/aws-test-config.yaml")
	if err != nil {
		return zero, err
	}
	if len(b) == 0 {
		return zero, errors.New("config empty")
	}

	yamlRootKey := "aws_test"

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

func runGitCmd(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("git failed")
		fmt.Println("repoPath:", repoPath)
		fmt.Println("args:", args)
		fmt.Println("output:\n", string(output))
		return "", err
	}
	return string(output), nil
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
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	fmt.Println("Running as:", os.Getenv("USER"))
	fmt.Println("host:", host)
	fmt.Println("loading config")
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("failed to load agent config: %w", err)
		return
	}
	fmt.Println("config loaded ", config)
	// incrementedFiles keeps track of which files/agents have already been incremented
	// var incrementedFiles []string

	for {
		fmt.Println("Fetching")
		fetchCmdOutput, err := runGitCmd(config.RepoPath, "fetch")
		if err != nil {
			fmt.Errorf("Cmd error: ", err)
			return
		}
		fmt.Println(fetchCmdOutput)

		filePath := fmt.Sprintf("%s/agent-config.yaml", host)
		diffCmdOutput, err := runGitCmd(config.RepoPath, "diff", "--name-only", "origin/main", "--", filePath)
		fmt.Println("diffCmdOutput ", diffCmdOutput)
		if err != nil {
			fmt.Errorf("Cmd error: ", err)
			return
		}

		// only merge if file was modified
		if diffCmdOutput != "" {
			// git merge
			mergeCmdOutput, err := runGitCmd(config.RepoPath, "merge")
			if err != nil {
				fmt.Errorf("Cmd error: ", err)
				return
			}
			fmt.Println(mergeCmdOutput)
		}

		time.Sleep(config.DelayBetweenCmds * time.Second)
	}
}
