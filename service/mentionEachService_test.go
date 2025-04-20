package service

import (
	"bytes"
	"encoding/json" // Import encoding/json
	// "errors"       // Import errors if using assert.AnError for queue mock
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Real-Dev-Squad/discord-service/config" // Import config
	"github.com/Real-Dev-Squad/discord-service/dtos"
	"github.com/Real-Dev-Squad/discord-service/queue"
	"github.com/Real-Dev-Squad/discord-service/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

// setupServiceTest helper (Optional but recommended from previous review)
func setupServiceTest(discordMessage *dtos.DiscordMessage) (*http.Request, *httptest.ResponseRecorder, *CommandService) {
	req, _ := http.NewRequest("POST", "/mention-each", bytes.NewBuffer([]byte("{}")))
	rr := httptest.NewRecorder()
	commandService := &CommandService{discordMessage: discordMessage}
	return req, rr, commandService
}

func TestMentionEachService(t *testing.T) {

	// --- Mock Setup ---
	originalSendMessage := queue.SendMessage
	// Save original config state for the feature flag
	originalFeatureFlagState := config.AppConfig.MENTION_EACH_ENABLED

	// Use t.Cleanup for reliable restoration after each sub-test (t.Run)
	t.Cleanup(func() {
		queue.SendMessage = originalSendMessage
		config.AppConfig.MENTION_EACH_ENABLED = originalFeatureFlagState // Restore flag
	})
	// --- End Mock Setup ---

	// --- Reusable Test Data Setup ---
	roleID := "123456789"
	createDefaultDiscordMessage := func(options []*discordgo.ApplicationCommandInteractionDataOption) *dtos.DiscordMessage {
		// Add Member.Permissions for the permission check test case
		perms := int64(discordgo.PermissionSendMessages | discordgo.PermissionMentionEveryone) // Default to having perms for success cases
		return &dtos.DiscordMessage{
			Data: &dtos.Data{
				GuildId: "876543210987654321",
				ApplicationCommandInteractionData: discordgo.ApplicationCommandInteractionData{
					Name:    utils.CommandNames.MentionEach,
					Options: options,
				},
			},
			ChannelId: "987654321",
			Member: &discordgo.Member{
				User:        &discordgo.User{ID: "user123"},
				Permissions: perms, // Include permissions
			},
		}
	}
	// --- End Test Data Setup ---

	// --- Tests for when FEATURE FLAG is ENABLED ---
	t.Run("when enabled, should queue message with role option only", func(t *testing.T) {
		// ARRANGE: Enable Feature Flag
		config.AppConfig.MENTION_EACH_ENABLED = true

		opts := []*discordgo.ApplicationCommandInteractionDataOption{{Name: "role", Value: roleID}}
		discordMessage := createDefaultDiscordMessage(opts)
		var capturedPacket *dtos.DataPacket
		queue.SendMessage = func(message []byte) error { /* ... capture ... */
			packetData := &dtos.DataPacket{}
			err := packetData.FromByte(message)
			assert.NoError(t, err)
			capturedPacket = packetData
			return nil
		}
		req, rr, commandService := setupServiceTest(discordMessage)

		// ACT
		commandService.MentionEachService(rr, req)

		// ASSERT
		expectedSubString := "Mentioning all users with the \\u003c@\\u0026" + roleID + "\\u003e"
		assert.Contains(t, rr.Body.String(), expectedSubString)
		assert.NotNil(t, capturedPacket) // Check queue was called
		// ... other packet assertions ...
		assert.Equal(t, roleID, capturedPacket.MetaData["role_id"])
		assert.Equal(t, "false", capturedPacket.MetaData["dev"])
		assert.Equal(t, "false", capturedPacket.MetaData["dev_title"])
	})

	t.Run("when enabled, should include optional parameters when provided", func(t *testing.T) {
		// ARRANGE: Enable Feature Flag
		config.AppConfig.MENTION_EACH_ENABLED = true

		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "message", Value: "Hello everyone!"},
			{Name: "dev", Value: true},
			{Name: "dev_title", Value: true},
		}
		discordMessage := createDefaultDiscordMessage(opts)
		var capturedPacket *dtos.DataPacket
		queue.SendMessage = func(message []byte) error { /* ... capture ... */
			packetData := &dtos.DataPacket{}
			err := packetData.FromByte(message)
			assert.NoError(t, err)
			capturedPacket = packetData
			return nil
		}
		req, rr, commandService := setupServiceTest(discordMessage)

		// ACT
		commandService.MentionEachService(rr, req)

		// ASSERT
		expectedSubString := "Fetching users with the \\u003c@\\u0026" + roleID + "\\u003e" // dev_title takes precedence
		assert.Contains(t, rr.Body.String(), expectedSubString)
		assert.NotNil(t, capturedPacket)
		assert.Equal(t, "Hello everyone!", capturedPacket.MetaData["message"])
		assert.Equal(t, "true", capturedPacket.MetaData["dev"])
		assert.Equal(t, "true", capturedPacket.MetaData["dev_title"])
	})

	t.Run("when enabled, should set correct response content for dev=true", func(t *testing.T) {
		// ARRANGE: Enable Feature Flag
		config.AppConfig.MENTION_EACH_ENABLED = true

		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "message", Value: "Dev message"},
			{Name: "dev", Value: true},
		}
		discordMessage := createDefaultDiscordMessage(opts)
		var capturedPacket *dtos.DataPacket
		queue.SendMessage = func(message []byte) error { /* ... capture ... */
			packetData := &dtos.DataPacket{}
			err := packetData.FromByte(message)
			assert.NoError(t, err)
			capturedPacket = packetData
			return nil
		}
		req, rr, commandService := setupServiceTest(discordMessage)

		// ACT
		commandService.MentionEachService(rr, req)

		// ASSERT
		expectedSubString := "Sending individual mentions to users with the \\u003c@\\u0026" + roleID + "\\u003e"
		assert.Contains(t, rr.Body.String(), expectedSubString)
		assert.NotNil(t, capturedPacket)
		assert.Equal(t, "Dev message", capturedPacket.MetaData["message"])
		assert.Equal(t, "true", capturedPacket.MetaData["dev"])
		assert.Equal(t, "false", capturedPacket.MetaData["dev_title"])
	})

	// --- Tests for expected failures WHEN ENABLED ---
	t.Run("when enabled, should handle queue errors", func(t *testing.T) {
		config.AppConfig.MENTION_EACH_ENABLED = true
		opts := []*discordgo.ApplicationCommandInteractionDataOption{{Name: "role", Value: roleID}}
		discordMessage := createDefaultDiscordMessage(opts)
		queue.SendMessage = func(message []byte) error { return assert.AnError } // Mock queue failure
		req, rr, commandService := setupServiceTest(discordMessage)

		commandService.MentionEachService(rr, req)

		assert.Contains(t, rr.Body.String(), "Failed to process your request") // Check error response
	})

	t.Run("when enabled, should handle missing role option", func(t *testing.T) {
		config.AppConfig.MENTION_EACH_ENABLED = true
		discordMessage := createDefaultDiscordMessage([]*discordgo.ApplicationCommandInteractionDataOption{}) // No options
		req, rr, commandService := setupServiceTest(discordMessage)

		commandService.MentionEachService(rr, req)

		assert.Contains(t, rr.Body.String(), "Role is required")
	})

	t.Run("when enabled, should handle invalid role format (non-string)", func(t *testing.T) {
		config.AppConfig.MENTION_EACH_ENABLED = true
		opts := []*discordgo.ApplicationCommandInteractionDataOption{{Name: "role", Value: 12345}} // Invalid type
		discordMessage := createDefaultDiscordMessage(opts)
		req, rr, commandService := setupServiceTest(discordMessage)

		commandService.MentionEachService(rr, req)

		assert.Contains(t, rr.Body.String(), "Invalid role format")
	})

	t.Run("when enabled, should handle invalid role format (empty string)", func(t *testing.T) {
		config.AppConfig.MENTION_EACH_ENABLED = true
		opts := []*discordgo.ApplicationCommandInteractionDataOption{{Name: "role", Value: ""}} // Empty string
		discordMessage := createDefaultDiscordMessage(opts)
		req, rr, commandService := setupServiceTest(discordMessage)

		commandService.MentionEachService(rr, req)

		assert.Contains(t, rr.Body.String(), "Invalid role format (empty ID)")
	})

	// --- NEW TEST: Feature Flag DISABLED ---
	t.Run("should return disabled error when feature flag is off", func(t *testing.T) {
		// ARRANGE: Ensure Feature Flag is FALSE
		config.AppConfig.MENTION_EACH_ENABLED = false

		// Setup basic valid options (doesn't matter much as it should fail early)
		opts := []*discordgo.ApplicationCommandInteractionDataOption{{Name: "role", Value: roleID}}
		discordMessage := createDefaultDiscordMessage(opts)
		queueCalled := false // Track if queue was called (it shouldn't be)
		queue.SendMessage = func(message []byte) error { queueCalled = true; return nil }
		req, rr, commandService := setupServiceTest(discordMessage)

		// ACT
		commandService.MentionEachService(rr, req)

		// ASSERT
		assert.Contains(t, rr.Body.String(), "command is currently disabled") // Check for specific disabled message

		// Check that the response was ephemeral
		var resp discordgo.InteractionResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err, "Should be able to unmarshal response")
		if err == nil {
			assert.NotNil(t, resp.Data, "Response data should not be nil")
			if resp.Data != nil {
				assert.Equal(t, discordgo.MessageFlagsEphemeral, resp.Data.Flags, "Error response should be ephemeral")
			}
		}
		assert.False(t, queueCalled, "queue.SendMessage should NOT have been called")
	})

	// --- Nil checks happen before feature flag check, so keep them as is ---
	t.Run("should handle nil checks", func(t *testing.T) {
		// --- ARRANGE: Ensure Feature Flag is ENABLED for this test ---
		// So that execution proceeds PAST the flag check to the nil checks.
		config.AppConfig.MENTION_EACH_ENABLED = true

		// Setup shared request/recorder
		req, _ := http.NewRequest("POST", "/mention-each", bytes.NewBuffer([]byte{}))
		var rr *httptest.ResponseRecorder // Declare rr outside loop

		// --- Test with nil discordMessage ---
		rr = httptest.NewRecorder() // Reset recorder for each case
		commandService := &CommandService{discordMessage: nil}
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid request data") // Expect nil data error

		// --- Test with nil Data ---
		rr = httptest.NewRecorder()
		discordMessage := createDefaultDiscordMessage(nil) // Use helper, options don't matter
		discordMessage.Data = nil                          // Set field to nil
		commandService = &CommandService{discordMessage: discordMessage}
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid request data")

		// --- Test with nil Member ---
		rr = httptest.NewRecorder()
		discordMessage = createDefaultDiscordMessage(nil)
		discordMessage.Member = nil // Set field to nil
		commandService = &CommandService{discordMessage: discordMessage}
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid request data")

		// --- Test with nil User ---
		rr = httptest.NewRecorder()
		discordMessage = createDefaultDiscordMessage(nil)
		// Need to ensure Member is not nil before setting User to nil
		if discordMessage.Member == nil {
			discordMessage.Member = &discordgo.Member{}
		}
		discordMessage.Member.User = nil // Set field to nil
		commandService = &CommandService{discordMessage: discordMessage}
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid request data")
	})
}

func TestFindOption(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "role", Value: "role-id-123"},
		{Name: "message", Value: "Hello!"},
	}

	t.Run("should find option when present", func(t *testing.T) {
		option := findOption(options, "message")
		assert.NotNil(t, option)
		assert.Equal(t, "message", option.Name)
		assert.Equal(t, "Hello!", option.Value)
	})

	t.Run("should return nil when option not found", func(t *testing.T) {
		option := findOption(options, "nonexistent")
		assert.Nil(t, option)
	})

	t.Run("should handle empty options", func(t *testing.T) {
		option := findOption([]*discordgo.ApplicationCommandInteractionDataOption{}, "role")
		assert.Nil(t, option)
	})
}
