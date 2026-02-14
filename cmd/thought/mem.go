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
	Use:          "ls [script]",
	Short:        "List memories for a script, or all scripts",
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
	memoriesBase := filepath.Join(config.HomeDir(), "memories")
	entries, err := os.ReadDir(memoriesBase)
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
		thoughtDir := filepath.Join(memoriesBase, e.Name())
		memEntries, err := os.ReadDir(thoughtDir)
		if err != nil || len(memEntries) == 0 {
			continue
		}

		found = true
		for _, m := range memEntries {
			if !m.IsDir() {
				fmt.Fprintln(os.Stdout, filepath.Join(thoughtDir, m.Name()))
			}
		}
	}

	if !found {
		fmt.Fprintln(os.Stderr, "No memories yet.")
	}
	return nil
}

func listScriptMemories(scriptPath string) error {
	memoriesDir := config.MemoriesDir(scriptPath)
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
