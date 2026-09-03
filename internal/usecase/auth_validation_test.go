package usecase

import "testing"

func TestNormalizeRegistration(t *testing.T) {
	tests := []struct {
		name    string
		input   RegisterInput
		wantErr bool
	}{
		{name: "normalizes a valid email", input: RegisterInput{Name: "  Ada  ", Email: " ADA@Example.COM ", Password: "password123"}},
		{name: "rejects malformed email", input: RegisterInput{Name: "Ada", Email: "ada-at-example.com", Password: "password123"}, wantErr: true},
		{name: "rejects display name email", input: RegisterInput{Name: "Ada", Email: "Ada <ada@example.com>", Password: "password123"}, wantErr: true},
		{name: "rejects short password", input: RegisterInput{Name: "Ada", Email: "ada@example.com", Password: "short"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, email, _, err := normalizeRegistration(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("normalizeRegistration() error = %v, wantErr %t", err, test.wantErr)
			}
			if !test.wantErr && (name != "Ada" || email != "ada@example.com") {
				t.Fatalf("normalized values = (%q, %q), want (Ada, ada@example.com)", name, email)
			}
		})
	}
}
