package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thinkingscript/cli/internal/config"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:     "memory",
	Aliases: []string{"mem"},
	Short:   "Manage script memories",
}

var memoryLsCmd = &cobra.Command{
	Use:          "ls [thought]",
	Short:        "List memories for a thought, or all thoughts",
	Long:         "List memory files for an installed thought, local file, or URL.\nWith no argument, lists all memories across all thoughts.",
	Args:         cobra.MaximumNArgs(1),
	RunE:         runMemoryLs,
	SilenceUsage: true,
}

func init() {
	memoryCmd.AddCommand(memoryLsCmd)
}

func runMemoryLs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return listAllMemories()
	}
	return listScriptMemories(args[0])
}

func listAllMemories() error {
	thoughtsBase := filepath.Join(config.HomeDir(), "thoughts")
	entries, err := os.ReadDir(thoughtsBase)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "No memories yet.")
			return nil
		}
		return err
	}

	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		memoriesDir := filepath.Join(thoughtsBase, e.Name(), "memories")
		memEntries, err := os.ReadDir(memoriesDir)
		if err != nil || len(memEntries) == 0 {
			continue
		}

		found = true
		for _, m := range memEntries {
			if !m.IsDir() {
				fmt.Fprintln(os.Stdout, filepath.Join(memoriesDir, m.Name()))
			}
		}
	}

	if !found {
		fmt.Fprintln(os.Stderr, "No memories yet.")
	}
	return nil
}

func listScriptMemories(arg string) error {
	resolved, err := ResolveThought(arg, "memory ls")
	if err != nil {
		return err
	}

	var memoriesDir string
	if resolved.Target == TargetInstalled {
		memoriesDir = filepath.Join(config.HomeDir(), "thoughts", resolved.Name, "memories")
	} else {
		memoriesDir = config.MemoriesDir(resolved.Path)
	}

	entries, err := os.ReadDir(memoriesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "No memories yet.")
			return nil
		}
		return fmt.Errorf("reading memories: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No memories yet.")
		return nil
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fmt.Fprintln(os.Stdout, filepath.Join(memoriesDir, e.Name()))
	}

	return nil
}
