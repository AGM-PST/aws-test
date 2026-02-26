package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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
		return "", err
	}
	return string(output), nil
}

func parseConfigSections(diffOutput string) ([]string, error) {
	sections := []string{}

	lines := strings.Split(diffOutput, "\n")

	for _, line := range lines {
		if strings.Contains(line, "@@") {
			// Example:
			// @@ -18,7 +18,7 @@ b_agent:
			parts := strings.Split(line, "@@")
			if len(parts) >= 3 {
				header := strings.TrimSpace(parts[2])
				if strings.HasSuffix(header, ":") {
					section := strings.TrimSuffix(header, ":")
					fmt.Println("section: ", section)
					sections = append(sections, section)
				}
			}
		}
	}

	return sections, nil
}

func main() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("failed to load agent config: %w", err)
		return
	}

	for {
		fetchCmdOutput, err := runGitCmd(config.RepoPath, "fetch")
		if err != nil {
			fmt.Errorf("Cmd error: ", err)
			return
		}
		fmt.Println(fetchCmdOutput)

		filePath := fmt.Sprintf("%s/agent-config.yaml", host)
		diffCmdOutput, err := runGitCmd(config.RepoPath, "diff", "--name-only", "origin/main", "--", filePath)
		if err != nil {
			fmt.Errorf("Cmd error: ", err)
			return
		}
		fmt.Println(diffCmdOutput)

		// only merge if file was modified
		if diffCmdOutput != "" {
			fullDiffCmdOutput, err := runGitCmd(config.RepoPath, "diff", "origin/main", "-U0", "--", filePath)
			if err != nil {
				fmt.Errorf("Cmd error: ", err)
				return
			}

			agentNames, err := parseConfigSections(fullDiffCmdOutput)
			if err != nil {
				fmt.Errorf("Error parsing config sections: ", err)
				return
			}
			fmt.Printf("%v have been edited ", agentNames)

			// git merge
			mergeCmdOutput, err := runGitCmd(config.RepoPath, "merge")
			if err != nil {
				fmt.Errorf("Cmd error: ", err)
				return
			}
			fmt.Println(mergeCmdOutput)

			// restart agent

		}

		time.Sleep(config.DelayBetweenCmds * time.Second)
	}
}
