package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "stegodon_test")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer exec.Command("rm", "stegodon_test").Run()

	// Test -v flag
	cmd := exec.Command("./stegodon_test", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run with -v flag: %v", err)
	}

	outputStr := strings.TrimSpace(string(output))

	// Check if output starts with "stegodon v"
	if !strings.HasPrefix(outputStr, "stegodon v") {
		t.Errorf("Expected output to start with 'stegodon v', got: %s", outputStr)
	}

	// Check if version format is correct (should be semantic versioning)
	parts := strings.Split(outputStr, "stegodon v")
	if len(parts) != 2 {
		t.Errorf("Expected format 'stegodon vX.Y.Z', got: %s", outputStr)
	}

	version := parts[1]
	versionParts := strings.Split(version, ".")
	if len(versionParts) != 3 {
		t.Errorf("Expected semantic version format X.Y.Z, got: %s", version)
	}
}

func TestVersionFlagExitCode(t *testing.T) {
	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "stegodon_test")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer exec.Command("rm", "stegodon_test").Run()

	// Test that -v flag exits with code 0
	cmd := exec.Command("./stegodon_test", "-v")
	err := cmd.Run()

	if err != nil {
		t.Errorf("Expected exit code 0, but got error: %v", err)
	}
}
