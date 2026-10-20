package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func checkDestination(input string, inputInfo os.FileInfo, destination string) (bool, error) {
	inputAbs, err := filepath.Abs(input)
	if err != nil {
		return false, err
	}
	destinationAbs, err := filepath.Abs(destination)
	if err != nil {
		return false, err
	}
	if inputAbs == destinationAbs {
		return false, fmt.Errorf("%s: output must not replace the input", destination)
	}
	info, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%s: output must not be a symlink", destination)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s: output must be a regular file", destination)
	}
	if os.SameFile(inputInfo, info) {
		return false, fmt.Errorf("%s: output refers to the input file", destination)
	}
	return true, nil
}

func writeOutput(input string, inputInfo os.FileInfo, destination string, data []byte, force bool) error {
	exists, err := checkDestination(input, inputInfo, destination)
	if err != nil {
		return err
	}
	if exists && !force {
		return fmt.Errorf("%s: output already exists (use --force to replace it)", destination)
	}
	if !force {
		file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("%s: create output: %w", destination, err)
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(destination)
			if writeErr != nil {
				return fmt.Errorf("%s: write output: %w", destination, writeErr)
			}
			return fmt.Errorf("%s: close output: %w", destination, closeErr)
		}
		return nil
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".voice-memos-transcript-*")
	if err != nil {
		return fmt.Errorf("%s: create temporary output: %w", destination, err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("%s: write output: %w", destination, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("%s: close output: %w", destination, closeErr)
	}
	if _, err := checkDestination(input, inputInfo, destination); err != nil {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("%s: replace output: %w", destination, err)
	}
	return nil
}
