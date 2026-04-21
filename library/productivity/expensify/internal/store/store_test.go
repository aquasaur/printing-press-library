// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.

package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestUpsertPerson_Insert verifies a new Person row lands in the people table
// and Get by accountID returns the same values.
func TestUpsertPerson_Insert(t *testing.T) {
	s := openTestStore(t)
	p := Person{
		AccountID:   20647491,
		DisplayName: "Myk Melez",
		Login:       "myk@example.com",
		Avatar:      "https://example.com/a.png",
	}
	if err := s.UpsertPerson(p); err != nil {
		t.Fatalf("UpsertPerson: %v", err)
	}
	got, err := s.GetPersonByAccountID(20647491)
	if err != nil {
		t.Fatalf("GetPersonByAccountID: %v", err)
	}
	if got == nil {
		t.Fatal("GetPersonByAccountID returned nil, want row")
	}
	if got.AccountID != p.AccountID || got.DisplayName != p.DisplayName || got.Login != p.Login || got.Avatar != p.Avatar {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, p)
	}
	if got.SyncedAt == "" {
		t.Fatalf("SyncedAt = empty, want a timestamp")
	}
}

// TestUpsertPerson_Update verifies a second upsert with the same accountID
// overwrites the previous displayName.
func TestUpsertPerson_Update(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertPerson(Person{AccountID: 42, DisplayName: "Old Name", Login: "x@example.com"}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := s.UpsertPerson(Person{AccountID: 42, DisplayName: "New Name", Login: "x@example.com"}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := s.GetPersonByAccountID(42)
	if err != nil {
		t.Fatalf("GetPersonByAccountID: %v", err)
	}
	if got == nil || got.DisplayName != "New Name" {
		t.Fatalf("DisplayName = %+v, want %q after update", got, "New Name")
	}
}

// TestGetPersonByAccountID_NotFound verifies an unknown ID returns sql.ErrNoRows.
func TestGetPersonByAccountID_NotFound(t *testing.T) {
	s := openTestStore(t)
	got, err := s.GetPersonByAccountID(999)
	if got != nil {
		t.Fatalf("GetPersonByAccountID returned %+v, want nil", got)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

// TestGetPersonByLogin verifies case-insensitive login lookup.
func TestGetPersonByLogin(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertPerson(Person{AccountID: 7, DisplayName: "Myk", Login: "Myk@Example.COM"}); err != nil {
		t.Fatalf("UpsertPerson: %v", err)
	}
	got, err := s.GetPersonByLogin("myk@example.com")
	if err != nil {
		t.Fatalf("GetPersonByLogin: %v", err)
	}
	if got == nil {
		t.Fatal("GetPersonByLogin returned nil, want a row")
	}
	if got.AccountID != 7 {
		t.Fatalf("AccountID = %d, want 7", got.AccountID)
	}
}

// TestGetPersonByLogin_NotFound verifies sql.ErrNoRows on miss.
func TestGetPersonByLogin_NotFound(t *testing.T) {
	s := openTestStore(t)
	got, err := s.GetPersonByLogin("nobody@example.com")
	if got != nil {
		t.Fatalf("got = %+v, want nil", got)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

// TestUpsertPerson_EmptyDisplayName verifies an entry with only a login still
// upserts cleanly; display name stays empty.
func TestUpsertPerson_EmptyDisplayName(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertPerson(Person{AccountID: 100, DisplayName: "", Login: "bare@example.com"}); err != nil {
		t.Fatalf("UpsertPerson: %v", err)
	}
	got, err := s.GetPersonByAccountID(100)
	if err != nil {
		t.Fatalf("GetPersonByAccountID: %v", err)
	}
	if got == nil {
		t.Fatal("nil row, want one")
	}
	if got.DisplayName != "" || got.Login != "bare@example.com" {
		t.Fatalf("got %+v, want DisplayName=\"\" Login=\"bare@example.com\"", got)
	}
}

// TestListPeople verifies ListPeople returns all upserted rows ordered by
// display name.
func TestListPeople(t *testing.T) {
	s := openTestStore(t)
	_ = s.UpsertPerson(Person{AccountID: 1, DisplayName: "Zed", Login: "z@example.com"})
	_ = s.UpsertPerson(Person{AccountID: 2, DisplayName: "Alice", Login: "a@example.com"})
	people, err := s.ListPeople()
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("len(people) = %d, want 2", len(people))
	}
	if people[0].DisplayName != "Alice" || people[1].DisplayName != "Zed" {
		t.Fatalf("order = [%q, %q], want [Alice, Zed]", people[0].DisplayName, people[1].DisplayName)
	}
}
