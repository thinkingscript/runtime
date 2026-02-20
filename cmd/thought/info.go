package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/approval"
	"github.com/thinkingscript/cli/internal/config"
)

var infoCmd = &cobra.Command{
	Use:          "info <name>",
	Short:        "Show details about an installed thought",
	Long:         "Display information about a thought including paths, policy summary, memory count, and workspace size.",
	Args:         cobra.ExactArgs(1),
	RunE:         runInfo,
	SilenceUsage: true,
}

func runInfo(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveThought(args[0], "info")
	if err != nil {
		return err
	}

	if resolved.Target == TargetFile {
		return fmt.Errorf("'%s' is a file, not an installed thought.\nUse 'cat %s' to view the file.", args[0], args[0])
	}

	name := resolved.Name
	binPath := resolved.Path
	thoughtDir := filepath.Join(config.HomeDir(), "thoughts", name)

	fmt.Printf("Name: %s\n", name)

	// Binary info
	info, _ := os.Stat(binPath)
	fmt.Printf("Binary: %s (%d bytes)\n", binPath, info.Size())

	dataExists := false
	if _, err := os.Stat(thoughtDir); err == nil {
		dataExists = true
	}

	if !dataExists {
		fmt.Printf("Data: (none)\n")
		return nil
	}

	// Workspace info
	workspaceDir := filepath.Join(thoughtDir, "workspace")
	workspaceSize, workspaceCount := dirStats(workspaceDir)
	if workspaceCount > 0 {
		fmt.Printf("Workspace: %s (%d files, %s)\n", workspaceDir, workspaceCount, formatBytes(workspaceSize))
	} else {
		fmt.Printf("Workspace: %s (empty)\n", workspaceDir)
	}

	// Memories info
	memoriesDir := filepath.Join(thoughtDir, "memories")
	_, memoryCount := dirStats(memoriesDir)
	if memoryCount > 0 {
		fmt.Printf("Memories: %s (%d files)\n", memoriesDir, memoryCount)
	} else {
		fmt.Printf("Memories: %s (empty)\n", memoriesDir)
	}

	// Policy info
	policyPath := filepath.Join(thoughtDir, "policy.json")
	if _, err := os.Stat(policyPath); err == nil {
		policy, err := loadPolicySummary(policyPath)
		if err != nil {
			fmt.Printf("Policy: %s (error reading: %v)\n", policyPath, err)
		} else {
			fmt.Printf("Policy: %s\n", policyPath)
			if len(policy.pathSummary) > 0 {
				fmt.Printf("  Paths: %s\n", policy.pathSummary)
			}
			if len(policy.envSummary) > 0 {
				fmt.Printf("  Env: %s\n", policy.envSummary)
			}
			if len(policy.netSummary) > 0 {
				fmt.Printf("  Net: %s\n", policy.netSummary)
			}
		}
	} else {
		fmt.Printf("Policy: (default)\n")
	}

	return nil
}

func dirStats(dir string) (totalSize int64, fileCount int) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})
	return
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

type policySummary struct {
	pathSummary string
	envSummary  string
	netSummary  string
}

func loadPolicySummary(path string) (*policySummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policy approval.Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}

	summary := &policySummary{}

	// Summarize paths
	allowedPaths := 0
	deniedPaths := 0
	for _, entry := range policy.Paths.Entries {
		if entry.Approval == approval.ApprovalAllow {
			allowedPaths++
		} else if entry.Approval == approval.ApprovalDeny {
			deniedPaths++
		}
	}
	if allowedPaths > 0 || deniedPaths > 0 {
		summary.pathSummary = fmt.Sprintf("%d allowed, %d denied", allowedPaths, deniedPaths)
	}

	// Summarize env
	allowedEnv := 0
	deniedEnv := 0
	for _, entry := range policy.Env.Entries {
		if entry.Approval == approval.ApprovalAllow {
			allowedEnv++
		} else if entry.Approval == approval.ApprovalDeny {
			deniedEnv++
		}
	}
	if allowedEnv > 0 || deniedEnv > 0 {
		summary.envSummary = fmt.Sprintf("%d allowed, %d denied", allowedEnv, deniedEnv)
	}

	// Summarize net hosts
	allowedHosts := 0
	deniedHosts := 0
	for _, entry := range policy.Net.Hosts.Entries {
		if entry.Approval == approval.ApprovalAllow {
			allowedHosts++
		} else if entry.Approval == approval.ApprovalDeny {
			deniedHosts++
		}
	}
	if allowedHosts > 0 || deniedHosts > 0 {
		summary.netSummary = fmt.Sprintf("%d hosts allowed, %d denied", allowedHosts, deniedHosts)
	}

	return summary, nil
}
