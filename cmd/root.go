package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jdiag",
	Short: "Java diagnostics for GC logs and dumps",
	Long:  `jdiag helps analyze Java application performance through GC logs, heap dumps, and thread dumps.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip setup during completion or special commands
		if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "__complete" {
			return
		}

		// Don't run setup during completion context
		if isCompletionContext() {
			return
		}

		// Allow users to disable auto-setup
		if os.Getenv("JDIAG_NO_AUTO_SETUP") != "" {
			return
		}

		if !completionsInstalled() {
			fmt.Println("🔧 Setting up completions...")
			setupCompletions()
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func GetRootCmd() *cobra.Command {
	return rootCmd
}

func setupCompletions() {
	shell := detectShell()
	executable, _ := os.Executable()

	var completionFile string
	var sourceCmd string

	switch shell {
	case "bash":
		completionFile = filepath.Join(os.Getenv("HOME"), ".jdiag_completion")
		sourceCmd = fmt.Sprintf("echo 'source %s' >> ~/.bashrc", completionFile)
	case "zsh":
		completionFile = filepath.Join(os.Getenv("HOME"), ".jdiag_completion")
		sourceCmd = fmt.Sprintf("echo 'source %s' >> ~/.zshrc", completionFile)
	case "fish":
		os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".config/fish/completions"), 0755)
		completionFile = filepath.Join(os.Getenv("HOME"), ".config/fish/completions/jdiag.fish")
		// Fish auto-loads, no sourcing needed
	case "powershell":
		completionFile = filepath.Join(os.Getenv("HOME"), "jdiag_completion.ps1")
		sourceCmd = fmt.Sprintf("echo '& \"%s\"' >> $PROFILE", completionFile)
	default:
		fmt.Printf("❌ Shell %s not supported\n", shell)
		return
	}

	// Generate completion file: jdiag completion zsh > ~/.jdiag_completion
	cmd := exec.Command(executable, "completion", shell)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("❌ Failed to generate completions: %v\n", err)
		return
	}

	if err := os.WriteFile(completionFile, output, 0644); err != nil {
		fmt.Printf("❌ Failed to write completion file: %v\n", err)
		return
	}

	fmt.Printf("✅ %s completions installed!\n", strings.Title(shell))

	// Add to shell profile (except fish which auto-loads)
	if sourceCmd != "" && shell != "fish" {
		if err := exec.Command("sh", "-c", sourceCmd).Run(); err != nil {
			fmt.Printf("💡 Add to your %s profile: source %s\n", shell, completionFile)
		} else {
			fmt.Println("💡 Restart your shell to enable completions")
		}
	}
}

func isCompletionContext() bool {
	for _, arg := range os.Args {
		if strings.Contains(arg, "__complete") || strings.Contains(arg, "completion") {
			return true
		}
	}
	return os.Getenv("COMP_LINE") != "" || os.Getenv("_COMPLETE") != ""
}

func completionsInstalled() bool {
	shell := detectShell()
	home := os.Getenv("HOME")

	paths := map[string]string{
		"bash":       filepath.Join(home, ".jdiag_completion"),
		"zsh":        filepath.Join(home, ".jdiag_completion"),
		"fish":       filepath.Join(home, ".config/fish/completions/jdiag.fish"),
		"powershell": filepath.Join(home, "jdiag_completion.ps1"),
	}

	if path, ok := paths[shell]; ok {
		_, err := os.Stat(path)
		return err == nil
	}
	return false
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		if os.Getenv("PSModulePath") != "" {
			return "powershell"
		}
		return "powershell"
	}

	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "" {
		return "bash"
	}
	return shell
}
