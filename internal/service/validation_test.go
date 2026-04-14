package service

import (
	"testing"

	"async-import-service/internal/model"
)

func TestIsValidEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "valid", email: "ivan@example.com", want: true},
		{name: "invalid without at", email: "ivan.example.com", want: false},
		{name: "invalid domain", email: "ivan@", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsValidEmail(tc.email); got != tc.want {
				t.Fatalf("IsValidEmail(%q)=%v, want %v", tc.email, got, tc.want)
			}
		})
	}
}

func TestValidateCreateImportPayload(t *testing.T) {
	t.Parallel()

	if err := ValidateCreateImportPayload(model.ImportPayload{}); err == nil {
		t.Fatal("expected error for empty clients")
	}

	payload := model.ImportPayload{Clients: []model.ClientInput{{Name: "Ivan", Email: "ivan@example.com"}}}
	if err := ValidateCreateImportPayload(payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateClientInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   model.ClientInput
		wantErr bool
	}{
		{name: "valid", input: model.ClientInput{Name: "Ivan", Email: "ivan@example.com"}, wantErr: false},
		{name: "missing name", input: model.ClientInput{Email: "ivan@example.com"}, wantErr: true},
		{name: "missing email", input: model.ClientInput{Name: "Ivan"}, wantErr: true},
		{name: "invalid email", input: model.ClientInput{Name: "Ivan", Email: "invalid"}, wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateClientInput(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
