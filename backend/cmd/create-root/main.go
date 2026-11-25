// Command create-root registers a ROOT user interactively from the terminal.
// See specs/001-root-user-registration/contracts/create-root-cli.md.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embeds the zone database so TZ=America/Sao_Paulo always resolves (Constitution VIII)

	"golang.org/x/term"

	"youxs/internal/user"
	"youxs/internal/user/postgres"
)

const pingTimeout = 5 * time.Second

func main() {
	os.Exit(realMain())
}

func realMain() int {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, msgNoArgs)
		return 1
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Fprintln(os.Stderr, msgNotTerminal)
		return 1
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, msgNoDatabaseURL)
		return 1
	}

	db, err := sql.Open("pgx", url)
	if err != nil {
		fmt.Fprintln(os.Stderr, msgDBUnreachable)
		return 1
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	err = db.PingContext(pingCtx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, msgDBUnreachable)
		return 1
	}

	svc := user.NewService(postgres.New(db))
	p := &prompter{
		in:           bufio.NewReader(os.Stdin),
		out:          os.Stdout,
		errOut:       os.Stderr,
		readPassword: func() ([]byte, error) { return term.ReadPassword(fd) },
	}

	// Save the terminal state so echo can be restored if a signal arrives
	// during a hidden prompt (research R6).
	state, _ := term.GetState(fd) // nil on error: there is nothing to restore
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	finished := make(chan struct{})
	defer close(finished) // runs before stop, so a normal return is never read as a signal
	go handleSignal(ctx, finished, fd, state, p)

	return p.run(ctx, svc)
}

// handleSignal waits for SIGINT/SIGTERM. At a prompt or during the e-mail
// check it restores the terminal and ends the process with status 1. While the
// user is being saved it only restores the terminal: run then reports the real
// outcome of the save (contract: a non-zero exit means no row from this run).
func handleSignal(ctx context.Context, finished <-chan struct{}, fd int, state *term.State, p *prompter) {
	select {
	case <-finished:
		return
	case <-ctx.Done():
	}
	select {
	case <-finished: // run returned at the same moment; its result stands
		return
	default:
	}
	if state != nil {
		_ = term.Restore(fd, state)
	}
	if p.canExitOnSignal() {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, msgCanceled)
		os.Exit(1)
	}
}
