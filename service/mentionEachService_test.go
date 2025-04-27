package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// No config import needed for this flag approach
	"github.com/Real-Dev-Squad/discord-service/dtos"
	"github.com/Real-Dev-Squad/discord-service/queue"
	_ "github.com/Real-Dev-Squad/discord-service/tests/helpers"
	"github.com/Real-Dev-Squad/discord-service/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

// setupServiceTest helper remains the same
func setupServiceTest(discordMessage *dtos.DiscordMessage) (*http.Request, *httptest.ResponseRecorder, *CommandService) {
	req, _ := http.NewRequest("POST", "/mention-each", bytes.NewBuffer([]byte("{}")))
	rr := httptest.NewRecorder()
	commandService := &CommandService{discordMessage: discordMessage}
	return req, rr, commandService
}

func TestMentionEachService(t *testing.T) {

	// --- Mock Setup for Queue ---
	originalSendMessage := queue.SendMessage
	t.Cleanup(func() {
		queue.SendMessage = originalSendMessage
	})
	// --- End Mock Setup ---

	// --- Reusable Test Data Setup ---
	roleID := "123456789"
	// Helper now creates the base message, options are added per test case
	createDefaultDiscordMessage := func(options []*discordgo.ApplicationCommandInteractionDataOption) *dtos.DiscordMessage {
		// Base member - NO permission check needed in this version
		member := &discordgo.Member{
			User: &discordgo.User{ID: "user123"},
			// Permissions field not needed if not checked in service
		}
		return &dtos.DiscordMessage{
			Data: &dtos.Data{
				GuildId: "guild123",
				ApplicationCommandInteractionData: discordgo.ApplicationCommandInteractionData{
					Name:    utils.CommandNames.MentionEach,
					Options: options, // Pass provided options
				},
			},
			ChannelId: "chan123",
			Member:    member,
		}
	}
	// --- End Test Data Setup ---

	// --- Test Cases ---

	t.Run("should queue message when ff_enabled=true and only role provided", func(t *testing.T) {
		// Arrange: Options include role + ff_enabled=true
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "ff_enabled", Value: true}, // Explicitly enable via option
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

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect normal ack, check queue packet
		expectedSubString := "Mentioning all users with the \\u003c@\\u0026" + roleID + "\\u003e" // Default response
		assert.Contains(t, rr.Body.String(), expectedSubString)
		assert.NotNil(t, capturedPacket)
		assert.Equal(t, roleID, capturedPacket.MetaData["role_id"])
		// Booleans default to false if not provided
		assert.Equal(t, "false", capturedPacket.MetaData["dev"])
		assert.Equal(t, "false", capturedPacket.MetaData["dev_title"])
	})

	t.Run("should include optional params when ff_enabled=true", func(t *testing.T) {
		// Arrange: Include message, dev, dev_title AND ff_enabled=true
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "message", Value: "Hello everyone!"},
			{Name: "dev", Value: true},
			{Name: "dev_title", Value: true},
			{Name: "ff_enabled", Value: true}, // Explicitly enable via option
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

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect dev_title ack, check queue packet
		expectedSubString := "Fetching users with the \\u003c@\\u0026" + roleID + "\\u003e"
		assert.Contains(t, rr.Body.String(), expectedSubString)
		assert.NotNil(t, capturedPacket)
		assert.Equal(t, "Hello everyone!", capturedPacket.MetaData["message"])
		assert.Equal(t, "true", capturedPacket.MetaData["dev"])
		assert.Equal(t, "true", capturedPacket.MetaData["dev_title"])
	})

	// --- Test for Disabled Case (Flag Option Missing) ---
	t.Run("should return disabled error when ff_enabled option is missing", func(t *testing.T) {
		// Arrange: Only include role option. ff_enabled is absent.
		opts := []*discordgo.ApplicationCommandInteractionDataOption{{Name: "role", Value: roleID}}
		discordMessage := createDefaultDiscordMessage(opts) // ff_enabled not added

		queueCalled := false
		queue.SendMessage = func(message []byte) error { queueCalled = true; return nil }
		req, rr, commandService := setupServiceTest(discordMessage)

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect disabled message, ephemeral, queue not called
		assert.Contains(t, rr.Body.String(), "command requires the `ff_enabled:True` option") // Check specific disabled message
		var resp discordgo.InteractionResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		if err == nil && resp.Data != nil {
			assert.Equal(t, discordgo.MessageFlagsEphemeral, resp.Data.Flags, "Error response should be ephemeral")
		}
		assert.False(t, queueCalled, "queue.SendMessage should NOT have been called")
	})

	// --- Test for Disabled Case (Flag Option False) ---
	t.Run("should return disabled error when ff_enabled option is false", func(t *testing.T) {
		// Arrange: Explicitly include ff_enabled=false
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "ff_enabled", Value: false}, // Explicitly false
		}
		discordMessage := createDefaultDiscordMessage(opts) // Pass raw opts

		queueCalled := false
		queue.SendMessage = func(message []byte) error { queueCalled = true; return nil }
		req, rr, commandService := setupServiceTest(discordMessage)

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect disabled message, ephemeral, queue not called
		assert.Contains(t, rr.Body.String(), "command requires the `ff_enabled:True` option")
		var resp discordgo.InteractionResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		if err == nil && resp.Data != nil {
			assert.Equal(t, discordgo.MessageFlagsEphemeral, resp.Data.Flags, "Error response should be ephemeral")
		}
		assert.False(t, queueCalled, "queue.SendMessage should NOT have been called")
	})

	// --- Tests for other errors (should only run if ff_enabled=true) ---
	t.Run("should handle queue errors when ff_enabled=true", func(t *testing.T) {
		// Arrange: Need ff_enabled=true to get past the first check
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "ff_enabled", Value: true},
		}
		discordMessage := createDefaultDiscordMessage(opts)
		queue.SendMessage = func(message []byte) error { return assert.AnError } // Mock queue failure
		req, rr, commandService := setupServiceTest(discordMessage)

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect the actual queue error response
		assert.Contains(t, rr.Body.String(), "Failed to process your request")
	})

	t.Run("should handle missing role option even if ff_enabled=true", func(t *testing.T) {
		// Arrange: ff_enabled=true, but no role option
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "ff_enabled", Value: true}, // Only ff_enabled provided
		}
		discordMessage := createDefaultDiscordMessage(opts)
		req, rr, commandService := setupServiceTest(discordMessage)

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect role required error
		assert.Contains(t, rr.Body.String(), "Role is required")
	})

	t.Run("should handle invalid role format (non-string) when ff_enabled=true", func(t *testing.T) {
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: 12345},
			{Name: "ff_enabled", Value: true},
		}
		discordMessage := createDefaultDiscordMessage(opts)
		req, rr, commandService := setupServiceTest(discordMessage)
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid role format")
	})

	t.Run("should handle invalid role format (empty string) when ff_enabled=true", func(t *testing.T) {
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: ""},
			{Name: "ff_enabled", Value: true},
		}
		discordMessage := createDefaultDiscordMessage(opts)
		req, rr, commandService := setupServiceTest(discordMessage)
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid role format (empty ID)")
	})

	// Optional: Add test for invalid ff_enabled value type
	t.Run("should handle invalid ff_enabled type (defaults to disabled)", func(t *testing.T) {
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID},
			{Name: "ff_enabled", Value: "not-a-bool"}, // Invalid type
		}
		discordMessage := createDefaultDiscordMessage(opts) // Pass raw opts

		queueCalled := false
		queue.SendMessage = func(message []byte) error { queueCalled = true; return nil }
		req, rr, commandService := setupServiceTest(discordMessage)

		// Act
		commandService.MentionEachService(rr, req)

		// Assert: Expect disabled message because invalid type defaults to false
		assert.Contains(t, rr.Body.String(), "command requires the `ff_enabled:True` option")
		assert.False(t, queueCalled, "queue.SendMessage should NOT have been called")
	})

	// --- Nil checks still happen after feature flag check now ---
	t.Run("should handle nil checks when ff_enabled=true", func(t *testing.T) {
		// Arrange: Enable flag first
		opts := []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "role", Value: roleID}, // Need role to pass first validation
			{Name: "ff_enabled", Value: true},
		}

		// Test with nil discordMessage - This specific setup is hard, commandService would be nil.
		// The check `if s.discordMessage == nil` in the service handles this.
		// We can't easily test it via TestMainService routing without modifying TestMainService itself.
		// Assuming the direct nil check in the service is sufficient.

		// Test with nil Data
		req, _ := http.NewRequest("POST", "/mention-each", bytes.NewBuffer([]byte("{}")))
		rr := httptest.NewRecorder()
		discordMessage := createDefaultDiscordMessage(opts)
		discordMessage.Data = nil // Set Data to nil AFTER creating base message
		commandService := &CommandService{discordMessage: discordMessage}
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid request data")

		// Test with nil Member
		rr = httptest.NewRecorder()
		discordMessage = createDefaultDiscordMessage(opts)
		discordMessage.Member = nil
		commandService = &CommandService{discordMessage: discordMessage}
		commandService.MentionEachService(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid request data")

		// Test with nil User
		rr = httptest.NewRecorder()
		discordMessage = createDefaultDiscordMessage(opts)
		if discordMessage.Member == nil {
			discordMessage.Member = &discordgo.Member{}
		} // Ensure Member exists
		discordMessage.Member.User = nil
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
