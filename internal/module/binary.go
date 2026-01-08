package module

import (
	"errors"
	"fmt"
	"os/exec"
)

func validGoBinary(name string) error {
	if err := exec.Command(name).Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return fmt.Errorf("failed to run binary %q: %w", name, err)
		}
	}

	return nil
}
