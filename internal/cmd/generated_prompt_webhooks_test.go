package cmd

const (
	testWebhookID = "11111111-1111-1111-1111-111111111111"
	testPromptID  = "22222222-2222-2222-2222-222222222222"
	testWebhook   = `{"id":"` + testWebhookID + `","tenant_id":"33333333-3333-3333-3333-333333333333","url":"https://example.com/hook","triggers":["commit"],"created_at":"2026-09-30T00:00:00Z","updated_at":"2026-09-30T00:00:00Z"}`
)

func init() {
	generatedContractCases["prompt-webhooks"] = []generatedContractCase{
		{
			name: "create with every field",
			op:   "create",
			args: []string{
				"--url", "https://example.com/hook",
				"--trigger", "commit", "--trigger", "tag:create",
				// Nullable list and map flags take a YAML literal. A repeated
				// --include-prompt keeps only the last value.
				"--include-prompt", "[" + testPromptID + "]",
				"--headers", "{Authorization: Bearer secret}",
			},
			wantMethod: "POST",
			wantPath:   "/api/v1/prompt-webhooks",
			wantBody:   `{"url":"https://example.com/hook","triggers":["commit","tag:create"],"include_prompts":["` + testPromptID + `"],"headers":{"Authorization":"Bearer secret"}}`,
			response:   testWebhook,
			wantStdout: `"id": "` + testWebhookID + `"`,
		},
		{
			// Runs after the create case, so it also proves flag values do
			// not carry over between invocations.
			name:       "update sends only the flags given",
			op:         "update",
			args:       []string{"--webhook-id", testWebhookID, "--url", "https://example.com/new"},
			wantMethod: "PATCH",
			wantPath:   "/api/v1/prompt-webhooks/" + testWebhookID,
			wantBody:   `{"url":"https://example.com/new"}`,
			response:   testWebhook,
			wantStdout: testWebhookID,
		},
		{
			name:       "retrieve",
			op:         "retrieve",
			args:       []string{"--webhook-id", testWebhookID},
			wantMethod: "GET",
			wantPath:   "/api/v1/prompt-webhooks/" + testWebhookID,
			response:   testWebhook,
			wantStdout: "https://example.com/hook",
		},
		{
			name:         "retrieve reports API errors",
			op:           "retrieve",
			args:         []string{"--webhook-id", testWebhookID},
			wantMethod:   "GET",
			wantPath:     "/api/v1/prompt-webhooks/" + testWebhookID,
			status:       404,
			response:     `{"detail":"Webhook not found"}`,
			wantExitCode: 1,
			wantStderr:   "404 Not Found",
		},
		{
			name:        "list in a workspace",
			op:          "list",
			args:        []string{"--workspace", "44444444-4444-4444-4444-444444444444"},
			wantMethod:  "GET",
			wantPath:    "/api/v1/prompt-webhooks",
			wantHeaders: map[string]string{"X-Tenant-Id": "44444444-4444-4444-4444-444444444444"},
			response:    "[" + testWebhook + "]",
			wantStdout:  testWebhookID,
		},
		{
			name:       "delete after confirmation",
			op:         "delete",
			args:       []string{"--webhook-id", testWebhookID},
			stdin:      "y\n",
			wantMethod: "DELETE",
			wantPath:   "/api/v1/prompt-webhooks/" + testWebhookID,
			wantStderr: "This permanently deletes",
		},
		{
			name:       "delete with --yes skips the prompt",
			op:         "delete",
			args:       []string{"--webhook-id", testWebhookID, "--yes"},
			wantMethod: "DELETE",
			wantPath:   "/api/v1/prompt-webhooks/" + testWebhookID,
		},
		{
			name:         "--yes is rejected outside delete",
			op:           "update",
			args:         []string{"--webhook-id", testWebhookID, "--yes"},
			wantRequests: 0,
			wantExitCode: 1,
			wantStderr:   "yes",
		},
		{
			name: "test sends the webhook and a sample payload",
			op:   "test",
			args: []string{
				"--webhook.url", "https://example.com/hook",
				"--payload.commit-hash", "abc123",
				"--payload.created-at", "2026-09-30T00:00:00Z",
				"--payload.created-by", "someone",
				"--payload.event", "commit",
				"--payload.manifest", "{}",
				"--payload.prompt-id", testPromptID,
				"--payload.prompt-name", "my-prompt",
			},
			wantMethod: "POST",
			wantPath:   "/api/v1/prompt-webhooks/test",
			wantBody:   `{"webhook":{"url":"https://example.com/hook"},"payload":{"commit_hash":"abc123","created_at":"2026-09-30T00:00:00Z","created_by":"someone","event":"commit","manifest":{},"prompt_id":"` + testPromptID + `","prompt_name":"my-prompt"}}`,
			response:   `{"message":"Test notification sent successfully"}`,
			wantStdout: "Test notification sent successfully",
		},
	}
}
