// AgentOS serves an explicitly scoped, read-only local fleet view.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Aazernajaf/agentos/internal/api"
	"github.com/Aazernajaf/agentos/internal/fleet"
	"github.com/Aazernajaf/agentos/internal/observe"
	"github.com/Aazernajaf/agentos/internal/projecttask"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	address := flag.String("listen", "127.0.0.1:8080", "loopback address")
	web := flag.String("web-dir", "apps/web/dist", "built frontend directory")
	demo := flag.Bool("demo", false, "synthetic data only; never connects to providers")
	provider := flag.String("provider", "", "stdin provider: codex or claude")
	version := flag.String("provider-version", "", "exact source CLI version")
	session := flag.String("session", "", "explicitly authorized provider session ID")
	workspace := flag.String("workspace", "local", "workspace label")
	installation := flag.String("installation", "local", "installation label")
	reviewManifest := flag.String("review-manifest", "", "operator-owned project manifest outside its workspace")
	reviewWorkspace := flag.String("review-workspace", ".", "project workspace to verify")
	policyRoot := flag.String("policy-repo", ".", "canonical AgentOS policy repository")
	runChecks := flag.Bool("run-review-checks", false, "authorize the manifest's project test commands")
	flag.Parse()
	host, _, err := net.SplitHostPort(*address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("listen must be a numeric loopback address")
	}
	if _, err := os.Stat(*web + "/index.html"); err != nil {
		return errors.New("frontend missing: run npm --prefix apps/web ci and npm --prefix apps/web run build")
	}
	mode := "observe"
	if *demo {
		mode = "demo"
	}
	identity := fleet.Identity{Provider: *provider, Version: *version, Installation: *installation, Workspace: *workspace, Session: *session}
	if *demo && *provider != "" {
		return errors.New("choose demo or provider stdin, not both")
	}
	if !*demo && !identity.Valid() {
		return errors.New("stdin observation requires valid provider, provider-version, installation, workspace and session")
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return err
	}
	defer listener.Close()
	store := fleet.New(mode)
	handler, err := api.New(store, listener.Addr().String(), os.DirFS(*web))
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *reviewManifest != "" {
		if !*runChecks {
			return errors.New("review tests require --run-review-checks")
		}
		root, err := filepath.EvalSymlinks(*reviewWorkspace)
		if err != nil {
			return err
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return err
		}
		config, err := filepath.EvalSymlinks(*reviewManifest)
		if err != nil {
			return err
		}
		config, err = filepath.Abs(config)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, config)
		if err != nil || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
			return errors.New("operator manifest must be outside workspace")
		}
		f, err := os.Open(config)
		if err != nil {
			return err
		}
		m, err := projecttask.DecodeManifest(f)
		f.Close()
		if err != nil {
			return err
		}
		task, reviewer, err := projecttask.New(root, *policyRoot, m)
		if err != nil {
			return err
		}
		defer task.Close()
		if _, err := task.Verify(ctx); err != nil {
			return err
		}
		if err := handler.EnableReview(task, reviewer); err != nil {
			return err
		}
	}
	if *demo {
		go runDemo(ctx, store)
	} else {
		go ingest(store, identity)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Printf("AgentOS %s — http://%s\n", mode, listener.Addr())
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func ingest(store *fleet.Store, id fleet.Identity) {
	normalizer := observe.Normalizer{Identity: id}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), observe.MaxLine)
	var rejected uint64
	for scanner.Scan() {
		event, err := normalizer.Decode(scanner.Bytes(), time.Now())
		if err != nil {
			rejected++
			if rejected&(rejected-1) == 0 {
				log.Printf("provider events rejected: %d (content withheld)", rejected)
			}
			continue
		}
		if event != nil {
			if _, err := store.Apply(*event); err != nil {
				log.Print("observation rejected")
			}
		}
	}
	if scanner.Err() != nil {
		log.Print("input stream stopped: invalid, oversized or unreadable input")
	}
	_, _ = store.Apply(normalizer.Disconnected(time.Now()))
}

func runDemo(ctx context.Context, store *fleet.Store) {
	ids := []fleet.Identity{
		{Provider: "codex", Version: "0.150.1", Installation: "demo", Workspace: "demo", Session: "synthetic-codex"},
		{Provider: "claude", Version: "2.1.316", Installation: "demo", Workspace: "demo", Session: "synthetic-claude"},
	}
	var sequence uint64
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	step := func() {
		sequence++
		for i, id := range ids {
			state := "running"
			turn := fmt.Sprintf("demo-turn-%d", (sequence-1)/4)
			phase := (sequence - 1) % 4
			if phase == 2 {
				if i == 0 {
					state = "completed"
				} else {
					state = "failed"
				}
			}
			if phase == 3 {
				state = "disconnected"
				turn = ""
			}
			_, _ = store.Apply(fleet.Event{Identity: id, Sequence: sequence, Turn: turn, State: state, At: time.Now()})
		}
	}
	step()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			step()
		}
	}
}
