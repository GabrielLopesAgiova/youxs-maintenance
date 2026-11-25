package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"youxs/internal/user"
)

// dbTimeout bounds every database call made after startup (research R9).
const dbTimeout = 10 * time.Second

// Operator-facing texts (pt-BR, FR-012). They must match
// specs/001-root-user-registration/contracts/create-root-cli.md exactly.
const (
	labelName            = "Nome: "
	labelEmail           = "E-mail: "
	labelPassword        = "Senha: "
	labelConfirmPassword = "Confirme a senha: "

	msgSuccess          = "Usuário ROOT criado com sucesso."
	msgPasswordMismatch = "As senhas não conferem."
	msgEmailTaken       = "O e-mail informado já está cadastrado. Nenhum usuário foi criado."
	msgSaveFailed       = "Não foi possível criar o usuário ROOT: falha ao gravar os dados."
	msgDBUnreachable    = "Não foi possível criar o usuário ROOT: o banco de dados não está acessível."
	msgCanceled         = "Operação cancelada. Nenhum usuário foi criado."
	msgNoArgs           = "Este comando não aceita argumentos."
	msgNotTerminal      = "Este comando deve ser executado em um terminal interativo."
	msgNoDatabaseURL    = "Variável de ambiente DATABASE_URL não definida."
)

// rootCreator is the part of user.Service the prompter needs.
type rootCreator interface {
	CheckEmailAvailable(ctx context.Context, email string) error
	CreateRoot(ctx context.Context, in user.CreateRootInput) (user.User, error)
}

// prompter runs the interactive dialog. out receives prompts and field
// messages; errOut receives fatal messages.
type prompter struct {
	in           *bufio.Reader
	out          io.Writer
	errOut       io.Writer
	readPassword func() ([]byte, error)

	// mu guards creating. Once creating is true, a signal no longer ends the
	// process directly: run waits for CreateRoot and reports its real outcome.
	mu       sync.Mutex
	creating bool
}

// run asks for every field, creates the ROOT user and returns the exit code.
func (p *prompter) run(ctx context.Context, svc rootCreator) int {
	name, err := p.askField(labelName, user.ValidateName)
	if err != nil {
		return p.fail(msgCanceled)
	}
	email, err := p.askField(labelEmail, user.ValidateEmail)
	if err != nil {
		return p.fail(msgCanceled)
	}
	if code, ok := p.checkEmail(ctx, svc, email); !ok {
		return code
	}
	password, err := p.askPassword()
	if err != nil {
		return p.fail(msgCanceled)
	}
	return p.create(ctx, svc, user.CreateRootInput{Name: name, Email: email, Password: password})
}

// askField reads a field until validate accepts it, printing the reason for
// each rejected value (FR-010). It returns the normalized value.
func (p *prompter) askField(label string, validate func(string) (string, error)) (string, error) {
	for {
		line, err := p.readLine(label)
		if err != nil {
			return "", err
		}
		value, err := validate(line)
		if err == nil {
			return value, nil
		}
		if !p.printValidation(err) {
			return "", err
		}
	}
}

// askPassword reads the password and its confirmation, both hidden, until the
// password is valid and both entries match (FR-006, FR-013, FR-015).
func (p *prompter) askPassword() (string, error) {
	for {
		password, err := p.readSecret(labelPassword)
		if err != nil {
			return "", err
		}
		if err := user.ValidatePassword(password); err != nil {
			if !p.printValidation(err) {
				return "", err
			}
			continue
		}
		confirmation, err := p.readSecret(labelConfirmPassword)
		if err != nil {
			return "", err
		}
		if confirmation != password {
			fmt.Fprintln(p.out, msgPasswordMismatch)
			continue
		}
		return password, nil
	}
}

// printValidation prints the message of a *user.ValidationError and reports
// whether err was one.
func (p *prompter) printValidation(err error) bool {
	var ve *user.ValidationError
	if !errors.As(err, &ve) {
		return false
	}
	fmt.Fprintln(p.out, ve.Message)
	return true
}

// checkEmail asks the service whether the e-mail is free. When it is not, or
// the check fails, it prints the fatal message and returns ok == false.
func (p *prompter) checkEmail(ctx context.Context, svc rootCreator, email string) (code int, ok bool) {
	callCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	err := svc.CheckEmailAvailable(callCtx, email)
	switch {
	case err == nil:
		return 0, true
	case errors.Is(err, user.ErrEmailAlreadyRegistered):
		return p.fail(msgEmailTaken), false
	case ctx.Err() != nil:
		return p.fail(msgCanceled), false
	default:
		return p.fail(msgDBUnreachable), false
	}
}

// create stores the user and reports the outcome as it happened: a signal that
// arrives during the call never turns a committed row into a failure report.
func (p *prompter) create(ctx context.Context, svc rootCreator, in user.CreateRootInput) int {
	p.mu.Lock()
	if ctx.Err() != nil {
		p.mu.Unlock()
		return p.fail(msgCanceled)
	}
	p.creating = true
	p.mu.Unlock()

	callCtx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := svc.CreateRoot(callCtx, in)
	switch {
	case err == nil:
		fmt.Fprintln(p.out, msgSuccess)
		return 0
	case errors.Is(err, user.ErrEmailAlreadyRegistered):
		return p.fail(msgEmailTaken)
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		return p.fail(msgCanceled)
	default:
		return p.fail(msgSaveFailed)
	}
}

// canExitOnSignal reports whether a signal may end the process right away,
// which is true until the create step starts.
func (p *prompter) canExitOnSignal() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.creating
}

// readLine writes label and reads one line without its line ending. A final
// line without a newline is accepted; an empty read at end of input returns
// io.EOF.
func (p *prompter) readLine(label string) (string, error) {
	fmt.Fprint(p.out, label)
	line, err := p.in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// readSecret writes label and reads a value without echo. The terminal does
// not print the operator's Enter while echo is off, so a newline is written.
func (p *prompter) readSecret(label string) (string, error) {
	fmt.Fprint(p.out, label)
	b, err := p.readPassword()
	fmt.Fprintln(p.out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// fail writes a fatal message and returns the failure exit code.
func (p *prompter) fail(msg string) int {
	fmt.Fprintln(p.errOut, msg)
	return 1
}
