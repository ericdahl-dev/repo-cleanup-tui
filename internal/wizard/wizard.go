package wizard

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/config"
)

var ErrUserAborted = huh.ErrUserAborted

func RunInteractive(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("config already exists at %s (use --force to overwrite)", path)
	}

	var workspacePath string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Workspace path").
				Description("Directory tree to scan for git repos with node_modules (e.g. ~/Documents/GitHub).").
				Value(&workspacePath).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("workspace path is required")
					}
					return nil
				}),
		).Title("repo-cleanup-tui init").Description("Create starter config.toml"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	expanded := strings.TrimSpace(workspacePath)
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		expanded = strings.Replace(expanded, "~", home, 1)
	}

	return config.WriteStarter(path, expanded)
}
