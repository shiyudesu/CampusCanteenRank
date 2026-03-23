package repository

import (
	"errors"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func TestNewMySQLUserRepositoryWithNilDB(t *testing.T) {
	repo, err := NewMySQLUserRepository(nil)
	if err == nil {
		t.Fatalf("expected error when db is nil")
	}
	if repo != nil {
		t.Fatalf("expected nil repository when db is nil")
	}
}

func TestIsMySQLDuplicate(t *testing.T) {
	if !isMySQLDuplicate(&mysqlDriver.MySQLError{Number: 1062}) {
		t.Fatalf("expected duplicate mysql error to be recognized")
	}
	if isMySQLDuplicate(&mysqlDriver.MySQLError{Number: 1048}) {
		t.Fatalf("expected non-duplicate mysql error to be rejected")
	}
	if isMySQLDuplicate(errors.New("plain error")) {
		t.Fatalf("expected plain error to be rejected")
	}
	wrapped := &gorm.DB{Error: &mysqlDriver.MySQLError{Number: 1062}}
	if !isMySQLDuplicate(wrapped.Error) {
		t.Fatalf("expected wrapped duplicate mysql error to be recognized")
	}
}
