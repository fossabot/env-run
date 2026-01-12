package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryName = "env-run-test"

func TestMain(m *testing.M) {
	// Build the binary
	// We assume we are in the directory containing main.go
	if err := exec.Command("go", "build", "-o", binaryName, ".").Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	_ = os.Remove(binaryName)
	os.Exit(code)
}

func TestStringArray(t *testing.T) {
	var sa stringArray
	_ = sa.Set("a")
	_ = sa.Set("b")
	if len(sa) != 2 {
		t.Errorf("Expected length 2, got %d", len(sa))
	}
	if sa.String() != "[a b]" {
		t.Errorf("Expected string representation '[a b]', got '%s'", sa.String())
	}
}

func TestEnvRun(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	binaryPath := filepath.Join(cwd, binaryName)

	// Create a temp dir for all tests
	tmpDir, err := os.MkdirTemp("", "env-run-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Cleanup temp dir after tests
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	tests := []struct {
		name           string
		args           []string
		envFiles       map[string]string
		shellEnv       []string
		setup          func(string) error
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "Basic echo",
			args:           []string{"--", "echo", "hello"},
			expectedOutput: "hello", // echo usually prints hello\n, contains checks for substring
		},
		{
			name: "Single env file",
			args: []string{"-e", "test.env", "--", "sh", "-c", "echo $FOO"},
			envFiles: map[string]string{
				"test.env": "FOO=bar",
			},
			expectedOutput: "bar",
		},
		{
			name: "Multiple env files override",
			args: []string{"-e", "env1", "-e", "env2", "--", "sh", "-c", "echo $FOO $FOO2 $FOO3"},
			envFiles: map[string]string{
				"env1": "FOO=1\nFOO2=hello",
				"env2": "FOO=2\nFOO3=world",
			},
			expectedOutput: "2 hello world",
		},
		{
			name: "Shell overrides env file",
			args: []string{"-e", "env1", "--", "sh", "-c", "echo $FOO"},
			envFiles: map[string]string{
				"env1": "FOO=file",
			},
			shellEnv:       []string{"FOO=shell"},
			expectedOutput: "shell",
		},
		{
			name:           "Missing env file (default) ignored",
			args:           []string{"--", "echo", "ok"},
			expectedOutput: "ok",
		},
		{
			name:           "Missing env file (explicit) ignored",
			args:           []string{"-e", "missing.env", "--", "echo", "ok"},
			expectedOutput: "ok",
		},
		{
			name: "Working directory flag",
			args: []string{"-d", "subdir", "--", "sh", "-c", "pwd"},
			setup: func(baseDir string) error {
				return os.Mkdir(filepath.Join(baseDir, "subdir"), 0755)
			},
			expectedOutput: "subdir",
		},
		{
			name:        "Command failure",
			args:        []string{"--", "false"},
			expectError: true, // false returns exit code 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write env files
			for name, content := range tt.envFiles {
				path := filepath.Join(tmpDir, name)
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write env file %s: %v", name, err)
				}
			}

			if tt.setup != nil {
				if err := tt.setup(tmpDir); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Dir = tmpDir
			cmd.Env = os.Environ()
			if len(tt.shellEnv) > 0 {
				cmd.Env = append(cmd.Env, tt.shellEnv...)
			}

			output, err := cmd.CombinedOutput()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil. Output: %s", output)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
				return
			}

			if !strings.Contains(string(output), tt.expectedOutput) {
				t.Errorf("Output %q did not contain %q", output, tt.expectedOutput)
			}
		})
	}
}
