//go:build !windows

package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
  langsmith sandbox console my-vm --shell /bin/sh
  langsmith sandbox console my-vm --forward-ssh-agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell, _ := cmd.Flags().GetString("shell")
			forwardSSHAgent, _ := cmd.Flags().GetBool("forward-ssh-agent")
			return runConsole(args[0], shell, forwardSSHAgent)
		},
	}

	cmd.Flags().String("shell", "", "Shell to use (default: sandbox default, usually /bin/bash)")
	cmd.Flags().Bool("forward-ssh-agent", false, "Forward the local SSH agent (SSH_AUTH_SOCK) into the sandbox")

	return cmd
}

func runConsole(name, shell string, forwardSSHAgent bool) error {
	ctx := context.Background()

	ep, err := resolveSandbox(ctx, name)
	if err != nil {
		return err
	}
	dpURL := ep.DataplaneURL

	// Build WebSocket URL.
	wsURL := dataplaneWSURL(dpURL, "/execute/ws")

	// Connect.
	dialer := websocket.Dialer{}
	header := http.Header{}
	for k, v := range sandboxAuthHeaders() {
		header.Set(k, v)
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
	if forwardSSHAgent {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return fmt.Errorf("--forward-ssh-agent requires SSH_AUTH_SOCK to be set (is ssh-agent running?)")
		}
		execMsg["ssh_agent_forward"] = true
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
	defer func() { _ = term.Restore(fd, oldState) }()

	// Mutex for concurrent WebSocket writes (gorilla/websocket requires it).
	var wsMu sync.Mutex
	wsWrite := func(msg []byte) error {
		wsMu.Lock()
		defer wsMu.Unlock()
		return ws.WriteMessage(websocket.TextMessage, msg)
	}

	// Send initial terminal size.
	sendResizeLocked(&wsMu, ws, fd)

	// Handle SIGWINCH (terminal resize).
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for range sigwinch {
			sendResizeLocked(&wsMu, ws, fd)
		}
	}()

	// SSH agent forwarding state (client side).
	var agentMu sync.Mutex
	agentConns := make(map[string]net.Conn)
	agentSock := os.Getenv("SSH_AUTH_SOCK")

	defer func() {
		agentMu.Lock()
		for _, c := range agentConns {
			c.Close()
		}
		agentMu.Unlock()
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
				Type      string `json:"type"`
				Data      string `json:"data,omitempty"`
				ChannelID string `json:"channel_id,omitempty"`
				ExitCode  *int   `json:"exit_code,omitempty"`
				Error     string `json:"error,omitempty"`
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
			case "ssh_agent_data":
				data, err := base64.StdEncoding.DecodeString(msg.Data)
				if err != nil {
					continue
				}
				agentMu.Lock()
				conn, ok := agentConns[msg.ChannelID]
				if !ok && agentSock != "" {
					conn, err = net.Dial("unix", agentSock)
					if err != nil {
						agentMu.Unlock()
						continue
					}
					agentConns[msg.ChannelID] = conn
					// Read responses from local agent and send back.
					go func(chID string, c net.Conn) {
						buf := make([]byte, 16384)
						for {
							n, err := c.Read(buf)
							if err != nil {
								return
							}
							resp, _ := json.Marshal(map[string]string{
								"type":       "ssh_agent_data",
								"channel_id": chID,
								"data":       base64.StdEncoding.EncodeToString(buf[:n]),
							})
							if err := wsWrite(resp); err != nil {
								return
							}
						}
					}(msg.ChannelID, conn)
				}
				agentMu.Unlock()
				if conn != nil {
					_, _ = conn.Write(data)
				}
			case "ssh_agent_close":
				agentMu.Lock()
				if conn, ok := agentConns[msg.ChannelID]; ok {
					conn.Close()
					delete(agentConns, msg.ChannelID)
				}
				agentMu.Unlock()
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
			if err := wsWrite(msg); err != nil {
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

func sendResizeLocked(mu *sync.Mutex, ws *websocket.Conn, fd int) {
	w, h, err := term.GetSize(fd)
	if err != nil {
		return
	}
	msg, _ := json.Marshal(map[string]any{
		"type": "resize",
		"cols": w,
		"rows": h,
	})
	mu.Lock()
	defer mu.Unlock()
	_ = ws.WriteMessage(websocket.TextMessage, msg)
}
