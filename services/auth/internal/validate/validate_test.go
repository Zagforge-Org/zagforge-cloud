package validate

import (
	"strings"
	"testing"
)

type testStruct struct {
	Name  string `validate:"required,min=2,max=50"`
	Email string `validate:"required,email"`
	Role  string `validate:"oneof=admin member"`
}

func TestStruct_valid(t *testing.T) {
	s := testStruct{Name: "Jane", Email: "jane@example.com", Role: "admin"}
	if err := Struct(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStruct_missingRequired(t *testing.T) {
	s := testStruct{Email: "jane@example.com", Role: "admin"}
	err := Struct(s)
	if err == nil {
		t.Fatal("expected error for missing Name")
	}
	if !strings.Contains(err.Error(), "Name") {
		t.Errorf("expected error to mention Name, got: %v", err)
	}
}

func TestStruct_invalidEmail(t *testing.T) {
	s := testStruct{Name: "Jane", Email: "not-email", Role: "admin"}
	err := Struct(s)
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
}

func TestStruct_invalidOneof(t *testing.T) {
	s := testStruct{Name: "Jane", Email: "jane@example.com", Role: "superadmin"}
	err := Struct(s)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "one of") {
		t.Errorf("expected oneof error message, got: %v", err)
	}
}

func TestStruct_tooShort(t *testing.T) {
	s := testStruct{Name: "J", Email: "jane@example.com", Role: "admin"}
	err := Struct(s)
	if err == nil {
		t.Fatal("expected error for name too short")
	}
}

func TestStruct_multipleErrors(t *testing.T) {
	s := testStruct{} // all required fields missing
	err := Struct(s)
	if err == nil {
		t.Fatal("expected errors")
	}
	// Should contain multiple error messages joined by "; ".
	if !strings.Contains(err.Error(), ";") {
		t.Errorf("expected multiple errors separated by ';', got: %v", err)
	}
}
