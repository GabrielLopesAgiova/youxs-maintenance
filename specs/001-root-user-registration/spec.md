# Feature Specification: ROOT User Registration

**Feature Branch**: `001-root-user-registration`

**Created**: 2026-11-24

**Status**: Draft

**Input**: User description: "Cadastro de usuário ROOT — o cadastro do usuário ROOT deve ser realizado por meio de um comando executado no terminal. Nome com no máximo 80 caracteres, e-mail com no máximo 100 caracteres e formato válido, senha com no máximo 20 caracteres armazenada com hash. Não deve ser permitido cadastrar e-mails repetidos. Em caso de erro, exibir mensagem explicando o motivo; em caso de sucesso, informar que o usuário ROOT foi criado. Fora do escopo: endpoints de cadastro, interface gráfica, autenticação, recuperação de senha."

## Clarifications

### Session 2026-11-24

- Q: Should the command allow registering several ROOT users, or only one ROOT user per installation? → A: Allow several ROOT users, each with a distinct e-mail.
- Q: Should the interactive command ask for the password twice and refuse to continue if the two entries differ? → A: Yes; in interactive mode the password is asked twice and a mismatch is reported as an error, creating no user.
- Q: Should the password have a minimum length of 8 characters? → A: Yes; the password must have between 8 and 20 characters, with no composition rules.
- Q: When the command runs without prompts (e.g. automation), how should it receive the password? → A: No non-interactive mode; the command is interactive only and always prompts for every field.
- Q: When the operator types an invalid value at a prompt, should the command re-ask that field or collect all fields and report every error before exiting? → A: Validate each field as it is entered and re-prompt until it is valid; a duplicate e-mail or a storage failure still ends the command with a failure status.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a ROOT user from the terminal (Priority: P1)

An operator with access to the server where the application is installed runs a terminal command, provides a name, an e-mail and a password, and gets a ROOT user registered in the system. The command confirms in the terminal that the ROOT user was created.

**Why this priority**: The ROOT user is the first privileged account of the system. Without it, nobody can administer the application, and there is no other way to create it (no endpoint or graphical interface exists for this).

**Independent Test**: Run the command on an environment with no users, provide valid data, and verify that the success message is shown and that a ROOT user with the given name and e-mail now exists, with the password not stored in plain text.

**Acceptance Scenarios**:

1. **Given** no user is registered with the e-mail `admin@youxs.com`, **When** the operator runs the command with name "Administrador", e-mail `admin@youxs.com` and a valid password, **Then** the command reports that the ROOT user was created and finishes successfully.
2. **Given** a ROOT user was just created, **When** the stored record is inspected, **Then** the password is stored only as a hash and the original password cannot be read from it.
3. **Given** the operator types the password, **When** it is entered, **Then** the password is not displayed on the screen.
4. **Given** the command is running, **When** the operator has typed the password, **Then** the command asks for it a second time for confirmation (also hidden).

---

### User Story 2 - Clear feedback when the data is invalid (Priority: P2)

An operator runs the command with data that breaks a rule (name too long, invalid e-mail, password too short or too long, password confirmation mismatch, missing field, e-mail already used). For each invalid field value, the command shows in the terminal a message explaining exactly what is wrong and asks for that field again, so the operator can fix it without restarting. If the e-mail is already registered or the data cannot be stored, the command ends without creating any user.

**Why this priority**: Correct feedback prevents partially created or inconsistent privileged accounts and lets the operator finish the task without reading code or logs.

**Independent Test**: Run the command and type each invalid input listed in the acceptance scenarios; verify that each message names the field and the reason, that the same field is asked again, and that no user is created while any field is invalid or when the command ends with a failure.

**Acceptance Scenarios**:

1. **Given** any state, **When** the operator provides a name with 81 characters, **Then** the command reports that the name must have at most 80 characters and asks for the name again.
2. **Given** any state, **When** the operator provides an e-mail with 101 characters, **Then** the command reports that the e-mail must have at most 100 characters and asks for the e-mail again.
3. **Given** any state, **When** the operator provides the e-mail `admin@`, **Then** the command reports that the e-mail format is invalid and asks for the e-mail again.
4. **Given** any state, **When** the operator provides a password with 21 characters, **Then** the command reports that the password must have at most 20 characters and asks for the password again.
5. **Given** any state, **When** the operator provides a password with 7 characters, **Then** the command reports that the password must have at least 8 characters and asks for the password again.
6. **Given** a user already exists with the e-mail `admin@youxs.com`, **When** the operator tries to register another ROOT user with `ADMIN@youxs.com`, **Then** no user is created, the command reports that the e-mail is already registered and finishes with a failure status.
7. **Given** any state, **When** the operator leaves the name, the e-mail or the password empty, **Then** the command reports that the field is required and asks for it again.
8. **Given** any state, **When** the password confirmation differs from the password, **Then** the command reports that the passwords do not match and asks for the password and its confirmation again.
9. **Given** the operator typed an invalid value for a field, **When** they type a valid value at the new prompt, **Then** the command accepts it and moves to the next field; no user is created until every field is valid.

---

### Edge Cases

- Values exactly at the limits (name with 80 characters, e-mail with 100 characters, password with 8 or 20 characters) MUST be accepted.
- Leading and trailing spaces in name and e-mail are removed before validation; a name made only of spaces is treated as empty.
- E-mail comparison for duplicates ignores letter case (`Admin@Youxs.com` and `admin@youxs.com` are the same e-mail).
- Name length is counted in characters, not bytes (accented names such as "João" count 4 characters).
- If the data store is unavailable, the command reports in the terminal that the user could not be created because the storage could not be reached, and no partial record remains.
- If two executions try to register the same e-mail at the same time, only one succeeds; the other reports that the e-mail is already registered.
- Running the command when one or more ROOT users already exist creates an additional ROOT user, as long as the e-mail is not already registered.
- If the operator interrupts the command (e.g. Ctrl+C) at any prompt, no user is created and the command finishes with a failure status.
- An unexpected failure never prints the password or its hash in the terminal.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a command, executed from the terminal, that registers a ROOT user.
- **FR-002**: The command MUST collect the user's name, e-mail and password interactively, prompting the operator for each field; it MUST NOT accept these values as command arguments.
- **FR-003**: The system MUST require the name, accepting at most 80 characters.
- **FR-004**: The system MUST require the e-mail, accepting at most 100 characters.
- **FR-005**: The system MUST reject e-mails whose format is not valid (local part, "@", and a domain containing at least one dot, with no spaces).
- **FR-006**: The system MUST require the password, accepting at least 8 and at most 20 characters, with no composition rules (no mandatory character classes).
- **FR-007**: The system MUST NOT allow registering an e-mail that already belongs to any existing user, comparing e-mails case-insensitively.
- **FR-008**: The system MUST store the password only in hashed form using a secure, salted, one-way password hashing algorithm; the plain-text password MUST NOT be persisted or written to any output or log.
- **FR-009**: The system MUST assign the ROOT role to the user created by the command.
- **FR-010**: Each field MUST be validated as soon as it is entered; when a value is invalid, the command MUST display in the terminal a message explaining the reason and ask for that field again, and MUST NOT create the user while any field is invalid. When the e-mail is already registered or processing fails (e.g. the data store is unavailable), the command MUST NOT create the user, MUST display a message explaining the reason, and MUST finish with a status that indicates failure.
- **FR-011**: When the user is created, the command MUST display in the terminal a message stating that the ROOT user was created and MUST finish with a status that indicates success.
- **FR-012**: All messages shown to the operator MUST be written in Portuguese (e.g. "Usuário ROOT criado com sucesso.", "O e-mail informado já está cadastrado.").
- **FR-013**: The password MUST NOT be echoed on the screen while being typed.
- **FR-014**: The system MUST allow more than one ROOT user to exist, as long as each one has a distinct e-mail; the existence of other ROOT users MUST NOT block the registration.
- **FR-015**: The command MUST ask for the password a second time (hidden) and MUST NOT create the user if the two entries differ, reporting that the passwords do not match.

### Key Entities

- **User**: A person who can access the system. Attributes: name (up to 80 characters), e-mail (up to 100 characters, unique among all users, case-insensitive), password hash (never the plain password), role, creation date.
- **Role**: The privilege level of a user. This feature only creates users with the ROOT role, the highest privilege level in the system.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can create a ROOT user with a single command execution in under 1 minute, including typing the data.
- **SC-002**: 100% of invalid values are rejected with a message identifying the field and the reason; no user is ever created with invalid data.
- **SC-003**: 0 users stored with a plain-text password; reading the stored data never reveals any user's password.
- **SC-004**: 0 duplicated e-mails among stored users, including under concurrent executions.
- **SC-005**: An operator who receives an error message can fix the input at the next prompt (or, for a duplicate e-mail, on the next run) and succeed without consulting documentation, logs or source code.

## Assumptions

- The operator running the command has access to the server/environment where the application runs; restricting who can run the command is handled by the environment's access controls, not by the command.
- The command is interactive only (prompting for each field, with hidden password input); non-interactive/automated execution is out of scope, so the password never appears in shell history or process listings.
- A minimum password length of 8 characters was confirmed during clarification (industry-standard baseline); no composition rules apply.
- E-mail uniqueness is checked against all users in the system, not only ROOT users.
- Name and e-mail are trimmed before validation and storage; e-mails are stored in lowercase.
- Out of scope: non-interactive (scripted) execution of the command, HTTP endpoints for user registration, graphical interface for registration, user authentication/login, password recovery, and editing or deleting users.
- The user data store is already available to the application where the command runs.
