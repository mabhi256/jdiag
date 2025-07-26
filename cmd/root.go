package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jdiag",
	Short: "Java diagnostics for GC logs and dumps",
	Long:  `jdiag helps analyze Java application performance through GC logs, heap dumps, and thread dumps.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "install" || cmd.Name() == "version" || cmd.Name() == "help" {
			return
		}

		if !isShellSupported() {
			return // Skip auto-setup for unsupported shells
		}

		if !completionsExist() {
			fmt.Println("🔧 First run detected, setting up jdiag...")
			if installCompletions(cmd.Root()) == nil {
				fmt.Println("✅ Shell completions installed")
				fmt.Println("💡 Restart your shell to enable tab completion")
			} else {
				fmt.Println("⚠️  Auto-setup failed. Run 'jdiag install' to try again.")
			}
		}
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell completions",
	Run: func(cmd *cobra.Command, args []string) {
		if !isInPath() {
			printPathInstructions()
			return
		}

		if !isShellSupported() {
			fmt.Printf("❌ Shell completion not supported for: %s\n", detectShell())
			fmt.Println("Supported shells: bash, zsh, fish, powershell")
			return
		}

		if completionsExist() {
			fmt.Println("✅ Already configured!")
			return
		}

		fmt.Println("📦 Installing completions...")
		if err := installCompletions(cmd.Root()); err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			fmt.Println("✅ Done! Restart your shell to enable tab completion.")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func GetRootCmd() *cobra.Command {
	return rootCmd
}

func completionsExist() bool {
	home, _ := os.UserHomeDir()

	paths := map[string]string{
		"bash":       filepath.Join(home, ".local/share/bash-completion/completions/jdiag"),
		"zsh":        filepath.Join(home, ".zsh/completions/_jdiag"),
		"fish":       filepath.Join(home, ".config/fish/completions/jdiag.fish"),
		"powershell": filepath.Join(home, "jdiag_completion.ps1"),
	}

	path := paths[detectShell()]
	_, err := os.Stat(path)
	return err == nil
}

func isShellSupported() bool {
	shell := detectShell()
	return shell == "bash" || shell == "zsh" || shell == "fish" || shell == "powershell"
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}

	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "" {
		return "bash"
	}
	return shell
}

type completionConfig struct {
	dir         string
	file        string
	genFunc     func(io.Writer) error
	activateCmd string
}

func installCompletions(rootCmd *cobra.Command) error {
	home, _ := os.UserHomeDir()
	shell := detectShell()

	configs := map[string]completionConfig{
		"bash": {
			dir:     filepath.Join(home, ".local/share/bash-completion/completions"),
			file:    "jdiag",
			genFunc: rootCmd.GenBashCompletion,
			activateCmd: fmt.Sprintf("source %s",
				filepath.Join(home, ".local/share/bash-completion/completions/jdiag")),
		},
		"zsh": {
			dir:     filepath.Join(home, ".zsh/completions"),
			file:    "_jdiag",
			genFunc: rootCmd.GenZshCompletion,
			activateCmd: fmt.Sprintf("fpath=(%s $fpath) && autoload -U compinit && compinit",
				filepath.Join(home, ".zsh/completions")),
		},
		"fish": {
			dir:         filepath.Join(home, ".config/fish/completions"),
			file:        "jdiag.fish",
			genFunc:     func(w io.Writer) error { return rootCmd.GenFishCompletion(w, true) },
			activateCmd: "complete --do-complete=jdiag", // Trigger fish to reload completions
		},
		"powershell": {
			dir:     home,
			file:    "jdiag_completion.ps1",
			genFunc: rootCmd.GenPowerShellCompletionWithDesc,
			activateCmd: fmt.Sprintf(". %s",
				filepath.Join(home, "jdiag_completion.ps1")),
		},
	}

	config, ok := configs[shell]
	if !ok {
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	os.MkdirAll(config.dir, 0755)

	file, err := os.Create(filepath.Join(config.dir, config.file))
	if err != nil {
		return err
	}
	defer file.Close()

	if err := config.genFunc(file); err != nil {
		return err
	}

	// Print activation command for immediate use
	fmt.Printf("🔄 Running this command to enable auto-completions:\n")
	fmt.Printf("   %s\n", config.activateCmd)

	return nil
}

func isInPath() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	execDir := filepath.Dir(execPath)

	return slices.Contains(paths, execDir)
}

func printPathInstructions() {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)

	fmt.Printf("❌ jdiag not in PATH. Binary location: %s\n\n", execPath)

	if runtime.GOOS == "windows" {
		fmt.Printf("Add to PATH: %s\n", execDir)
	} else {
		fmt.Printf("Add to shell profile: export PATH=\"%s:$PATH\"\n", execDir)
		fmt.Printf("Or copy to: /usr/local/bin\n")
	}
}

func init() {
	rootCmd.AddCommand(installCmd)
}
