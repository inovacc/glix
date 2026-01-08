package module

import (
	"context"
	"testing"

	"github.com/spf13/afero"
)

func TestModule_Check(t *testing.T) {
	mod, err := NewModule(context.TODO(), afero.NewOsFs(), "go")
	if err != nil {
		t.Fatal(err)
	}

	if err := mod.FetchModuleInfo("https://github.com/spf13/afero.git"); err != nil {
		t.Fatal(err)
	}

	if err := mod.SaveToFile("module_data.json"); err != nil {
		t.Fatal(err)
	}
}

func TestModule_Check_Latest(t *testing.T) {
	mod, err := NewModule(context.TODO(), afero.NewOsFs(), "go")
	if err != nil {
		t.Fatal(err)
	}

	if err := mod.FetchModuleInfo("https://github.com/spf13/afero.git@latest"); err != nil {
		t.Fatal(err)
	}

	if err := mod.SaveToFile("module_data_latest.json"); err != nil {
		t.Fatal(err)
	}

	mod1, err := LoadModuleFromFile(afero.NewOsFs(), "module_data_latest.json")
	if err != nil {
		t.Fatal(err)
	}

	if mod.Name != mod1.Name {
		t.Fatalf("expected %s but got %s", mod.Name, mod1.Name)
	}
}
