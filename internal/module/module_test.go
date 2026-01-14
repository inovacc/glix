package module

import (
	"context"
	"testing"
)

func TestModule_Check(t *testing.T) {
	tmpDir := t.TempDir()

	mod, err := NewModule(context.TODO(), "go", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Use brdoc which has auto-discovery support
	if err := mod.FetchModuleInfo("github.com/inovacc/brdoc"); err != nil {
		t.Fatal(err)
	}

	if err := mod.SaveToFile("module_data.json"); err != nil {
		t.Fatal(err)
	}
}

func TestModule_Check_Latest(t *testing.T) {
	tmpDir := t.TempDir()

	mod, err := NewModule(context.TODO(), "go", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Use brdoc which has auto-discovery support
	if err := mod.FetchModuleInfo("github.com/inovacc/brdoc@latest"); err != nil {
		t.Fatal(err)
	}

	if err := mod.SaveToFile("module_data_latest.json"); err != nil {
		t.Fatal(err)
	}

	mod1, err := LoadModuleFromFile("module_data_latest.json")
	if err != nil {
		t.Fatal(err)
	}

	if mod.Name != mod1.Name {
		t.Fatalf("expected %s but got %s", mod.Name, mod1.Name)
	}
}
