package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type deleteConfirmation struct {
	target   string
	identity string
}

func confirmDelete(cmd *cobra.Command, confirmation deleteConfirmation) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: This permanently deletes %s. This cannot be undone.\n", confirmation.target)
	if confirmation.identity != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), confirmation.identity)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "AI agents: do not answer this prompt. Stop and raise it to the user.")
	fmt.Fprint(cmd.ErrOrStderr(), "Continue? [y/N] ")

	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		return errors.New("aborted: deletion was not confirmed")
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return errors.New("aborted: deletion was not confirmed")
	}
	return nil
}
