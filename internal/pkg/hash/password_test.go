package hash

import "testing"

func TestPasswordHashing(t *testing.T) {
	password := "SecretPassword@123"

	hashed, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !CheckPassword(password, hashed) {
		t.Errorf("Expected password to match hash")
	}

	if CheckPassword("WrongPassword", hashed) {
		t.Errorf("Expected wrong password to fail matching")
	}
}
