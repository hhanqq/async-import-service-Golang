package service

import (
	"fmt"
	"net/mail"
	"strings"

	"async-import-service/internal/model"
)

func ValidateCreateImportPayload(payload model.ImportPayload) error {
	if len(payload.Clients) == 0 {
		return fmt.Errorf("clients array must not be empty")
	}
	return nil
}

func ValidateClientInput(input model.ClientInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(input.Email) == "" {
		return fmt.Errorf("email is required")
	}
	if !IsValidEmail(input.Email) {
		return fmt.Errorf("invalid email")
	}
	return nil
}

func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
