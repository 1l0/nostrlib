package grasp

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip34"

	"fiatjaf.com/nostr/khatru"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
)

const zeroRef = "0000000000000000000000000000000000000000"

var asciiPattern = regexp.MustCompile(`^[\w-.]+$`)

type GraspServer struct {
	ServiceURL    string
	RepositoryDir string

	Relay *khatru.Relay
}

// New creates a new GraspServer and registers its handlers on the relay's router
func New(rl *khatru.Relay, repositoryDir string) *GraspServer {
	gs := &GraspServer{
		Relay:         rl,
		RepositoryDir: repositoryDir,
	}

	base := rl.Router()
	mux := http.NewServeMux()

	// use specific route patterns for git endpoints
	mux.HandleFunc("GET /{npub}/{repo}/info/refs", func(w http.ResponseWriter, r *http.Request) {
		gs.handleGitRequest(w, r, base, func(w http.ResponseWriter, r *http.Request, pubkey nostr.PubKey, repoName string) {
			gs.handleInfoRefs(w, r, pubkey, repoName)
		})
	})

	mux.HandleFunc("POST /{npub}/{repo}/git-upload-pack", func(w http.ResponseWriter, r *http.Request) {
		gs.handleGitRequest(w, r, base, func(w http.ResponseWriter, r *http.Request, pubkey nostr.PubKey, repoName string) {
			gs.handleGitUploadPack(w, r, pubkey, repoName)
		})
	})

	mux.HandleFunc("POST /{npub}/{repo}/git-receive-pack", func(w http.ResponseWriter, r *http.Request) {
		gs.handleGitRequest(w, r, base, func(w http.ResponseWriter, r *http.Request, pubkey nostr.PubKey, repoName string) {
			gs.handleGitReceivePack(w, r, pubkey, repoName)
		})
	})

	mux.HandleFunc("GET /{npub}/{repo}", func(w http.ResponseWriter, r *http.Request) {
		gs.handleGitRequest(w, r, base, func(w http.ResponseWriter, r *http.Request, pubkey nostr.PubKey, repoName string) {
			if r.URL.RawQuery == "" {
				if gs.repoExists(pubkey, repoName) {
					gs.serveRepoPage(w, r, r.PathValue("npub"), repoName)
				} else {
					http.NotFound(w, r)
				}
			} else {
				base.ServeHTTP(w, r)
			}
		})
	})

	// fallback handler for all other paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		base.ServeHTTP(w, r)
	})

	rl.SetRouter(mux)

	return gs
}

// handleGitRequest validates .git suffix and decodes npub, then calls the handler
func (gs *GraspServer) handleGitRequest(
	w http.ResponseWriter,
	r *http.Request,
	base http.Handler,
	handler func(http.ResponseWriter,
		*http.Request,
		nostr.PubKey,
		string,
	),
) {
	npub := r.PathValue("npub")
	repoWithGit := r.PathValue("repo")

	// validate .git suffix
	if !strings.HasSuffix(repoWithGit, ".git") {
		base.ServeHTTP(w, r)
		return
	}

	repoName := strings.TrimSuffix(repoWithGit, ".git")

	// validate repo name
	if !asciiPattern.MatchString(repoName) {
		http.Error(w, "invalid repository name", http.StatusBadRequest)
		return
	}

	// decode npub to pubkey
	_, value, err := nip19.Decode(npub)
	if err != nil {
		http.Error(w, "invalid npub", http.StatusBadRequest)
		return
	}
	pk, ok := value.(nostr.PubKey)
	if !ok {
		http.Error(w, "invalid npub", http.StatusBadRequest)
		return
	}

	handler(w, r, pk, repoName)
}

// handleInfoRefs handles the git info/refs endpoint
func (gs *GraspServer) handleInfoRefs(
	w http.ResponseWriter,
	r *http.Request,
	pubkey nostr.PubKey,
	repoName string,
) {
	serviceName := r.URL.Query().Get("service")

	switch serviceName {
	case "git-upload-pack":
		if !gs.repoExists(pubkey, repoName) {
			gs.gitError(w, "repository not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		w.Header().Set("Connection", "Keep-Alive")
		w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
		w.WriteHeader(http.StatusOK)

		repoPath := filepath.Join(gs.RepositoryDir, repoName)
		if err := gs.runInfoRefs(w, r, repoPath); err != nil {
			fmt.Printf("runInfoRefs error: %s\n", err)
			return
		}
	case "git-receive-pack":
		// for receive-pack on non-existent repos, send fake advertisement to allow initial push
		if !gs.repoExists(pubkey, repoName) {
			w.Header().Set("content-type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "couldn't find the specified repository '%s' for '%s', you must publish its NIP-34 events here first\n", repoName, pubkey.Hex())
			return
		}
		w.Header().Set("content-type", "application/x-git-receive-pack-advertisement")
		v, _ := base64.StdEncoding.DecodeString("MDAxZiMgc2VydmljZT1naXQtcmVjZWl2ZS1wYWNrCjAwMDAwMGIxMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMCBjYXBhYmlsaXRpZXNee30AcmVwb3J0LXN0YXR1cyByZXBvcnQtc3RhdHVzLXYyIGRlbGV0ZS1yZWZzIHNpZGUtYmFuZC02NGsgcXVpZXQgYXRvbWljIG9mcy1kZWx0YSBvYmplY3QtZm9ybWF0PXNoYTEgYWdlbnQ9Z2l0LzIuNDMuMAowMDAw")
		w.Write(v)
	default:
		gs.gitError(w, fmt.Sprintf("service unsupported: '%s'", serviceName), http.StatusForbidden)
	}
}

func (gs *GraspServer) handleGitUploadPack(
	w http.ResponseWriter,
	r *http.Request,
	pubkey nostr.PubKey,
	repoName string,
) {
	repoPath := filepath.Join(gs.RepositoryDir, repoName)

	// for upload-pack (pull), check if repository exists
	if !gs.repoExists(pubkey, repoName) {
		gs.gitError(w, "repository not found", http.StatusNotFound)
		return
	}

	const expectedContentType = "application/x-git-upload-pack-request"
	contentType := r.Header.Get("Content-Type")
	if contentType != expectedContentType {
		gs.gitError(w, fmt.Sprintf("expected Content-Type: '%s', but received '%s'", expectedContentType, contentType), http.StatusUnsupportedMediaType)
		return
	}

	var bodyReader io.ReadCloser = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			gs.gitError(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("git: failed to create gzip reader, handler: UploadPack, error: %v\n", err)
			return
		}
		defer gzipReader.Close()
		bodyReader = gzipReader
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	w.WriteHeader(http.StatusOK)

	fmt.Printf("git: executing git-upload-pack, handler: UploadPack, repo: %s\n", repoPath)

	if err := gs.runUploadPack(w, r, repoPath, bodyReader); err != nil {
		fmt.Printf("git: failed to execute git-upload-pack, handler: UploadPack, error: %v\n", err)
		return
	}
}

func (gs *GraspServer) handleGitReceivePack(
	w http.ResponseWriter,
	r *http.Request,
	pubkey nostr.PubKey,
	repoName string,
) {
	// for receive-pack (push), validate authorization via NIP-34 events
	body := &bytes.Buffer{}
	io.Copy(body, r.Body)

	if !gs.validatePush(r.Context(), pubkey, repoName, body.Bytes()) {
		gs.gitError(w, "unauthorized push", http.StatusForbidden)
		return
	}

	repoPath := filepath.Join(gs.RepositoryDir, repoName)

	// ensure repository directory exists
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		gs.gitError(w, fmt.Sprintf("failed to create repository: %s", err), http.StatusInternalServerError)
		return
	}

	// initialize git repo if .git doesn't exist
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		cmd := exec.Command("git", "init", "--bare")
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			gs.gitError(w, fmt.Sprintf("failed to initialize repository: %s, output: %s", err, string(output)), http.StatusInternalServerError)
			return
		}
		fmt.Printf("initialized new git repository at %s\n", repoPath)
	}

	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	w.WriteHeader(http.StatusOK)

	if err := gs.runReceivePack(w, r, repoPath, io.NopCloser(bytes.NewReader(body.Bytes()))); err != nil {
		fmt.Printf("runReceivePack error: %s\n", err)
		return
	}

	// update HEAD per state announcement
	if err := gs.updateHEAD(r.Context(), pubkey, repoName, repoPath); err != nil {
		fmt.Printf("failed to update HEAD: %s\n", err)
		// don't fail the push, just log
	}

	// cleanup merged patches
	go gs.cleanupMergedPatches(r.Context(), pubkey, repoName, repoPath)
}

// validatePush checks if a push is authorized via NIP-34 repository state events
func (gs *GraspServer) validatePush(
	ctx context.Context,
	pubkey nostr.PubKey,
	repoName string,
	bodyBytes []byte,
) bool {
	// query for repository state events (kind 30618)
	if gs.Relay.QueryStored == nil {
		fmt.Printf("relay has no QueryStored function\n")
		return false
	}

	// check state
	var state nip34.RepositoryState
	for evt := range gs.Relay.QueryStored(ctx, nostr.Filter{
		Kinds:   []nostr.Kind{nostr.KindRepositoryState},
		Authors: []nostr.PubKey{pubkey},
		Tags:    nostr.TagMap{"d": []string{repoName}},
		Limit:   1,
	}) {
		state = nip34.ParseRepositoryState(evt)
	}
	if state.Event.ID == nostr.ZeroID {
		return false
	}

	// get repository announcement to check maintainers
	var announcement nip34.Repository
	for evt := range gs.Relay.QueryStored(ctx, nostr.Filter{
		Kinds:   []nostr.Kind{nostr.KindRepositoryAnnouncement},
		Authors: []nostr.PubKey{pubkey},
		Tags:    nostr.TagMap{"d": []string{repoName}},
		Limit:   1,
	}) {
		announcement = nip34.ParseRepository(evt)
	}
	if announcement.Event.ID == nostr.ZeroID {
		return false
	}

	// ensure pusher is authorized (owner or maintainer)
	if pubkey != announcement.PubKey && !slices.Contains(announcement.Maintainers, pubkey) {
		return false
	}

	// parse pktline to extract and validate all push refs
	pkt := pktline.NewScanner(bytes.NewReader(bodyBytes))
	for pkt.Scan() {
		if err := pkt.Err(); err != nil {
			fmt.Printf("invalid pkt: %v\n", err)
			return false
		}
		line := string(pkt.Bytes())
		if len(line) < 40 {
			continue
		}

		spl := strings.Split(line, " ")
		from := spl[0]
		to := spl[1]
		ref := strings.TrimRight(spl[2], "\x00")

		// handle refs/nostr/<event-id> pushes
		if strings.HasPrefix(ref, "refs/nostr/") {
			// query for the event
			eventId := ref[11:]
			id, err := nostr.IDFromHex(eventId)
			if err != nil {
				fmt.Printf("push rejected: invalid event id %s\n", eventId)
				return false
			}
			var foundEvent bool
			for evt := range gs.Relay.QueryStored(ctx, nostr.Filter{
				IDs: []nostr.ID{id},
			}) {
				// check if event has a "c" tag matching the commit
				hasMatchingCommit := false
				for _, tag := range evt.Tags {
					if tag[0] == "c" && len(tag) > 1 && tag[1] == to {
						hasMatchingCommit = true
						break
					}
				}
				if !hasMatchingCommit {
					fmt.Printf("push rejected: event %s has different tip (expected %s)\n", eventId, to)
					return false
				}
				foundEvent = true
				break
			}
			if !foundEvent {
				fmt.Printf("push rejected: event %s not found\n", eventId)
				return false
			}
			continue
		}

		// validate branch pushes
		if strings.HasPrefix(ref, "refs/heads/") {
			branchName := ref[11:]
			// pushing a branch
			if commitId, exists := state.Branches[branchName]; exists && to == commitId {
				fmt.Printf("push accepted: %s %s->%s\n", ref, from, to)
				continue
			}
			// deleting a branch
			if _, exists := state.Branches[branchName]; to == zeroRef && !exists {
				fmt.Printf("delete accepted: %s\n", ref)
				continue
			}
			fmt.Printf("push unauthorized: ref %s %s->%s does not match state\n", ref, from, to)
			return false
		}

		// validate tag pushes
		if strings.HasPrefix(ref, "refs/tags/") {
			tagName := ref[10:]
			// pushing a tag
			if commitId, exists := state.Tags[tagName]; exists && to == commitId {
				fmt.Printf("push accepted: %s %s->%s\n", ref, from, to)
				continue
			}
			// deleting a tag
			if _, exists := state.Tags[tagName]; to == zeroRef && !exists {
				fmt.Printf("delete accepted: %s\n", ref)
				continue
			}
			fmt.Printf("push unauthorized: ref %s %s->%s does not match state\n", ref, from, to)
			return false
		}
	}

	return true
}

// repoExists checks if a repository has an announcement event (kind 30617)
func (gs *GraspServer) repoExists(pubkey nostr.PubKey, repoName string) bool {
	for range gs.Relay.QueryStored(context.Background(), nostr.Filter{
		Kinds:   []nostr.Kind{nostr.KindRepositoryAnnouncement},
		Authors: []nostr.PubKey{pubkey},
		Tags:    nostr.TagMap{"d": []string{repoName}},
	}) {
		return true
	}
	return false
}

// runInfoRefs executes git-upload-pack with --http-backend-info-refs
func (gs *GraspServer) runInfoRefs(w http.ResponseWriter, r *http.Request, repoPath string) error {
	cmd := exec.Command("git",
		"-c", "uploadpack.allowReachableSHA1InWant=true",
		"-c", "uploadpack.allowTipSHA1InWant=true",
		"upload-pack", "--stateless-rpc", "--http-backend-info-refs", ".")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_PROTOCOL=%s", r.Header.Get("Git-Protocol")))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start git-upload-pack: %w", err)
	}

	// write pack line header only if not git protocol v2
	if !strings.Contains(r.Header.Get("Git-Protocol"), "version=2") {
		if err := gs.packLine(w, "# service=git-upload-pack\n"); err != nil {
			return fmt.Errorf("failed to write pack line: %w", err)
		}
		if err := gs.packFlush(w); err != nil {
			return fmt.Errorf("failed to flush pack: %w", err)
		}
	}

	io.Copy(gs.newWriteFlusher(w), stdoutPipe)
	stdoutPipe.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git-upload-pack failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// runUploadPack executes git-upload-pack for pull operations
func (gs *GraspServer) runUploadPack(w http.ResponseWriter, r *http.Request, repoPath string, bodyReader io.ReadCloser) error {
	cmd := exec.Command("git",
		"-c", "uploadpack.allowFilter=true",
		"-c", "uploadpack.allowReachableSHA1InWant=true",
		"-c", "uploadpack.allowTipSHA1InWant=true",
		"upload-pack", "--stateless-rpc", ".")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_PROTOCOL=%s", r.Header.Get("Git-Protocol")))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start git-upload-pack: %w", err)
	}

	// copy input to stdin
	go func() {
		defer stdinPipe.Close()
		io.Copy(stdinPipe, bodyReader)
	}()

	// copy output to response
	io.Copy(gs.newWriteFlusher(w), stdoutPipe)
	stdoutPipe.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git-upload-pack failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// runReceivePack executes git-receive-pack for push operations
func (gs *GraspServer) runReceivePack(w http.ResponseWriter, r *http.Request, repoPath string, bodyReader io.ReadCloser) error {
	cmd := exec.Command("git", "receive-pack", "--stateless-rpc", ".")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_PROTOCOL=%s", r.Header.Get("Git-Protocol")))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start git-receive-pack: %w", err)
	}

	// copy input to stdin
	go func() {
		defer stdinPipe.Close()
		io.Copy(stdinPipe, bodyReader)
	}()

	// copy output to response
	io.Copy(gs.newWriteFlusher(w), stdoutPipe)
	stdoutPipe.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git-receive-pack failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// updateHEAD updates the repository HEAD based on the latest state announcement
func (gs *GraspServer) updateHEAD(ctx context.Context, pubkey nostr.PubKey, repoName, repoPath string) error {
	if gs.Relay.QueryStored == nil {
		return fmt.Errorf("no QueryStored function")
	}

	// query for the latest state event
	var latestState *nip34.RepositoryState
	for evt := range gs.Relay.QueryStored(ctx, nostr.Filter{
		Kinds:   []nostr.Kind{nostr.KindRepositoryState},
		Authors: []nostr.PubKey{pubkey},
		Tags:    nostr.TagMap{"d": []string{repoName}},
		Limit:   1,
	}) {
		state := nip34.ParseRepositoryState(evt)
		latestState = &state
		break
	}

	if latestState == nil || latestState.HEAD == "" {
		// no state or no HEAD specified
		return nil
	}

	// verify the HEAD branch exists in the state
	if _, exists := latestState.Branches[latestState.HEAD]; !exists {
		return fmt.Errorf("HEAD branch %s not found in state", latestState.HEAD)
	}

	// update HEAD using git symbolic-ref
	cmd := exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/"+latestState.HEAD)
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update HEAD: %w, output: %s", err, string(output))
	}

	fmt.Printf("updated HEAD to %s for repo %s\n", latestState.HEAD, repoName)
	return nil
}

// cleanupMergedPatches removes refs/nostr/<event-id> refs that have been merged into branches
func (gs *GraspServer) cleanupMergedPatches(ctx context.Context, pubkey nostr.PubKey, repoName, repoPath string) {
	// use background context since request context will be cancelled
	ctx = context.Background()

	// wait 20 minutes before cleanup to allow events to propagate
	time.Sleep(20 * time.Minute)

	if gs.Relay.QueryStored == nil {
		return
	}

	// get current state to know which branches exist
	var state *nip34.RepositoryState
	for evt := range gs.Relay.QueryStored(ctx, nostr.Filter{
		Kinds:   []nostr.Kind{nostr.KindRepositoryState},
		Authors: []nostr.PubKey{pubkey},
		Tags:    nostr.TagMap{"d": []string{repoName}},
		Limit:   1,
	}) {
		parsed := nip34.ParseRepositoryState(evt)
		state = &parsed
		break
	}

	if state == nil {
		return
	}

	// list all refs/nostr/* refs
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/nostr")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		// no refs/nostr refs, nothing to clean up
		return
	}

	refs := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, ref := range refs {
		if ref == "" {
			continue
		}

		eventId := strings.TrimPrefix(ref, "refs/nostr/")
		id, err := nostr.IDFromHex(eventId)
		if err != nil {
			return
		}

		// check if there's still a valid patch event with a "c" tag referencing this commit
		hasValidEvent := false
		for evt := range gs.Relay.QueryStored(ctx, nostr.Filter{
			IDs: []nostr.ID{id},
		}) {
			// check if event has a "c" tag
			for _, tag := range evt.Tags {
				if tag[0] == "c" && len(tag) > 1 {
					hasValidEvent = true
					break
				}
			}
			break
		}

		if !hasValidEvent {
			// no valid event, delete the ref
			cmd := exec.Command("git", "update-ref", "-d", ref)
			cmd.Dir = repoPath
			if err := cmd.Run(); err != nil {
				fmt.Printf("failed to delete ref %s: %s\n", ref, err)
			} else {
				fmt.Printf("deleted ref %s (no corresponding event)\n", ref)
			}
			continue
		}

		// check if the commit is merged into any branch
		for branchName, commitId := range state.Branches {
			// get the commit ID for this ref
			cmd := exec.Command("git", "rev-parse", ref)
			cmd.Dir = repoPath
			refCommit, err := cmd.Output()
			if err != nil {
				continue
			}

			// check if ref commit is ancestor of branch head
			cmd = exec.Command("git", "merge-base", "--is-ancestor", strings.TrimSpace(string(refCommit)), commitId)
			cmd.Dir = repoPath
			if err := cmd.Run(); err == nil {
				// it's merged! delete the ref
				cmd := exec.Command("git", "update-ref", "-d", ref)
				cmd.Dir = repoPath
				if err := cmd.Run(); err != nil {
					fmt.Printf("failed to delete ref %s: %s\n", ref, err)
				} else {
					fmt.Printf("deleted ref %s (merged into %s)\n", ref, branchName)
				}
				break
			}
		}
	}
}

// serveRepoPage serves a webpage for the repository
func (gs *GraspServer) serveRepoPage(w http.ResponseWriter, r *http.Request, npub, repoName string) {
	w.Header().Set("Content-Type", "text/html")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>%s/%s - NIP-34 Git Repository</title>
	<style>
		body { font-family: sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
		h1 { color: #333; }
		code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
		pre { background: #f4f4f4; padding: 15px; border-radius: 5px; overflow-x: auto; }
		.info { background: #e7f3ff; padding: 15px; border-left: 4px solid #2196F3; margin: 20px 0; }
	</style>
</head>
<body>
	<h1>Repository: %s/%s</h1>
	<div class="info">
		<p>This is a NIP-34 git repository served over Nostr.</p>
	</div>
	<h2>Clone this repository</h2>
	<p>Use a git-nostr client to clone:</p>
	<pre>git clone %s/%s/%s.git</pre>
	<h2>Browse</h2>
	<p>Use a git-nostr web client or Nostr client to browse this repository.</p>
</body>
</html>`, npub, repoName, npub, repoName, r.Host, npub, repoName)
	fmt.Fprint(w, html)
}

// gitError writes a git error response
func (gs *GraspServer) gitError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("content-type", "text/plain; charset=UTF-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "%s\n", msg)
}

// packLine writes a pktline formatted line
func (gs *GraspServer) packLine(w io.Writer, s string) error {
	_, err := fmt.Fprintf(w, "%04x%s", len(s)+4, s)
	return err
}

// packFlush writes a pktline flush
func (gs *GraspServer) packFlush(w io.Writer) error {
	_, err := fmt.Fprint(w, "0000")
	return err
}

// newWriteFlusher creates a write flusher for streaming responses
func (gs *GraspServer) newWriteFlusher(w http.ResponseWriter) io.Writer {
	return writeFlusher{w.(interface {
		io.Writer
		http.Flusher
	})}
}

type writeFlusher struct {
	wf interface {
		io.Writer
		http.Flusher
	}
}

func (w writeFlusher) Write(p []byte) (int, error) {
	defer w.wf.Flush()
	return w.wf.Write(p)
}
