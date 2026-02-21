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

var policyCmd = &cobra.Command{
	Use:          "policy",
	Short:        "Manage policy settings",
	Long:         "View and manage policy entries for paths, environment variables, and network hosts.",
	SilenceUsage: true,
}

var policyListCmd = &cobra.Command{
	Use:          "ls [name]",
	Aliases:      []string{"list"},
	Short:        "List policy entries",
	Long:         "List all policy entries for an installed thought.\nIf no name is provided, lists the global policy.",
	Args:         cobra.MaximumNArgs(1),
	RunE:         runPolicyList,
	SilenceUsage: true,
}

var policyAddCmd = &cobra.Command{
	Use:   "add <type> <name> <value>",
	Short: "Add a policy entry",
	Long: `Add a policy entry for an installed thought. Type must be 'path', 'env', or 'host'.

Examples:
  thought policy add path myapp /Users/brad/data --mode rwd
  thought policy add env myapp HOME
  thought policy add host myapp "*.github.com"`,
	Args:         cobra.ExactArgs(3),
	RunE:         runPolicyAdd,
	SilenceUsage: true,
}

var policyRemoveCmd = &cobra.Command{
	Use:   "rm <type> <name> <value>",
	Aliases: []string{"remove"},
	Short: "Remove a policy entry",
	Long: `Remove a policy entry from an installed thought. Type must be 'path', 'env', or 'host'.

Examples:
  thought policy rm path myapp /Users/brad/data
  thought policy rm env myapp HOME
  thought policy rm host myapp "*.github.com"`,
	Args:         cobra.ExactArgs(3),
	RunE:         runPolicyRemove,
	SilenceUsage: true,
}

var (
	policyModeFlag     string
	policyApprovalFlag string
)

func init() {
	policyAddCmd.Flags().StringVar(&policyModeFlag, "mode", "rwd", "Permission mode for paths (r=read, w=write, d=delete)")
	policyAddCmd.Flags().StringVar(&policyApprovalFlag, "approval", "allow", "Approval decision (allow, deny, prompt)")

	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyRemoveCmd)
}

func runPolicyList(cmd *cobra.Command, args []string) error {
	var policyPath string
	if len(args) == 0 {
		policyPath = filepath.Join(config.HomeDir(), "policy.json")
	} else {
		thoughtDir := filepath.Join(config.HomeDir(), "thoughts", args[0])
		policyPath = filepath.Join(thoughtDir, "policy.json")
	}

	policy, err := approval.LoadPolicy(policyPath)
	if err != nil {
		return fmt.Errorf("loading policy: %w", err)
	}

	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("formatting policy: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func runPolicyAdd(cmd *cobra.Command, args []string) error {
	entryType := args[0]
	thoughtName := args[1]
	value := args[2]

	thoughtDir := filepath.Join(config.HomeDir(), "thoughts", thoughtName)
	policyPath := filepath.Join(thoughtDir, "policy.json")

	policy, err := approval.LoadPolicy(policyPath)
	if err != nil {
		return fmt.Errorf("loading policy: %w", err)
	}

	approvalVal := approval.Approval(policyApprovalFlag)
	if approvalVal != approval.ApprovalAllow && approvalVal != approval.ApprovalDeny && approvalVal != approval.ApprovalPrompt {
		return fmt.Errorf("invalid approval value: %s (must be allow, deny, or prompt)", policyApprovalFlag)
	}

	switch entryType {
	case "path":
		policy.AddPathEntry(value, policyModeFlag, approvalVal, approval.SourceCLI)
		fmt.Fprintf(os.Stderr, "Added path entry: %s (mode=%s, approval=%s)\n", value, policyModeFlag, policyApprovalFlag)
	case "env":
		policy.AddEnvEntry(value, approvalVal, approval.SourceCLI)
		fmt.Fprintf(os.Stderr, "Added env entry: %s (approval=%s)\n", value, policyApprovalFlag)
	case "host":
		policy.AddHostEntry(value, approvalVal, approval.SourceCLI)
		fmt.Fprintf(os.Stderr, "Added host entry: %s (approval=%s)\n", value, policyApprovalFlag)
	default:
		return fmt.Errorf("invalid type: %s (must be path, env, or host)", entryType)
	}

	if err := policy.Save(policyPath); err != nil {
		return fmt.Errorf("saving policy: %w", err)
	}

	return nil
}

func runPolicyRemove(cmd *cobra.Command, args []string) error {
	entryType := args[0]
	thoughtName := args[1]
	value := args[2]

	thoughtDir := filepath.Join(config.HomeDir(), "thoughts", thoughtName)
	policyPath := filepath.Join(thoughtDir, "policy.json")

	policy, err := approval.LoadPolicy(policyPath)
	if err != nil {
		return fmt.Errorf("loading policy: %w", err)
	}

	var removed bool
	switch entryType {
	case "path":
		newEntries := make([]approval.PathEntry, 0, len(policy.Paths.Entries))
		for _, e := range policy.Paths.Entries {
			if e.Path != value {
				newEntries = append(newEntries, e)
			} else {
				removed = true
			}
		}
		policy.Paths.Entries = newEntries

	case "env":
		newEntries := make([]approval.EnvEntry, 0, len(policy.Env.Entries))
		for _, e := range policy.Env.Entries {
			if e.Name != value {
				newEntries = append(newEntries, e)
			} else {
				removed = true
			}
		}
		policy.Env.Entries = newEntries

	case "host":
		newEntries := make([]approval.HostEntry, 0, len(policy.Net.Hosts.Entries))
		for _, e := range policy.Net.Hosts.Entries {
			if e.Host != value {
				newEntries = append(newEntries, e)
			} else {
				removed = true
			}
		}
		policy.Net.Hosts.Entries = newEntries

	default:
		return fmt.Errorf("invalid type: %s (must be path, env, or host)", entryType)
	}

	if !removed {
		return fmt.Errorf("entry not found: %s %s", entryType, value)
	}

	if err := policy.Save(policyPath); err != nil {
		return fmt.Errorf("saving policy: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Removed %s entry: %s\n", entryType, value)
	return nil
}
