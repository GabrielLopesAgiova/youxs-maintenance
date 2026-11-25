package user

import (
	"errors"
	"strings"
	"testing"
)

func assertValidationError(t *testing.T, err error, field, message string) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	if ve.Field != field {
		t.Errorf("Field = %q, want %q", ve.Field, field)
	}
	if ve.Message != message {
		t.Errorf("Message = %q, want %q", ve.Message, message)
	}
}

func TestValidateName(t *testing.T) {
	const (
		required = "O nome é obrigatório."
		tooLong  = "O nome deve ter no máximo 80 caracteres."
	)
	tests := []struct {
		name    string
		input   string
		want    string
		wantMsg string
	}{
		{"empty", "", "", required},
		{"spaces only", "   \t ", "", required},
		{"80 runes accepted", strings.Repeat("a", 80), strings.Repeat("a", 80), ""},
		{"81 runes rejected", strings.Repeat("a", 81), "", tooLong},
		{"80 accented runes accepted", strings.Repeat("ã", 80), strings.Repeat("ã", 80), ""},
		{"81 accented runes rejected", strings.Repeat("ã", 81), "", tooLong},
		{"trimmed", "  João  ", "João", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateName(tt.input)
			if tt.wantMsg != "" {
				assertValidationError(t, err, FieldName, tt.wantMsg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	const (
		required = "O e-mail é obrigatório."
		tooLong  = "O e-mail deve ter no máximo 100 caracteres."
		invalid  = "O formato do e-mail é inválido."
	)
	const domain = "@youxs.com" // 10 characters
	email100 := strings.Repeat("a", 90) + domain
	email101 := strings.Repeat("a", 91) + domain

	tests := []struct {
		name    string
		input   string
		want    string
		wantMsg string
	}{
		{"empty", "", "", required},
		{"spaces only", "   ", "", required},
		{"100 runes accepted", email100, email100, ""},
		{"101 runes rejected", email101, "", tooLong},
		{"missing domain", "admin@", "", invalid},
		{"domain without dot", "a@b", "", invalid},
		{"inner space", "a b@c.com", "", invalid},
		{"display name", "Nome <a@b.com>", "", invalid},
		{"angle brackets", "<a@b.com>", "", invalid},
		{"double at", "a@@b.com", "", invalid},
		{"two at signs", "a@b@c.com", "", invalid},
		{"missing local part", "@b.com", "", invalid},
		{"domain starts with dot", "a@.com", "", invalid},
		{"domain ends with dot", "a@b.com.", "", invalid},
		{"no at sign", "admin.youxs.com", "", invalid},
		{"valid", "admin@youxs.com", "admin@youxs.com", ""},
		{"subdomain", "ops@mail.youxs.com.br", "ops@mail.youxs.com.br", ""},
		{"trimmed and lowercased", " Admin@Youxs.com ", "admin@youxs.com", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateEmail(tt.input)
			if tt.wantMsg != "" {
				assertValidationError(t, err, FieldEmail, tt.wantMsg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	const (
		required = "A senha é obrigatória."
		tooShort = "A senha deve ter no mínimo 8 caracteres."
		tooLong  = "A senha deve ter no máximo 20 caracteres."
	)
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", required},
		{"7 runes rejected", strings.Repeat("a", 7), tooShort},
		{"8 runes accepted", strings.Repeat("a", 8), ""},
		{"20 runes accepted", strings.Repeat("a", 20), ""},
		{"21 runes rejected", strings.Repeat("a", 21), tooLong},
		{"20 accented runes accepted", strings.Repeat("ç", 20), ""},
		// Not trimmed: 6 visible characters plus 2 leading spaces count as 8.
		{"leading spaces are not trimmed", "  abcdef", ""},
		{"spaces count toward the maximum", "   " + strings.Repeat("a", 18), tooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.input)
			if tt.wantMsg != "" {
				assertValidationError(t, err, FieldPassword, tt.wantMsg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidationErrorMessageHasNoValue(t *testing.T) {
	err := ValidatePassword("short")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "short") {
		t.Errorf("error text %q must not contain the password", err.Error())
	}
}
