package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const artifactOwnerFile = ".capi-test-context"

// resolveArtifactDir selects a private run directory without creating it.
// Old run contexts remain readable without rewriting immutable identity.
func resolveArtifactDir(context *RunContext) (string, error) {
	dir := os.Getenv("CAPI_TEST_OUTPUT_DIR")
	if dir == "" {
		dir = context.OutputDir
	}
	if dir == "" {
		if strings.ContainsAny(context.TestRunID, "/\\") || context.TestRunID == "" {
			return "", fmt.Errorf("invalid run ID for artifact directory %q", context.TestRunID)
		}
		// Legacy contexts have no output_dir. Scope their stable fallback to
		// the context location so fixed cluster names cannot share credentials.
		dir = filepath.Join(filepath.Dir(RunContextFilePath()), ".capi-test-artifacts", context.TestRunID)
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(runContextWorkspace(), dir)
	}
	dir = filepath.Clean(dir)
	if context.OutputDir != "" && dir != context.OutputDir {
		return "", fmt.Errorf("CAPI_TEST_OUTPUT_DIR %q conflicts with persisted output directory %q", dir, context.OutputDir)
	}
	home, _ := os.UserHomeDir()
	if dir == string(filepath.Separator) || dir == home || dir == os.TempDir() || dir == runContextWorkspace() || dir == filepath.Dir(RunContextFilePath()) {
		return "", fmt.Errorf("unsafe artifact directory %q: select a dedicated run directory", dir)
	}
	return dir, nil
}

func ensureArtifactDir(dir string) error {
	owner := RunContextFilePath()
	marker := filepath.Join(dir, artifactOwnerFile)
	rootInfo, err := os.Lstat(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if rootInfo != nil && rootInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("artifact directory must not be a symlink: %s", dir)
	}
	if info, err := os.Lstat(marker); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("artifact ownership marker must be a private regular file: %s", marker)
		}
		if rootInfo == nil || rootInfo.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("artifact directory must be private: %s", dir)
		}
		data, err := os.ReadFile(marker)
		if err != nil {
			return err
		}
		if string(data) != owner {
			return fmt.Errorf("artifact directory %s belongs to another run context", dir)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("refusing to claim nonempty artifact directory %s", dir)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(owner), 0600)
}

// GetArtifactDir returns the private directory shared by this run's phases.
// Fail closed if identity/ownership is invalid, before writing any credentials.
func GetArtifactDir() string {
	context, err := EnsureRunContext()
	if err != nil {
		panic(fmt.Sprintf("initialize artifact directory: %v", err))
	}
	dir, err := resolveArtifactDir(context)
	if err == nil {
		err = ensureArtifactDir(dir)
	}
	if err != nil {
		panic(fmt.Sprintf("initialize artifact directory: %v", err))
	}
	return dir
}

// removeRunArtifacts only removes a directory with a matching ownership marker.
// It does not create a context or claim an existing directory during cleanup.
func removeRunArtifacts() error {
	context, err := readRunContext(RunContextFilePath())
	if err != nil {
		return err
	}
	dir, err := resolveArtifactDir(context)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		return nil
	}
	// Do not follow a substituted root or marker symlink during cleanup.
	for _, path := range []string{dir, filepath.Join(dir, artifactOwnerFile)} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to clean symlink %s", path)
		}
		if info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("refusing to clean non-private artifact path %s", path)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, artifactOwnerFile))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != RunContextFilePath() {
		return fmt.Errorf("artifact directory %s belongs to another run context", dir)
	}
	return os.RemoveAll(dir)
}
