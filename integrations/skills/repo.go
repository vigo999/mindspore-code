package skills

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultRepoURL           = "https://github.com/vigo999/mindspore-skills"
	DefaultRepoBranch        = "refactor-arch-4.1"
	defaultRepoName          = "mindspore-skills"
	defaultSkillsDir         = "skills"
	defaultCommitFile        = ".mscode-commit"
	defaultLogPrefix         = "skills sync"
	defaultHTTPTimeout       = 2 * time.Minute
	defaultRemoteHEADTimeout = 5 * time.Second
	defaultCommandLimit      = 2 * time.Minute
)

// UpdateInfo describes whether a skills repo update is available.
type UpdateInfo struct {
	Available     bool
	LocalVersion  string
	RemoteVersion string
}

// RepoSync manages skills repository sync.
type RepoSync interface {
	Sync() error
}

// RepoSyncConfig controls where the shared skills repo is synced locally.
type RepoSyncConfig struct {
	HomeDir   string
	LogWriter io.Writer
}

// DefaultRepoSync keeps the bundled skills repo fresh under ~/.mscode.
type DefaultRepoSync struct {
	homeDir     string
	repoURL     string
	branch      string
	skipInTests bool
	httpClient  *http.Client
	lookPath    func(file string) (string, error)
	runCommand  func(name string, args ...string) (string, error)
	logWriter   io.Writer
}

// NewDefaultRepoSync creates the default startup syncer for the shared skills repo.
func NewDefaultRepoSync(homeDir string) *DefaultRepoSync {
	return NewRepoSync(RepoSyncConfig{HomeDir: homeDir})
}

// NewRepoSync creates a startup syncer using the provided repo settings.
func NewRepoSync(cfg RepoSyncConfig) *DefaultRepoSync {
	logWriter := cfg.LogWriter
	if logWriter == nil {
		logWriter = os.Stderr
	}
	return &DefaultRepoSync{
		homeDir:     strings.TrimSpace(cfg.HomeDir),
		repoURL:     DefaultRepoURL,
		branch:      DefaultRepoBranch,
		skipInTests: true,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		lookPath:   exec.LookPath,
		runCommand: defaultRunCommand,
		logWriter:  logWriter,
	}
}

// SyncedRepoDir returns the local checkout/download directory for the shared skills repo.
func SyncedRepoDir(homeDir string) string {
	return filepath.Join(strings.TrimSpace(homeDir), ".mscode", defaultRepoName)
}

// SyncedSkillsDir returns the highest-priority skills directory synced at startup.
func SyncedSkillsDir(homeDir string) string {
	return filepath.Join(SyncedRepoDir(homeDir), defaultSkillsDir)
}

// SkillsDir returns the synced skills directory for the receiver.
func (s *DefaultRepoSync) SkillsDir() string {
	return SyncedSkillsDir(s.homeDir)
}

// Sync handles first-time clone of the shared skills repo. If the repo already
// exists locally it returns immediately — update checks are done separately via
// CheckUpdate / ApplyUpdate.
func (s *DefaultRepoSync) Sync() error {
	if strings.TrimSpace(s.homeDir) == "" {
		return fmt.Errorf("home directory is required")
	}
	if s.skipInTests && runningUnderGoTest() {
		return nil
	}

	repoDir := SyncedRepoDir(s.homeDir)
	skillsDir := SyncedSkillsDir(s.homeDir)
	hasGit := s.hasGit()

	s.logf("repo dir: %s", repoDir)
	if hasGit {
		s.logf("git detected")
	} else {
		s.logf("git not detected")
	}

	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		return fmt.Errorf("create skills parent dir: %w", err)
	}

	// If the repo already exists, nothing to do — Sync only handles first-time clone.
	if dirExists(repoDir) {
		s.logf("local repo exists; skipping clone")
		return nil
	}

	s.logf("local repo does not exist")
	if hasGit {
		s.logf("cloning %s@%s", s.repoURL, s.branch)
		if err := s.cloneRepo(repoDir); err != nil {
			return err
		}
		commit, err := s.localGitCommit(repoDir)
		if err != nil {
			return err
		}
		if err := s.writeCommitFile(repoDir, commit); err != nil {
			return err
		}
		s.logf("clone complete at commit %s", ShortCommit(commit))
	} else {
		s.logf("resolving remote commit before archive download")
		remoteCommit, err := s.remoteCommit()
		if err != nil {
			return err
		}
		s.logf("downloading and extracting archive")
		if err := s.downloadArchive(repoDir); err != nil {
			return err
		}
		if err := s.writeCommitFile(repoDir, remoteCommit); err != nil {
			return err
		}
		s.logf("archive install complete at commit %s", ShortCommit(remoteCommit))
	}
	return s.ensureSkillsDir(skillsDir)
}

// ReadVersion reads the VERSION file from the given repo directory.
// Returns "unknown" if the file is missing or empty.
func ReadVersion(repoDir string) string {
	data, err := os.ReadFile(filepath.Join(repoDir, "VERSION"))
	if err != nil {
		return "unknown"
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "unknown"
	}
	return v
}

// CheckRemoteVersion fetches the VERSION file from the remote repo via GitHub raw content URL.
func (s *DefaultRepoSync) CheckRemoteVersion() (string, error) {
	repoPath, err := githubRepoPath(s.repoURL)
	if err != nil {
		return "", err
	}
	rawURL := "https://raw.githubusercontent.com/" + repoPath + "/" + url.PathEscape(s.branch) + "/VERSION"

	ctx, cancel := context.WithTimeout(context.Background(), defaultRemoteHEADTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build remote version request: %w", err)
	}
	req.Header.Set("User-Agent", "mscode")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch remote version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch remote version: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read remote version body: %w", err)
	}
	v := strings.TrimSpace(string(body))
	if v == "" {
		return "", fmt.Errorf("remote VERSION file is empty")
	}
	return v, nil
}

// CheckUpdate compares local and remote VERSION files to determine if an update is available.
func (s *DefaultRepoSync) CheckUpdate() (*UpdateInfo, error) {
	repoDir := SyncedRepoDir(s.homeDir)
	localVersion := ReadVersion(repoDir)

	remoteVersion, err := s.CheckRemoteVersion()
	if err != nil {
		return nil, err
	}

	return &UpdateInfo{
		Available:     localVersion != remoteVersion,
		LocalVersion:  localVersion,
		RemoteVersion: remoteVersion,
	}, nil
}

// ApplyUpdate performs the actual download/pull to bring the local repo up to date.
func (s *DefaultRepoSync) ApplyUpdate() error {
	repoDir := SyncedRepoDir(s.homeDir)
	skillsDir := SyncedSkillsDir(s.homeDir)

	if !dirExists(repoDir) {
		return s.Sync()
	}

	hasGit := s.hasGit()
	gitRepo := dirExists(filepath.Join(repoDir, ".git"))
	switch {
	case hasGit && gitRepo:
		s.logf("updating repo with git pull")
		if err := s.updateWithGitPull(repoDir); err != nil {
			return err
		}
		newCommit, err := s.localGitCommit(repoDir)
		if err != nil {
			return err
		}
		if err := s.writeCommitFile(repoDir, newCommit); err != nil {
			return err
		}
		s.logf("git update complete at commit %s", ShortCommit(newCommit))
	case hasGit:
		s.logf("git is available but local copy has no .git metadata; replacing with a fresh clone")
		if err := s.replaceWithGitClone(repoDir); err != nil {
			return err
		}
		newCommit, err := s.localGitCommit(repoDir)
		if err != nil {
			return err
		}
		if err := s.writeCommitFile(repoDir, newCommit); err != nil {
			return err
		}
		s.logf("clone replacement complete at commit %s", ShortCommit(newCommit))
	default:
		s.logf("resolving remote commit before archive download")
		remoteCommit, err := s.remoteCommit()
		if err != nil {
			return err
		}
		s.logf("refreshing local copy from archive download")
		if err := s.downloadArchive(repoDir); err != nil {
			return err
		}
		if err := s.writeCommitFile(repoDir, remoteCommit); err != nil {
			return err
		}
		s.logf("archive refresh complete at commit %s", ShortCommit(remoteCommit))
	}

	return s.ensureSkillsDir(skillsDir)
}

func (s *DefaultRepoSync) hasGit() bool {
	if s.lookPath == nil {
		return false
	}
	_, err := s.lookPath("git")
	return err == nil
}

func (s *DefaultRepoSync) localCommit(repoDir string) (string, error) {
	if s.hasGit() && dirExists(filepath.Join(repoDir, ".git")) {
		commit, err := s.localGitCommit(repoDir)
		if err == nil && commit != "" {
			return commit, nil
		}
	}
	return s.readCommitFile(repoDir)
}

func (s *DefaultRepoSync) localGitCommit(repoDir string) (string, error) {
	output, err := s.runCommand("git", "-C", repoDir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	commit := strings.TrimSpace(output)
	if commit == "" {
		return "", fmt.Errorf("git rev-parse HEAD returned empty commit")
	}
	return commit, nil
}

func (s *DefaultRepoSync) cloneRepo(repoDir string) error {
	if _, err := s.runCommand("git", "clone", "--branch", s.branch, s.repoURL, repoDir); err != nil {
		return fmt.Errorf("git clone %s@%s: %w", s.repoURL, s.branch, err)
	}
	return nil
}

func (s *DefaultRepoSync) replaceWithGitClone(repoDir string) error {
	tempRoot, cloneDir, err := s.cloneRepoToTemp(filepath.Dir(repoDir))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)

	if err := replaceDir(repoDir, cloneDir); err != nil {
		return fmt.Errorf("replace repo with cloned copy: %w", err)
	}
	return nil
}

func (s *DefaultRepoSync) cloneRepoToTemp(parentDir string) (string, string, error) {
	tempRoot, err := os.MkdirTemp(parentDir, defaultRepoName+"-clone-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	cloneDir := filepath.Join(tempRoot, defaultRepoName)
	if _, err := s.runCommand("git", "clone", "--branch", s.branch, s.repoURL, cloneDir); err != nil {
		_ = os.RemoveAll(tempRoot)
		return "", "", fmt.Errorf("git clone %s@%s: %w", s.repoURL, s.branch, err)
	}
	return tempRoot, cloneDir, nil
}

func (s *DefaultRepoSync) updateWithGitPull(repoDir string) error {
	if err := s.ensureOrigin(repoDir); err != nil {
		return err
	}
	if _, err := s.runCommand("git", "-C", repoDir, "fetch", "origin", s.branch); err != nil {
		return fmt.Errorf("git fetch %s: %w", s.branch, err)
	}
	if err := s.checkoutBranch(repoDir); err != nil {
		return err
	}
	if _, err := s.runCommand("git", "-C", repoDir, "pull", "--ff-only", "origin", s.branch); err != nil {
		return fmt.Errorf("git pull %s: %w", s.branch, err)
	}
	return nil
}

func (s *DefaultRepoSync) ensureOrigin(repoDir string) error {
	if _, err := s.runCommand("git", "-C", repoDir, "remote", "set-url", "origin", s.repoURL); err == nil {
		return nil
	}
	if _, err := s.runCommand("git", "-C", repoDir, "remote", "add", "origin", s.repoURL); err != nil {
		return fmt.Errorf("configure git origin: %w", err)
	}
	return nil
}

func (s *DefaultRepoSync) checkoutBranch(repoDir string) error {
	if _, err := s.runCommand("git", "-C", repoDir, "checkout", s.branch); err == nil {
		return nil
	}
	if _, err := s.runCommand("git", "-C", repoDir, "checkout", "-b", s.branch, "--track", "origin/"+s.branch); err != nil {
		return fmt.Errorf("git checkout %s: %w", s.branch, err)
	}
	return nil
}

func (s *DefaultRepoSync) remoteCommit() (string, error) {
	apiURL, err := githubRefAPIURL(s.repoURL, s.branch)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRemoteHEADTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("build remote commit request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mscode")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch remote commit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch remote commit: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode remote commit: %w", err)
	}

	commit := strings.TrimSpace(payload.Object.SHA)
	if commit == "" {
		return "", fmt.Errorf("remote commit response did not contain sha")
	}
	return commit, nil
}

func (s *DefaultRepoSync) downloadArchive(repoDir string) error {
	tempRoot, extractDir, err := s.downloadArchiveToTemp(filepath.Dir(repoDir))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)

	if err := replaceDir(repoDir, extractDir); err != nil {
		return fmt.Errorf("install downloaded skills repo: %w", err)
	}
	return nil
}

func (s *DefaultRepoSync) downloadArchiveToTemp(parentDir string) (string, string, error) {
	archiveURL, err := archiveURL(s.repoURL, s.branch)
	if err != nil {
		return "", "", err
	}

	resp, err := s.httpClient.Get(archiveURL)
	if err != nil {
		return "", "", fmt.Errorf("download skills archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download skills archive: unexpected status %d", resp.StatusCode)
	}

	tempRoot, err := os.MkdirTemp(parentDir, defaultRepoName+"-download-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	extractDir := filepath.Join(tempRoot, defaultRepoName)
	if err := extractTarGz(resp.Body, extractDir); err != nil {
		_ = os.RemoveAll(tempRoot)
		return "", "", fmt.Errorf("extract skills archive: %w", err)
	}
	return tempRoot, extractDir, nil
}

func (s *DefaultRepoSync) readCommitFile(repoDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoDir, defaultCommitFile))
	if err != nil {
		return "", fmt.Errorf("read commit file: %w", err)
	}
	commit := strings.TrimSpace(string(data))
	if commit == "" {
		return "", fmt.Errorf("commit file is empty")
	}
	return commit, nil
}

func (s *DefaultRepoSync) writeCommitFile(repoDir, commit string) error {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return fmt.Errorf("commit id is empty")
	}
	path := filepath.Join(repoDir, defaultCommitFile)
	if err := os.WriteFile(path, []byte(commit+"\n"), 0o644); err != nil {
		return fmt.Errorf("write commit file: %w", err)
	}
	s.logf("stored commit id in %s", path)
	return nil
}

func (s *DefaultRepoSync) ensureSkillsDir(skillsDir string) error {
	if !dirExists(skillsDir) {
		return fmt.Errorf("skills dir not found after sync: %s", skillsDir)
	}
	return nil
}

func (s *DefaultRepoSync) logf(format string, args ...any) {
	if s.logWriter == nil {
		return
	}
	fmt.Fprintf(s.logWriter, "%s: %s\n", defaultLogPrefix, fmt.Sprintf(format, args...))
}

func githubRefAPIURL(repoURL, branch string) (string, error) {
	repoPath, err := githubRepoPath(repoURL)
	if err != nil {
		return "", err
	}
	return "https://api.github.com/repos/" + repoPath + "/git/ref/heads/" + url.PathEscape(branch), nil
}

func githubRepoPath(repoURL string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimSuffix(repoURL, "/")

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parse repo url: %w", err)
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return "", fmt.Errorf("unsupported skills repo url: %s", repoURL)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("unsupported skills repo url: %s", repoURL)
	}
	return parts[0] + "/" + parts[1], nil
}

func archiveURL(repoURL, branch string) (string, error) {
	repoPath, err := githubRepoPath(repoURL)
	if err != nil {
		return "", err
	}
	return "https://codeload.github.com/" + repoPath + "/tar.gz/refs/heads/" + url.PathEscape(branch), nil
}

func extractTarGz(src io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		relPath := stripArchiveRoot(hdr.Name)
		if relPath == "" {
			continue
		}

		targetPath, err := safeJoin(destDir, relPath)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
}

func replaceDir(targetDir, newDir string) error {
	if !dirExists(targetDir) {
		return os.Rename(newDir, targetDir)
	}

	backupDir := targetDir + ".bak"
	_ = os.RemoveAll(backupDir)
	if err := os.Rename(targetDir, backupDir); err != nil {
		return fmt.Errorf("move current repo to backup: %w", err)
	}
	if err := os.Rename(newDir, targetDir); err != nil {
		_ = os.Rename(backupDir, targetDir)
		return fmt.Errorf("move new repo into place: %w", err)
	}
	_ = os.RemoveAll(backupDir)
	return nil
}

func stripArchiveRoot(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "./")
	if name == "" {
		return ""
	}
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimPrefix(parts[1], "/")
}

func safeJoin(rootDir, relPath string) (string, error) {
	rootDir = filepath.Clean(rootDir)
	relPath = filepath.Clean(relPath)
	targetPath := filepath.Join(rootDir, relPath)
	if targetPath == rootDir {
		return targetPath, nil
	}
	prefix := rootDir + string(os.PathSeparator)
	if !strings.HasPrefix(targetPath, prefix) {
		return "", fmt.Errorf("archive path escapes destination: %s", relPath)
	}
	return targetPath, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func runningUnderGoTest() bool {
	return flag.Lookup("test.v") != nil || flag.Lookup("test.run") != nil
}

func ShortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "unknown"
	}
	if len(commit) <= 12 {
		return commit
	}
	return commit[:12]
}

func defaultRunCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCommandLimit)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, text)
	}
	return text, nil
}
