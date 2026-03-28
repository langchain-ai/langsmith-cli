package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newSandboxConsoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console <name>",
		Short: "Open an interactive shell inside a sandbox",
		Long: `Open an interactive terminal session inside a running sandbox.

Connects via WebSocket to the sandbox daemon and allocates a PTY,
giving you a full interactive shell (bash by default).

Examples:
  langsmith sandbox console my-vm
  langsmith sandbox console my-vm --shell /bin/sh`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell, _ := cmd.Flags().GetString("shell")
			return runConsole(args[0], shell)
		},
	}

	cmd.Flags().String("shell", "", "Shell to use (default: sandbox default, usually /bin/bash)")

	return cmd
}

// consoleBoxResponse is the subset of the box API response we need.
type consoleBoxResponse struct {
	DataplaneURL *string `json:"dataplane_url"`
	Status       string  `json:"status"`
}

func runConsole(name, shell string) error {
	ctx := context.Background()

	// SANDBOX_DIRECT_URL overrides API lookup — connect directly to a
	// port-forwarded sandbox (e.g. http://localhost:8888).
	dpURL := os.Getenv("SANDBOX_DIRECT_URL")
	if dpURL == "" {
		apiKey := getAPIKey()
		if apiKey == "" {
			return fmt.Errorf("LANGSMITH_API_KEY not set")
		}

		// Resolve the sandbox's dataplane URL via the API.
		c := mustGetClient()

		var box consoleBoxResponse
		if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+name, &box); err != nil {
			return fmt.Errorf("getting sandbox: %w", err)
		}
		if box.Status != "ready" {
			return fmt.Errorf("sandbox %q is not ready (status: %s)", name, box.Status)
		}
		if box.DataplaneURL == nil || *box.DataplaneURL == "" {
			return fmt.Errorf("sandbox %q has no dataplane URL", name)
		}
		dpURL = *box.DataplaneURL
	}

	// Build WebSocket URL.
	wsScheme := "wss"
	if strings.HasPrefix(dpURL, "http://") {
		wsScheme = "ws"
	}
	// Strip scheme, keep host+path.
	hostPath := strings.TrimPrefix(strings.TrimPrefix(dpURL, "https://"), "http://")
	wsURL := fmt.Sprintf("%s://%s/execute/ws", wsScheme, hostPath)

	// Connect.
	dialer := websocket.Dialer{}
	header := http.Header{}
	if apiKey := getAPIKey(); apiKey != "" {
		header.Set("X-Api-Key", apiKey)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	ws, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket connect (%d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket connect: %w", err)
	}
	defer ws.Close()

	// Send execute message with PTY.
	execMsg := map[string]any{
		"type": "execute",
		"pty":  true,
	}
	if shell != "" {
		execMsg["shell"] = shell
	}
	if err := ws.WriteJSON(execMsg); err != nil {
		return fmt.Errorf("send execute: %w", err)
	}

	// Wait for "started" message.
	var startMsg struct {
		Type  string `json:"type"`
		Error string `json:"error,omitempty"`
	}
	if err := ws.ReadJSON(&startMsg); err != nil {
		return fmt.Errorf("read start: %w", err)
	}
	if startMsg.Type == "error" {
		return fmt.Errorf("server error: %s", startMsg.Error)
	}
	if startMsg.Type != "started" {
		return fmt.Errorf("unexpected message type: %s", startMsg.Type)
	}

	// Put terminal in raw mode.
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("make raw: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Send initial terminal size.
	sendResize(ws, fd)

	// Handle SIGWINCH (terminal resize).
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for range sigwinch {
			sendResize(ws, fd)
		}
	}()

	// Read from WebSocket → stdout.
	done := make(chan error, 1)
	go func() {
		for {
			_, raw, err := ws.ReadMessage()
			if err != nil {
				done <- err
				return
			}
			var msg struct {
				Type     string `json:"type"`
				Data     string `json:"data,omitempty"`
				ExitCode *int   `json:"exit_code,omitempty"`
				Error    string `json:"error,omitempty"`
			}
			if err := json.Unmarshal(raw, &msg); err != nil {
				os.Stdout.Write(raw)
				continue
			}
			switch msg.Type {
			case "stdout", "stderr":
				os.Stdout.WriteString(msg.Data)
			case "exit":
				done <- nil
				return
			case "error":
				done <- fmt.Errorf("server: %s", msg.Error)
				return
			}
		}
	}()

	// Read from stdin → WebSocket.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			msg, _ := json.Marshal(map[string]string{
				"type": "input",
				"data": string(buf[:n]),
			})
			if err := ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Wait for exit or interrupt.
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return nil
	}
}

func sendResize(ws *websocket.Conn, fd int) {
	w, h, err := term.GetSize(fd)
	if err != nil {
		return
	}
	msg, _ := json.Marshal(map[string]any{
		"type": "resize",
		"cols": w,
		"rows": h,
	})
	ws.WriteMessage(websocket.TextMessage, msg)
}
