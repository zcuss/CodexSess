package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ricki/codexsess/internal/store"
)

type selfHealConfig struct {
	Enabled      bool   `json:"enabled"`
	OnError      bool   `json:"on_error"`
	Command      string `json:"command"`
	AutoPush     bool   `json:"auto_push"`
	GitRemote    string `json:"git_remote"`
	GitBranch    string `json:"git_branch"`
	CommitPrefix string `json:"commit_prefix"`
	HasGitHubTok bool   `json:"has_github_token"`
	GitHubToken  string `json:"-"`
}

func defaultSelfHealConfig() selfHealConfig {
	return selfHealConfig{
		Enabled:      false,
		OnError:      true,
		Command:      "",
		AutoPush:     true,
		GitRemote:    "origin",
		GitBranch:    "main",
		CommitPrefix: "self-heal",
	}
}

func (s *Server) currentSelfHealConfig(ctx context.Context) selfHealConfig {
	cfg := defaultSelfHealConfig()
	cfg.Enabled = s.readBoolSetting(ctx, store.SettingSelfHealEnabled, cfg.Enabled)
	cfg.OnError = s.readBoolSetting(ctx, store.SettingSelfHealOnError, cfg.OnError)
	cfg.Command = strings.TrimSpace(s.readStringSetting(ctx, store.SettingSelfHealCommand, cfg.Command))
	cfg.AutoPush = s.readBoolSetting(ctx, store.SettingSelfHealAutoPush, cfg.AutoPush)
	cfg.GitRemote = firstNonEmpty(strings.TrimSpace(s.readStringSetting(ctx, store.SettingSelfHealGitRemote, cfg.GitRemote)), cfg.GitRemote)
	cfg.GitBranch = firstNonEmpty(strings.TrimSpace(s.readStringSetting(ctx, store.SettingSelfHealGitBranch, cfg.GitBranch)), cfg.GitBranch)
	cfg.CommitPrefix = firstNonEmpty(strings.TrimSpace(s.readStringSetting(ctx, store.SettingSelfHealCommitPrefix, cfg.CommitPrefix)), cfg.CommitPrefix)
	cfg.GitHubToken = strings.TrimSpace(s.readStringSetting(ctx, store.SettingSelfHealGitHubToken, ""))
	cfg.HasGitHubTok = cfg.GitHubToken != ""
	return cfg
}

func (s *Server) saveSelfHealConfig(ctx context.Context, cfg selfHealConfig) error {
	pairs := map[string]string{
		store.SettingSelfHealEnabled:      boolString(cfg.Enabled),
		store.SettingSelfHealOnError:      boolString(cfg.OnError),
		store.SettingSelfHealCommand:      strings.TrimSpace(cfg.Command),
		store.SettingSelfHealAutoPush:     boolString(cfg.AutoPush),
		store.SettingSelfHealGitRemote:    firstNonEmpty(strings.TrimSpace(cfg.GitRemote), "origin"),
		store.SettingSelfHealGitBranch:    firstNonEmpty(strings.TrimSpace(cfg.GitBranch), "main"),
		store.SettingSelfHealCommitPrefix: firstNonEmpty(strings.TrimSpace(cfg.CommitPrefix), "self-heal"),
		store.SettingSelfHealGitHubToken:  strings.TrimSpace(cfg.GitHubToken),
	}
	for k, v := range pairs {
		if err := s.saveSetting(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) runSelfHealLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		cfg := s.currentSelfHealConfig(context.Background())
		if !cfg.Enabled || !cfg.OnError || strings.TrimSpace(cfg.Command) == "" {
			continue
		}
		if !s.selfHealAcquireRunSlot() {
			continue
		}
		logID, reason := s.findLatestErrorSignal(context.Background())
		if strings.TrimSpace(logID) == "" {
			continue
		}
		if !s.selfHealMarkIfNew(logID) {
			continue
		}
		runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		_, err := s.executeSelfHeal(runCtx, cfg, reason)
		cancel()
		if err != nil {
			log.Printf("[self-heal] failed: %v", err)
			s.svc.AddSystemLog(context.Background(), "self_heal", "Self-heal failed", map[string]any{
				"error":  err.Error(),
				"reason": reason,
			})
			continue
		}
		s.svc.AddSystemLog(context.Background(), "self_heal", "Self-heal completed", map[string]any{
			"reason": reason,
		})
	}
}

func (s *Server) selfHealAcquireRunSlot() bool {
	s.selfHealMu.Lock()
	defer s.selfHealMu.Unlock()
	if time.Since(s.selfHealLastRun) < 90*time.Second {
		return false
	}
	s.selfHealLastRun = time.Now()
	return true
}

func (s *Server) selfHealMarkIfNew(logID string) bool {
	s.selfHealMu.Lock()
	defer s.selfHealMu.Unlock()
	if s.selfHealLastHandledID == logID {
		return false
	}
	s.selfHealLastHandledID = logID
	return true
}

func (s *Server) findLatestErrorSignal(ctx context.Context) (string, string) {
	if s == nil || s.svc == nil || s.svc.Store == nil {
		return "", ""
	}
	rows, err := s.svc.Store.ListSystemLogs(ctx, 100)
	if err != nil {
		return "", ""
	}
	for i := 0; i < len(rows); i++ {
		item := rows[i]
		text := strings.ToLower(strings.TrimSpace(item.Message + " " + item.Kind))
		if strings.Contains(text, "error") || strings.Contains(text, "failed") || strings.Contains(text, "panic") {
			return strings.TrimSpace(item.ID), strings.TrimSpace(item.Message)
		}
	}
	return "", ""
}

func (s *Server) executeSelfHeal(ctx context.Context, cfg selfHealConfig, reason string) (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if err := runPlatformShell(ctx, root, cfg.Command); err != nil {
		return "", fmt.Errorf("self-heal command failed: %w", err)
	}
	if err := runCommand(ctx, root, "go", "build", "-o", "codexsess.exe", "."); err != nil {
		return "", fmt.Errorf("go build failed: %w", err)
	}
	webDir := filepath.Join(root, "web")
	if st, err := os.Stat(webDir); err == nil && st.IsDir() {
		if err := runCommand(ctx, webDir, "npm", "run", "build"); err != nil {
			return "", fmt.Errorf("web build failed: %w", err)
		}
	}
	status, err := runCommandOut(ctx, root, "git", "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "no_changes", nil
	}
	_ = runCommand(ctx, root, "git", "add", "-A")
	prefix := firstNonEmpty(strings.TrimSpace(cfg.CommitPrefix), "self-heal")
	msg := fmt.Sprintf("%s: auto-fix (%s)", prefix, firstNonEmpty(strings.TrimSpace(reason), "runtime error"))
	if err := runCommand(ctx, root, "git", "commit", "-m", msg); err != nil {
		after, _ := runCommandOut(ctx, root, "git", "status", "--porcelain")
		if strings.TrimSpace(after) != "" {
			return "", fmt.Errorf("git commit failed: %w", err)
		}
	}
	if cfg.AutoPush {
		remote := firstNonEmpty(strings.TrimSpace(cfg.GitRemote), "origin")
		branch := firstNonEmpty(strings.TrimSpace(cfg.GitBranch), "main")
		if err := runGitPush(ctx, root, remote, branch, strings.TrimSpace(cfg.GitHubToken)); err != nil {
			return "", fmt.Errorf("git push failed: %w", err)
		}
	}
	return "pushed", nil
}

func runGitPush(ctx context.Context, workdir, remote, branch, githubToken string) error {
	args := []string{"push", remote, "HEAD:" + branch}
	token := strings.TrimSpace(githubToken)
	if token == "" {
		return runCommand(ctx, workdir, "git", args...)
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	env := append(os.Environ(), "GIT_HTTP_EXTRAHEADER=AUTHORIZATION: basic "+basic)
	return runCommandEnv(ctx, workdir, env, "git", args...)
}

func runGitForcePush(ctx context.Context, workdir, remote, branch, githubToken string) (string, error) {
	args := []string{"push", "--force-with-lease", remote, "HEAD:" + branch}
	token := strings.TrimSpace(githubToken)
	if token == "" {
		return runCommandOut(ctx, workdir, "git", args...)
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	env := append(os.Environ(), "GIT_HTTP_EXTRAHEADER=AUTHORIZATION: basic "+basic)
	return runCommandEnvOut(ctx, workdir, env, "git", args...)
}

func runGitPushDryRun(ctx context.Context, workdir, remote, branch, githubToken string) (string, error) {
	args := []string{"push", "--dry-run", remote, "HEAD:" + branch}
	token := strings.TrimSpace(githubToken)
	if token == "" {
		return runCommandOut(ctx, workdir, "git", args...)
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	env := append(os.Environ(), "GIT_HTTP_EXTRAHEADER=AUTHORIZATION: basic "+basic)
	return runCommandEnvOut(ctx, workdir, env, "git", args...)
}

func gitRemoteURL(ctx context.Context, workdir, remote string) (string, error) {
	return runCommandOut(ctx, workdir, "git", "remote", "get-url", strings.TrimSpace(remote))
}

func setGitRemoteURL(ctx context.Context, workdir, remote, url string) error {
	name := firstNonEmpty(strings.TrimSpace(remote), "origin")
	target := strings.TrimSpace(url)
	if target == "" {
		return fmt.Errorf("git remote url is empty")
	}
	_, err := runCommandOut(ctx, workdir, "git", "remote", "get-url", name)
	if err != nil {
		return runCommand(ctx, workdir, "git", "remote", "add", name, target)
	}
	return runCommand(ctx, workdir, "git", "remote", "set-url", name, target)
}

func (s *Server) handleWebSelfHealGitRemote(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cfg := s.currentSelfHealConfig(r.Context())
		root, err := os.Getwd()
		if err != nil {
			respondErr(w, 500, "internal_error", err.Error())
			return
		}
		url, _ := gitRemoteURL(r.Context(), root, cfg.GitRemote)
		respondJSON(w, 200, map[string]any{
			"ok":      true,
			"remote":  cfg.GitRemote,
			"url":     strings.TrimSpace(url),
			"branch":  cfg.GitBranch,
			"has_pat": cfg.HasGitHubTok,
		})
		return
	case "POST":
		var req struct {
			Remote string `json:"remote"`
			URL    string `json:"url"`
			Branch string `json:"branch"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		cfg := s.currentSelfHealConfig(r.Context())
		if strings.TrimSpace(req.Remote) != "" {
			cfg.GitRemote = strings.TrimSpace(req.Remote)
		}
		if strings.TrimSpace(req.Branch) != "" {
			cfg.GitBranch = strings.TrimSpace(req.Branch)
		}
		root, err := os.Getwd()
		if err != nil {
			respondErr(w, 500, "internal_error", err.Error())
			return
		}
		if strings.TrimSpace(req.URL) != "" {
			if err := setGitRemoteURL(r.Context(), root, cfg.GitRemote, req.URL); err != nil {
				respondErr(w, 400, "bad_request", err.Error())
				return
			}
		}
		if err := s.saveSelfHealConfig(r.Context(), cfg); err != nil {
			respondErr(w, 500, "internal_error", err.Error())
			return
		}
		url, _ := gitRemoteURL(r.Context(), root, cfg.GitRemote)
		respondJSON(w, 200, map[string]any{
			"ok":      true,
			"remote":  cfg.GitRemote,
			"url":     strings.TrimSpace(url),
			"branch":  cfg.GitBranch,
			"has_pat": cfg.HasGitHubTok,
		})
		return
	default:
		respondErr(w, 405, "method_not_allowed", "method not allowed")
		return
	}
}

func (s *Server) handleWebSelfHealTestPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondErr(w, 405, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentSelfHealConfig(r.Context())
	root, err := os.Getwd()
	if err != nil {
		respondErr(w, 500, "internal_error", err.Error())
		return
	}
	remote := firstNonEmpty(strings.TrimSpace(cfg.GitRemote), "origin")
	branch := firstNonEmpty(strings.TrimSpace(cfg.GitBranch), "main")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	remoteOut, remoteErr := runCommandOut(ctx, root, "git", "ls-remote", "--heads", remote)
	pushOut, pushErr := runGitPushDryRun(ctx, root, remote, branch, strings.TrimSpace(cfg.GitHubToken))
	branchRef := "refs/heads/" + branch
	branchExists := strings.Contains(remoteOut, branchRef)
	if remoteErr != nil {
		respondJSON(w, 200, map[string]any{
			"ok":         false,
			"step":       "remote_probe",
			"error":      remoteErr.Error(),
			"remote_out": strings.TrimSpace(remoteOut),
		})
		return
	}
	if pushErr != nil {
		respondJSON(w, 200, map[string]any{
			"ok":            false,
			"step":          "push_dry_run",
			"error":         pushErr.Error(),
			"remote_out":    strings.TrimSpace(remoteOut),
			"push_out":      strings.TrimSpace(pushOut),
			"branch_exists": branchExists,
		})
		return
	}
	respondJSON(w, 200, map[string]any{
		"ok":            true,
		"remote_out":    strings.TrimSpace(remoteOut),
		"push_out":      strings.TrimSpace(pushOut),
		"remote":        remote,
		"branch":        branch,
		"branch_exists": branchExists,
	})
}

func (s *Server) handleWebSelfHealSyncRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondErr(w, 405, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentSelfHealConfig(r.Context())
	remote := firstNonEmpty(strings.TrimSpace(cfg.GitRemote), "origin")
	branch := firstNonEmpty(strings.TrimSpace(cfg.GitBranch), "main")
	root, err := os.Getwd()
	if err != nil {
		respondErr(w, 500, "internal_error", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	statusOut, statusErr := runCommandOut(ctx, root, "git", "status", "--porcelain")
	if statusErr != nil {
		respondJSON(w, 200, map[string]any{
			"ok":         false,
			"step":       "status",
			"error":      statusErr.Error(),
			"status_out": strings.TrimSpace(statusOut),
			"remote":     remote,
			"branch":     branch,
		})
		return
	}
	if strings.TrimSpace(statusOut) != "" {
		respondJSON(w, 200, map[string]any{
			"ok":         false,
			"step":       "dirty_worktree",
			"error":      "working tree has local changes; commit/stash first",
			"status_out": strings.TrimSpace(statusOut),
			"remote":     remote,
			"branch":     branch,
		})
		return
	}
	remoteOut, remoteErr := runCommandOut(ctx, root, "git", "ls-remote", "--heads", remote, branch)
	if remoteErr != nil {
		respondJSON(w, 200, map[string]any{
			"ok":         false,
			"step":       "remote_probe",
			"error":      remoteErr.Error(),
			"remote_out": strings.TrimSpace(remoteOut),
			"remote":     remote,
			"branch":     branch,
		})
		return
	}
	fetchOut, fetchErr := runCommandOut(ctx, root, "git", "fetch", remote, branch)
	if fetchErr != nil {
		respondJSON(w, 200, map[string]any{
			"ok":        false,
			"step":      "fetch",
			"error":     fetchErr.Error(),
			"fetch_out": strings.TrimSpace(fetchOut),
			"remote":    remote,
			"branch":    branch,
		})
		return
	}
	localHead, localErr := runCommandOut(ctx, root, "git", "rev-parse", "HEAD")
	remoteHead, remoteHeadErr := runCommandOut(ctx, root, "git", "rev-parse", "FETCH_HEAD")
	if localErr != nil || remoteHeadErr != nil {
		respondJSON(w, 200, map[string]any{
			"ok":          false,
			"step":        "rev_parse",
			"error":       firstNonEmpty(errString(localErr), errString(remoteHeadErr)),
			"local_head":  strings.TrimSpace(localHead),
			"remote_head": strings.TrimSpace(remoteHead),
			"remote":      remote,
			"branch":      branch,
		})
		return
	}
	localHead = strings.TrimSpace(localHead)
	remoteHead = strings.TrimSpace(remoteHead)
	if localHead == remoteHead {
		respondJSON(w, 200, map[string]any{
			"ok":          true,
			"step":        "up_to_date",
			"remote":      remote,
			"branch":      branch,
			"local_head":  localHead,
			"remote_head": remoteHead,
			"fetch_out":   strings.TrimSpace(fetchOut),
		})
		return
	}
	behind := runCommand(ctx, root, "git", "merge-base", "--is-ancestor", "HEAD", "FETCH_HEAD") == nil
	ahead := runCommand(ctx, root, "git", "merge-base", "--is-ancestor", "FETCH_HEAD", "HEAD") == nil
	if behind {
		ffOut, ffErr := runCommandOut(ctx, root, "git", "merge", "--ff-only", "FETCH_HEAD")
		respondJSON(w, 200, map[string]any{
			"ok":          ffErr == nil,
			"step":        "fast_forward",
			"remote":      remote,
			"branch":      branch,
			"local_head":  localHead,
			"remote_head": remoteHead,
			"fetch_out":   strings.TrimSpace(fetchOut),
			"merge_out":   strings.TrimSpace(ffOut),
			"merge_err":   errString(ffErr),
		})
		return
	}
	if ahead {
		respondJSON(w, 200, map[string]any{
			"ok":          true,
			"step":        "already_ahead",
			"remote":      remote,
			"branch":      branch,
			"local_head":  localHead,
			"remote_head": remoteHead,
			"fetch_out":   strings.TrimSpace(fetchOut),
		})
		return
	}
	respondJSON(w, 200, map[string]any{
		"ok":          false,
		"step":        "diverged",
		"error":       "local and remote histories diverged; cannot auto-sync safely",
		"remote":      remote,
		"branch":      branch,
		"local_head":  localHead,
		"remote_head": remoteHead,
		"fetch_out":   strings.TrimSpace(fetchOut),
		"remote_out":  strings.TrimSpace(remoteOut),
	})
}

func (s *Server) handleWebSelfHealForcePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondErr(w, 405, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentSelfHealConfig(r.Context())
	remote := firstNonEmpty(strings.TrimSpace(cfg.GitRemote), "origin")
	branch := firstNonEmpty(strings.TrimSpace(cfg.GitBranch), "main")
	root, err := os.Getwd()
	if err != nil {
		respondErr(w, 500, "internal_error", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	out, pushErr := runGitForcePush(ctx, root, remote, branch, strings.TrimSpace(cfg.GitHubToken))
	respondJSON(w, 200, map[string]any{
		"ok":      pushErr == nil,
		"remote":  remote,
		"branch":  branch,
		"output":  strings.TrimSpace(out),
		"error":   errString(pushErr),
		"forced":  true,
		"warning": "force push can overwrite remote history",
	})
}

func runPlatformShell(ctx context.Context, workdir string, script string) error {
	cmd := strings.TrimSpace(script)
	if cmd == "" {
		return nil
	}
	if runtime.GOOS == "windows" {
		return runCommand(ctx, workdir, "powershell", "-NoProfile", "-Command", cmd)
	}
	return runCommand(ctx, workdir, "sh", "-lc", cmd)
}

func runCommand(ctx context.Context, workdir string, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = workdir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, truncateSelfHealOutput(out.String()))
	}
	return nil
}

func runCommandEnv(ctx context.Context, workdir string, env []string, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = workdir
	c.Env = env
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, truncateSelfHealOutput(out.String()))
	}
	return nil
}

func runCommandEnvOut(ctx context.Context, workdir string, env []string, name string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = workdir
	c.Env = env
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	if err := c.Run(); err != nil {
		return out.String(), fmt.Errorf("%s %v: %w: %s", name, args, err, truncateSelfHealOutput(out.String()))
	}
	return out.String(), nil
}

func runCommandOut(ctx context.Context, workdir string, name string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = workdir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", name, args, err, truncateSelfHealOutput(out.String()))
	}
	return out.String(), nil
}

func truncateSelfHealOutput(v string) string {
	s := strings.TrimSpace(v)
	if len(s) <= 1500 {
		return s
	}
	return s[:1500]
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
