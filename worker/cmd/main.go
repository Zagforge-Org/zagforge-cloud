package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	BinaryName = "zigzag"
)

func main() {
	timeout := time.Minute * 5

	// The first argument is always the program name.
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <command> [args...]", os.Args[0])
	}

	path, err := exec.LookPath(BinaryName)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		log.Fatalf("error: %s not found: %v", BinaryName, err)
	} else if err != nil {
		log.Fatalf("error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, os.Args[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	switch {
	case errors.Is(err, exec.ErrDot):
		log.Fatalf("error: cannot execute %q: resolved via relative path in $PATH", path)
	case errors.Is(err, context.DeadlineExceeded):
		log.Fatalf("error: command timed out")
	case err != nil:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatalf("error running command: %v", err)
	}
}
