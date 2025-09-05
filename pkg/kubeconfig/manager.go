package kubeconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// MergeToDefault writes the kubeconfig bytes to a temporary file and merges it
// into the default kubeconfig (~/.kube/config) using kubectl.
func MergeToDefault(kubeconfigBytes []byte, contextName string) error {
	// Write to temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("mck-kubeconfig-%s", contextName))
	if err := os.WriteFile(tmpFile, kubeconfigBytes, 0600); err != nil {
		return fmt.Errorf("writing temp kubeconfig: %w", err)
	}
	defer os.Remove(tmpFile)

	// Merge with existing kubeconfig
	defaultPath := DefaultPath()
	
	// Ensure ~/.kube directory exists
	kubeDir := filepath.Dir(defaultPath)
	if err := os.MkdirAll(kubeDir, 0700); err != nil {
		return fmt.Errorf("creating .kube directory: %w", err)
	}

	// If default kubeconfig doesn't exist, just copy the new one
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		return os.WriteFile(defaultPath, kubeconfigBytes, 0600)
	}

	// Use KUBECONFIG env var trick to merge
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}
	mergedEnv := defaultPath + separator + tmpFile

	cmd := exec.Command("kubectl", "config", "view", "--flatten")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+mergedEnv)
	merged, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("merging kubeconfigs: %w", err)
	}

	// Write the merged config back
	return os.WriteFile(defaultPath, merged, 0600)
}

// SetContext sets the current kubectl context.
func SetContext(contextName string) error {
	cmd := exec.Command("kubectl", "config", "use-context", contextName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SaveToFile writes kubeconfig bytes to a specific file path.
func SaveToFile(kubeconfigBytes []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	return os.WriteFile(path, kubeconfigBytes, 0600)
}

// DefaultPath returns the default kubeconfig path.
func DefaultPath() string {
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		return kc
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".kube", "config")
	}
	return filepath.Join(home, ".kube", "config")
}
