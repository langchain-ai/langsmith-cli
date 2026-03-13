package deployment

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// RunCommand executes a command and returns stdout, stderr, and any error.
func RunCommand(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// RunCommandVerbose executes a command, printing it first if verbose is true,
// and streaming stdout/stderr to the terminal.
func RunCommandVerbose(verbose bool, name string, args ...string) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "+ %s %s\n", name, strings.Join(args, " "))
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunCommandWithInput executes a command with stdin input and returns stdout, stderr, and any error.
func RunCommandWithInput(input string, name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// RunCommandWithInputVerbose executes a command with stdin, streaming output and handling signals.
func RunCommandWithInputVerbose(verbose bool, input string, name string, args ...string) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "+ %s %s\n", name, strings.Join(args, " "))
	}
	cmd := exec.Command(name, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case sig := <-sigCh:
		_ = cmd.Process.Signal(sig)
		return <-done
	case err := <-done:
		signal.Stop(sigCh)
		return err
	}
}

// RunCommandStream runs a command streaming output while also collecting it.
func RunCommandStream(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return buf.String(), err
}
