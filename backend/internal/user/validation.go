package user

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Field names carried by ValidationError.
const (
	FieldName     = "name"
	FieldEmail    = "email"
	FieldPassword = "password"
)

// Limits from data-model.md (User.name, User.email and the password rules).
const (
	maxNameLength     = 80
	maxEmailLength    = 100
	minPasswordLength = 8
	maxPasswordLength = 20
)

// Operator-facing messages (pt-BR, FR-012).
const (
	msgNameRequired     = "O nome é obrigatório."
	msgNameTooLong      = "O nome deve ter no máximo 80 caracteres."
	msgEmailRequired    = "O e-mail é obrigatório."
	msgEmailTooLong     = "O e-mail deve ter no máximo 100 caracteres."
	msgEmailInvalid     = "O formato do e-mail é inválido."
	msgPasswordRequired = "A senha é obrigatória."
	msgPasswordTooShort = "A senha deve ter no mínimo 8 caracteres."
	msgPasswordTooLong  = "A senha deve ter no máximo 20 caracteres."
)

// ValidationError reports that a field broke a rule. Message is in pt-BR and
// is safe to show to the operator; it never contains the rejected value.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// ValidateName trims the name and checks that it has 1 to 80 characters
// (runes). It returns the normalized name.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalid(FieldName, msgNameRequired)
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", invalid(FieldName, msgNameTooLong)
	}
	return name, nil
}

// ValidateEmail trims the e-mail, checks that it is present, has at most 100
// characters and has a valid format (research R4), and returns it lowercased.
func ValidateEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", invalid(FieldEmail, msgEmailRequired)
	}
	if utf8.RuneCountInString(email) > maxEmailLength {
		return "", invalid(FieldEmail, msgEmailTooLong)
	}
	if !validEmailFormat(email) {
		return "", invalid(FieldEmail, msgEmailInvalid)
	}
	return strings.ToLower(email), nil
}

// validEmailFormat implements FR-005 as refined in research R4: no whitespace,
// exactly one "@", a non-empty local part, a domain that contains a "." and
// does not start or end with one, and a bare address accepted by net/mail.
func validEmailFormat(email string) bool {
	if len(strings.Fields(email)) != 1 || strings.Count(email, "@") != 1 {
		return false
	}
	local, domain, _ := strings.Cut(email, "@")
	if local == "" || !strings.Contains(domain, ".") ||
		strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Name == "" && addr.Address == email
}

// ValidatePassword checks that the password has 8 to 20 characters (runes).
// The password is not trimmed and has no composition rules (FR-006).
func ValidatePassword(password string) error {
	if password == "" {
		return invalid(FieldPassword, msgPasswordRequired)
	}
	n := utf8.RuneCountInString(password)
	if n < minPasswordLength {
		return invalid(FieldPassword, msgPasswordTooShort)
	}
	if n > maxPasswordLength {
		return invalid(FieldPassword, msgPasswordTooLong)
	}
	return nil
}
