package service

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestValidateSSHUserName(t *testing.T) {
	valid := []string{"", "root", "ubuntu", "first.last", "svc_user-1", "_admin"}
	for _, name := range valid {
		if err := ValidateSSHUserName(name); err != nil {
			t.Errorf("expected %q to be valid, got %v", name, err)
		}
	}

	invalid := []string{`DOMAIN\alice`, "a b", "-oProxyCommand=x", "root;id", "$(id)", "user@host", "a'b"}
	for _, name := range invalid {
		if err := ValidateSSHUserName(name); err == nil {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestPreferencesStoreSaveAndLoad(t *testing.T) {
	storage := test.NewTempApp(t).Storage()

	ps, err := NewPreferencesStore(storage)
	if err != nil {
		t.Fatalf("NewPreferencesStore: %v", err)
	}

	if err := ps.Save(Preferences{DefaultSSHUserName: "root"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := NewPreferencesStore(storage)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Get().DefaultSSHUserName; got != "root" {
		t.Fatalf("expected reloaded user name %q, got %q", "root", got)
	}
}

func TestPreferencesStoreSaveInvalidKeepsCurrent(t *testing.T) {
	ps, err := NewPreferencesStore(test.NewTempApp(t).Storage())
	if err != nil {
		t.Fatalf("NewPreferencesStore: %v", err)
	}
	if err := ps.Save(Preferences{DefaultSSHUserName: "root"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := ps.Save(Preferences{DefaultSSHUserName: `DOMAIN\alice`}); err == nil {
		t.Fatal("expected error saving invalid user name")
	}
	if got := ps.Get().DefaultSSHUserName; got != "root" {
		t.Fatalf("expected preferences to remain %q after failed save, got %q", "root", got)
	}
}

func TestPreferencesStoreLoadDiscardsInvalidUserName(t *testing.T) {
	s := test.NewTempApp(t).Storage()

	w, err := s.Save(preferencesFileName)
	if err != nil {
		w, err = s.Create(preferencesFileName)
	}
	if err != nil {
		t.Fatalf("opening %s for writing: %v", preferencesFileName, err)
	}
	if _, err := w.Write([]byte(`{"default_ssh_user_name": "$(id)"}`)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ps, err := NewPreferencesStore(s)
	if err != nil {
		t.Fatalf("NewPreferencesStore: %v", err)
	}
	if got := ps.Get().DefaultSSHUserName; got != "" {
		t.Fatalf("expected invalid user name to be discarded, got %q", got)
	}
}
