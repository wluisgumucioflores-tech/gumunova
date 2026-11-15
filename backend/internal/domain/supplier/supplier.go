// Package supplier holds the supplier domain: the businesses inventory is bought from.
package supplier

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"gumunova/backend/internal/domain/shared/validation"
)

var ErrNotFound = errors.New("supplier not found")

const (
	maxNameLength  = 120
	maxPhoneLength = 30
	maxEmailLength = 254
	maxNotesLength = 500
)

// Supplier is a business the inventory is bought from. Deactivating it keeps its history.
type Supplier struct {
	ID        string
	Name      string
	Phone     string
	Email     string
	Notes     string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Draft is the editable data of a supplier.
type Draft struct {
	Name  string
	Phone string
	Email string
	Notes string
}

// New builds a valid, active supplier from a draft.
func New(draft Draft) (Supplier, error) {
	draft = trimmed(draft)
	if err := validate(draft); err != nil {
		return Supplier{}, err
	}
	return Supplier{
		Name:     draft.Name,
		Phone:    draft.Phone,
		Email:    draft.Email,
		Notes:    draft.Notes,
		IsActive: true,
	}, nil
}

// Revise replaces the editable data. A failed revision leaves the supplier unchanged.
func (s *Supplier) Revise(draft Draft) error {
	draft = trimmed(draft)
	if err := validate(draft); err != nil {
		return err
	}
	s.Name = draft.Name
	s.Phone = draft.Phone
	s.Email = draft.Email
	s.Notes = draft.Notes
	return nil
}

func (s *Supplier) Activate()   { s.IsActive = true }
func (s *Supplier) Deactivate() { s.IsActive = false }

func trimmed(draft Draft) Draft {
	return Draft{
		Name:  strings.TrimSpace(draft.Name),
		Phone: strings.TrimSpace(draft.Phone),
		Email: strings.TrimSpace(draft.Email),
		Notes: strings.TrimSpace(draft.Notes),
	}
}

func validate(draft Draft) error {
	var problems []validation.FieldError
	add := func(field, code string) {
		problems = append(problems, validation.FieldError{Field: field, Code: code})
	}

	switch {
	case draft.Name == "":
		add("name", validation.CodeRequired)
	case utf8.RuneCountInString(draft.Name) > maxNameLength:
		add("name", validation.CodeTooLong)
	}

	if utf8.RuneCountInString(draft.Phone) > maxPhoneLength {
		add("phone", validation.CodeTooLong)
	}

	switch {
	case draft.Email == "":
	case utf8.RuneCountInString(draft.Email) > maxEmailLength:
		add("email", validation.CodeTooLong)
	case !isPlainEmail(draft.Email):
		add("email", validation.CodeInvalidEmail)
	}

	if utf8.RuneCountInString(draft.Notes) > maxNotesLength {
		add("notes", validation.CodeTooLong)
	}

	if len(problems) > 0 {
		return &validation.ValidationError{Fields: problems}
	}
	return nil
}

// isPlainEmail accepts a bare address such as "ventas@proveedor.com". It rejects display
// names ("Ana <ana@x.com>") and addresses without a dotted domain.
func isPlainEmail(value string) bool {
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return false
	}
	_, domain, found := strings.Cut(value, "@")
	return found && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}
