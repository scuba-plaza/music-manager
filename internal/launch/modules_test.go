package launch_test

import (
	"os"
	"strings"
	"testing"
)

func TestTheModuleHasNoCommandLineFramework(t *testing.T) {
	gomod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, framework := range []string{"spf13/cobra", "spf13/pflag", "charmbracelet/", "golang.org/x/term"} {
		if strings.Contains(string(gomod), framework) {
			t.Errorf("go.mod still requires %s", framework)
		}
	}
}
