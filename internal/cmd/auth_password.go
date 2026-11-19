package cmd

import (
	"bytes"
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newHashPasswordCmd() *cobra.Command {
	var passwordStdin bool
	cmd := structured.Command[struct{}]{
		Use:   "hash-password",
		Short: "Hash a local recovery password without contacting LangSmith",
		Long: "Generate a PBKDF2-SHA256 hash for RECOVERY_ADMIN_PASSWORD_HASH locally.\n" +
			"Prompts for a password and confirmation without echoing either. No login or API key is required.\n" +
			"Use --password-stdin for automation; one trailing line ending is removed.\n" +
			"Use --format pretty to print only the hash, or --format json for a JSON string.",
		Args: cobra.NoArgs,
		Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
			password, err := readHashPassword(cmd, passwordStdin)
			if err != nil {
				return nil, err
			}
			defer clear(password)
			if len(password) == 0 || len(password) > 4096 {
				return nil, fmt.Errorf("password must contain between 1 and 4096 bytes")
			}
			salt := make([]byte, 16)
			if _, err := rand.Read(salt); err != nil {
				return nil, fmt.Errorf("generating password salt: %w", err)
			}
			digest, err := pbkdf2.Key(sha256.New, string(password), salt, 600_000, 32)
			if err != nil {
				return nil, fmt.Errorf("hashing password: %w", err)
			}
			return fmt.Sprintf("pbkdf2_sha256$600000$%s$%x", base64.StdEncoding.EncodeToString(salt), digest), nil
		},
		Render: structured.Template("{{.}}\n"),
	}.Cobra()
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the password from stdin (at most 4096 bytes)")
	return cmd
}

func readHashPassword(cmd *cobra.Command, passwordStdin bool) ([]byte, error) {
	if passwordStdin {
		password, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 4097))
		if err != nil {
			return nil, fmt.Errorf("reading password: %w", err)
		}
		if len(password) > 4096 {
			clear(password)
			return nil, fmt.Errorf("password input exceeds 4096 bytes")
		}
		if bytes.HasSuffix(password, []byte("\n")) {
			password = bytes.TrimSuffix(password, []byte("\n"))
			password = bytes.TrimSuffix(password, []byte("\r"))
		}
		return password, nil
	}
	input, ok := cmd.InOrStdin().(*os.File)
	if !ok || !inputIsTerminal(input) {
		return nil, fmt.Errorf("password prompt requires a terminal; use --password-stdin for piped input")
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Password: ")
	password, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Confirm password: ")
	confirmation, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	defer clear(confirmation)
	if err != nil {
		clear(password)
		return nil, fmt.Errorf("reading password confirmation: %w", err)
	}
	if subtle.ConstantTimeCompare(password, confirmation) != 1 {
		clear(password)
		return nil, fmt.Errorf("passwords do not match")
	}
	return password, nil
}
