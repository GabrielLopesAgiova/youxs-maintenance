package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"youxs/internal/user"
)

const testPassword = "senhaSegura1"

// fakeService records calls made by the prompter.
type fakeService struct {
	checkErr    error
	checkCalls  int
	checkCtxOK  bool // the ctx passed to CheckEmailAvailable had a deadline
	createErr   error
	createCalls int
	createIn    user.CreateRootInput
	createCtxOK bool // the ctx passed to CreateRoot had a deadline
	onCreate    func()
}

func (f *fakeService) CheckEmailAvailable(ctx context.Context, _ string) error {
	f.checkCalls++
	_, f.checkCtxOK = ctx.Deadline()
	return f.checkErr
}

func (f *fakeService) CreateRoot(ctx context.Context, in user.CreateRootInput) (user.User, error) {
	f.createCalls++
	f.createIn = in
	_, f.createCtxOK = ctx.Deadline()
	if f.onCreate != nil {
		f.onCreate()
	}
	if f.createErr != nil {
		return user.User{}, f.createErr
	}
	return user.User{ID: 1, Name: in.Name, Email: in.Email, Role: user.RoleRoot}, nil
}

// harness wires a prompter to in-memory input and output.
type harness struct {
	p             *prompter
	out, errOut   *bytes.Buffer
	passwordCalls int
}

// newHarness feeds lines to the line reader and secrets, in order, to
// readPassword. When the secrets run out, readPassword returns io.EOF.
func newHarness(lines string, secrets ...string) *harness {
	h := &harness{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	h.p = &prompter{
		in:     bufio.NewReader(strings.NewReader(lines)),
		out:    h.out,
		errOut: h.errOut,
		readPassword: func() ([]byte, error) {
			h.passwordCalls++
			if len(secrets) == 0 {
				return nil, io.EOF
			}
			s := secrets[0]
			secrets = secrets[1:]
			return []byte(s), nil
		},
	}
	return h
}

func (h *harness) assertNoPassword(t *testing.T, passwords ...string) {
	t.Helper()
	for _, pw := range passwords {
		if pw == "" {
			continue
		}
		if strings.Contains(h.out.String(), pw) || strings.Contains(h.errOut.String(), pw) {
			t.Errorf("output contains the password %q", pw)
		}
	}
}

func assertInOrder(t *testing.T, s string, parts ...string) {
	t.Helper()
	rest := s
	for _, p := range parts {
		i := strings.Index(rest, p)
		if i < 0 {
			t.Fatalf("output %q is missing %q (in order)", s, p)
		}
		rest = rest[i+len(p):]
	}
}

func TestRunHappyPath(t *testing.T) {
	h := newHarness("Administrador\nadmin@youxs.com\n", testPassword, testPassword)
	svc := &fakeService{}

	code := h.p.run(context.Background(), svc)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (FR-011); stderr=%q", code, h.errOut.String())
	}
	assertInOrder(t, h.out.String(),
		"Nome: ", "E-mail: ", "Senha: ", "Confirme a senha: ", "Usuário ROOT criado com sucesso.")
	if h.errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", h.errOut.String())
	}
	if h.passwordCalls != 2 {
		t.Errorf("readPassword called %d times, want 2 (FR-013, FR-015)", h.passwordCalls)
	}
	if svc.createCalls != 1 {
		t.Fatalf("CreateRoot called %d times, want 1", svc.createCalls)
	}
	if !svc.createCtxOK {
		t.Error("CreateRoot ctx has no deadline (10 s per-call timeout, research R9)")
	}
	want := user.CreateRootInput{Name: "Administrador", Email: "admin@youxs.com", Password: testPassword}
	if svc.createIn != want {
		t.Errorf("CreateRoot input = %+v, want %+v", svc.createIn, want)
	}
	h.assertNoPassword(t, testPassword)
}

// Ensure the dbTimeout constant stays at the value from research R9.
func TestDBTimeout(t *testing.T) {
	if dbTimeout != 10*time.Second {
		t.Errorf("dbTimeout = %v, want 10s", dbTimeout)
	}
}

const (
	validName  = "Administrador"
	validEmail = "admin@youxs.com"
)

func TestRunRepromptsInvalidField(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name     string
		lines    string   // name and e-mail lines
		secrets  []string // passwords and confirmations
		message  string   // expected field message on out
		reprompt string   // the prompt that must follow the message
	}{
		{"name too long (scenario 1)", long(81) + "\n" + validName + "\n" + validEmail + "\n",
			[]string{testPassword, testPassword}, "O nome deve ter no máximo 80 caracteres.", labelName},
		{"name empty (scenario 7)", "\n" + validName + "\n" + validEmail + "\n",
			[]string{testPassword, testPassword}, "O nome é obrigatório.", labelName},
		{"name only spaces", "    \n" + validName + "\n" + validEmail + "\n",
			[]string{testPassword, testPassword}, "O nome é obrigatório.", labelName},
		{"e-mail too long (scenario 2)", validName + "\n" + long(91) + "@youxs.com\n" + validEmail + "\n",
			[]string{testPassword, testPassword}, "O e-mail deve ter no máximo 100 caracteres.", labelEmail},
		{"e-mail format (scenario 3)", validName + "\nadmin@\n" + validEmail + "\n",
			[]string{testPassword, testPassword}, "O formato do e-mail é inválido.", labelEmail},
		{"e-mail empty (scenario 7)", validName + "\n\n" + validEmail + "\n",
			[]string{testPassword, testPassword}, "O e-mail é obrigatório.", labelEmail},
		{"password too long (scenario 4)", validName + "\n" + validEmail + "\n",
			[]string{long(21), testPassword, testPassword}, "A senha deve ter no máximo 20 caracteres.", labelPassword},
		{"password too short (scenario 5)", validName + "\n" + validEmail + "\n",
			[]string{"1234567", testPassword, testPassword}, "A senha deve ter no mínimo 8 caracteres.", labelPassword},
		{"password empty (scenario 7)", validName + "\n" + validEmail + "\n",
			[]string{"", testPassword, testPassword}, "A senha é obrigatória.", labelPassword},
		{"confirmation mismatch (scenario 8)", validName + "\n" + validEmail + "\n",
			[]string{testPassword, "outraSenha99", testPassword, testPassword}, "As senhas não conferem.", labelPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(tt.lines, tt.secrets...)
			svc := &fakeService{}

			code := h.p.run(context.Background(), svc)

			// Scenario 9: after the valid value the command moves on and succeeds.
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, h.errOut.String())
			}
			if !strings.Contains(h.out.String(), tt.message+"\n"+tt.reprompt) {
				t.Errorf("out %q does not contain %q followed by the prompt %q", h.out.String(), tt.message, tt.reprompt)
			}
			if svc.createCalls != 1 {
				t.Errorf("CreateRoot called %d times, want 1", svc.createCalls)
			}
			want := user.CreateRootInput{Name: validName, Email: validEmail, Password: testPassword}
			if svc.createIn != want {
				t.Errorf("CreateRoot input = %+v, want %+v", svc.createIn, want)
			}
			if !strings.Contains(h.out.String(), msgSuccess) {
				t.Error("success message missing")
			}
			if h.errOut.Len() != 0 {
				t.Errorf("stderr = %q, want empty", h.errOut.String())
			}
			h.assertNoPassword(t, append(tt.secrets, testPassword)...)
		})
	}
}

func TestRunConfirmationMismatchAsksBothAgain(t *testing.T) {
	h := newHarness(validName+"\n"+validEmail+"\n", testPassword, "outraSenha99", testPassword, testPassword)
	code := h.p.run(context.Background(), &fakeService{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertInOrder(t, h.out.String(),
		labelPassword, labelConfirmPassword, "As senhas não conferem.", labelPassword, labelConfirmPassword, msgSuccess)
	if h.passwordCalls != 4 {
		t.Errorf("readPassword called %d times, want 4", h.passwordCalls)
	}
}

func TestRunFatalPaths(t *testing.T) {
	const lines = validName + "\n" + validEmail + "\n"
	secrets := []string{testPassword, testPassword}
	tests := []struct {
		name          string
		svc           *fakeService
		cancelBefore  bool // cancel the parent ctx before run
		cancelOnCall  bool // cancel the parent ctx inside CreateRoot
		wantCode      int
		wantErrOut    string
		wantOut       string
		wantPwdCalls  int // -1 means "do not check"
		wantCreate    int
		forbiddenText string
	}{
		{
			name:       "e-mail already registered at the check (scenario 6)",
			svc:        &fakeService{checkErr: user.ErrEmailAlreadyRegistered},
			wantCode:   1,
			wantErrOut: msgEmailTaken, wantPwdCalls: 0, wantCreate: 0,
		},
		{
			name:       "e-mail check fails for another reason (FR-010)",
			svc:        &fakeService{checkErr: errors.New("dial tcp 10.0.0.5:5432: connection refused")},
			wantCode:   1,
			wantErrOut: msgDBUnreachable, wantPwdCalls: 0, wantCreate: 0,
			forbiddenText: "connection refused",
		},
		{
			name:       "duplicate detected by CreateRoot (race)",
			svc:        &fakeService{createErr: fmt.Errorf("insert user: %w", user.ErrEmailAlreadyRegistered)},
			wantCode:   1,
			wantErrOut: msgEmailTaken, wantPwdCalls: 2, wantCreate: 1,
		},
		{
			name:       "CreateRoot fails with another error",
			svc:        &fakeService{createErr: errors.New("insert user: disk full")},
			wantCode:   1,
			wantErrOut: msgSaveFailed, wantPwdCalls: 2, wantCreate: 1,
			forbiddenText: "disk full",
		},
		{
			name:         "CreateRoot canceled by a signal",
			svc:          &fakeService{createErr: fmt.Errorf("insert user: %w", context.Canceled)},
			cancelOnCall: true,
			wantCode:     1,
			wantErrOut:   msgCanceled, wantPwdCalls: 2, wantCreate: 1,
		},
		{
			name:         "parent ctx canceled before CreateRoot",
			svc:          &fakeService{},
			cancelBefore: true,
			wantCode:     1,
			wantErrOut:   msgCanceled, wantPwdCalls: -1, wantCreate: 0,
		},
		{
			name:         "CreateRoot commits although a signal arrived during the call",
			svc:          &fakeService{},
			cancelOnCall: true,
			wantCode:     0,
			wantOut:      msgSuccess, wantPwdCalls: 2, wantCreate: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(lines, secrets...)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelBefore {
				cancel()
			}
			if tt.cancelOnCall {
				tt.svc.onCreate = cancel
			}

			code := h.p.run(ctx, tt.svc)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; out=%q errOut=%q", code, tt.wantCode, h.out.String(), h.errOut.String())
			}
			if tt.wantErrOut != "" && !strings.Contains(h.errOut.String(), tt.wantErrOut) {
				t.Errorf("errOut = %q, want it to contain %q", h.errOut.String(), tt.wantErrOut)
			}
			if tt.wantOut != "" && !strings.Contains(h.out.String(), tt.wantOut) {
				t.Errorf("out = %q, want it to contain %q", h.out.String(), tt.wantOut)
			}
			if tt.wantCode != 0 && strings.Contains(h.out.String(), msgSuccess) {
				t.Error("success message printed on a failure path")
			}
			if tt.wantPwdCalls >= 0 && h.passwordCalls != tt.wantPwdCalls {
				t.Errorf("readPassword called %d times, want %d", h.passwordCalls, tt.wantPwdCalls)
			}
			if tt.svc.createCalls != tt.wantCreate {
				t.Errorf("CreateRoot called %d times, want %d", tt.svc.createCalls, tt.wantCreate)
			}
			if tt.svc.checkCalls != 1 && !tt.cancelBefore {
				t.Errorf("CheckEmailAvailable called %d times, want 1", tt.svc.checkCalls)
			}
			if tt.forbiddenText != "" &&
				(strings.Contains(h.out.String(), tt.forbiddenText) || strings.Contains(h.errOut.String(), tt.forbiddenText)) {
				t.Errorf("output leaks the underlying error text %q", tt.forbiddenText)
			}
			h.assertNoPassword(t, testPassword)
		})
	}
}

func TestRunEmailCheckHasDeadline(t *testing.T) {
	h := newHarness(validName+"\n"+validEmail+"\n", testPassword, testPassword)
	svc := &fakeService{}
	if code := h.p.run(context.Background(), svc); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if svc.checkCalls != 1 {
		t.Fatalf("CheckEmailAvailable called %d times, want 1", svc.checkCalls)
	}
	if !svc.checkCtxOK {
		t.Error("CheckEmailAvailable ctx has no deadline (research R9)")
	}
}

func TestRunEOFCancels(t *testing.T) {
	tests := []struct {
		name    string
		lines   string
		secrets []string
	}{
		{"at the name prompt", "", nil},
		{"at the e-mail prompt", validName + "\n", nil},
		{"at the password prompt", validName + "\n" + validEmail + "\n", nil},
		{"at the confirmation prompt", validName + "\n" + validEmail + "\n", []string{testPassword}},
		{"after re-prompting a field", "\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(tt.lines, tt.secrets...)
			svc := &fakeService{}
			code := h.p.run(context.Background(), svc)
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if !strings.Contains(h.errOut.String(), msgCanceled) {
				t.Errorf("errOut = %q, want %q", h.errOut.String(), msgCanceled)
			}
			if svc.createCalls != 0 {
				t.Error("CreateRoot called after EOF")
			}
			h.assertNoPassword(t, tt.secrets...)
		})
	}
}

func TestCanExitOnSignal(t *testing.T) {
	h := newHarness(validName+"\n"+validEmail+"\n", testPassword, testPassword)
	if !h.p.canExitOnSignal() {
		t.Fatal("canExitOnSignal = false before the create step")
	}
	svc := &fakeService{}
	var during bool
	svc.onCreate = func() { during = h.p.canExitOnSignal() }
	if code := h.p.run(context.Background(), svc); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if during {
		t.Error("canExitOnSignal = true while CreateRoot is running")
	}
}
