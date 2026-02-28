package gemini

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/aasm3535/swittcher/internal/driver"
)

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (d *Driver) ID() string {
	return "gemini"
}

func (d *Driver) DisplayName() string {
	return "Gemini CLI"
}

func (d *Driver) IsAvailable() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}

func (d *Driver) Login(profileDir string) error {
	return runGemini(profileDir)
}

func (d *Driver) Launch(profileDir string) error {
	return runGemini(profileDir)
}

func (d *Driver) ProfileInfo(profileDir string) (driver.ProfileInfo, error) {
	return driver.ProfileInfo{}, nil
}

func (d *Driver) Usage(profileDir string) (*driver.UsageStats, error) {
	return &driver.UsageStats{}, nil
}

func runGemini(profileDir string, args ...string) error {
	cmd := exec.Command("gemini", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envWithProfileHome(profileDir)
	return cmd.Run()
}

func envWithProfileHome(profileDir string) []string {
	env := os.Environ()
	set := func(key, value string) {
		prefix := key + "="
		for i, item := range env {
			if strings.HasPrefix(item, prefix) {
				env[i] = prefix + value
				return
			}
		}
		env = append(env, prefix+value)
	}

	set("HOME", profileDir)
	if runtime.GOOS == "windows" {
		set("USERPROFILE", profileDir)
	}
	return env
}
