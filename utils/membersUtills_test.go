package utils

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Real-Dev-Squad/discord-service/tests/mocks"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

// TestGetUsersWithRole tests the GetUsersWithRole function which is responsible for
// fetching members with a specific role, handling pagination implicitly via the session interface.
// It uses MockDiscordSession to simulate responses from the Discord API (GuildMembers call).
func TestGetUsersWithRole(t *testing.T) {
	guildID := "testGuild"
	roleID := "testRole"

	member1 := &discordgo.Member{User: &discordgo.User{ID: "123"}, Roles: []string{roleID}}
	member2 := &discordgo.Member{User: &discordgo.User{ID: "456"}, Roles: []string{"otherRole"}}
	member3 := &discordgo.Member{User: &discordgo.User{ID: "789"}, Roles: []string{roleID, "anotherRole"}}

	var memberPtrNilUser *discordgo.Member = &discordgo.Member{User: nil, Roles: []string{roleID}}
	var memberPtrNil *discordgo.Member = nil
	var emptyMemberListPtr []*discordgo.Member

	memberVal1 := *member1
	memberVal3 := *member3

	// utils/members_utils_test.go

	// Inside TestGetUsersWithRole
	t.Run("returns single user with matching role", func(t *testing.T) {
		mockSess := new(mocks.DiscordSession)
		membersInput := []*discordgo.Member{member1, member2} // member1.ID="123", member2.ID="456"
		expectedResultVal := []discordgo.Member{memberVal1}

		// --- Mock Expectations ---
		// 1. Expect the first call
		mockSess.On("GuildMembers", guildID, "", DISCORD_GUILD_MEMBER_API_LIMIT).Return(membersInput, nil).Once()
		// 2. Expect the second call (after processing page 1) with after=ID of last member ("456")
		//    Mock it to return an empty list to terminate the loop.
		mockSess.On("GuildMembers", guildID, member2.User.ID, DISCORD_GUILD_MEMBER_API_LIMIT).Return(emptyMemberListPtr, nil).Once()
		// --- End Mock Expectations ---

		result, err := GetUsersWithRole(mockSess, guildID, roleID)

		assert.NoError(t, err)
		assert.Len(t, result, 1) // Still expect only member1 to be filtered in
		assert.Equal(t, expectedResultVal, result)
		mockSess.AssertExpectations(t) // Verify *both* calls were made
	})

	// ... rest of tests ...

	t.Run("returns multiple users with matching role", func(t *testing.T) {
		mockSess := new(mocks.DiscordSession)
		membersInput := []*discordgo.Member{member1, member2, member3} // IDs: "123", "456", "789"
		expectedResultVal := []discordgo.Member{memberVal1, memberVal3}

		// --- Mock Expectations ---
		// 1. Expect the first call
		mockSess.On("GuildMembers", guildID, "", DISCORD_GUILD_MEMBER_API_LIMIT).Return(membersInput, nil).Once()
		// 2. Expect the second call with after=ID of last member ("789")
		//    Mock it to return an empty list to terminate the loop.
		mockSess.On("GuildMembers", guildID, member3.User.ID, DISCORD_GUILD_MEMBER_API_LIMIT).Return(emptyMemberListPtr, nil).Once()
		// --- End Mock Expectations ---

		result, err := GetUsersWithRole(mockSess, guildID, roleID)

		assert.NoError(t, err)
		assert.Len(t, result, 2) // Expect 2 members (member1, member3)
		// Use ElementsMatch for concise checking of slice contents regardless of order
		assert.ElementsMatch(t, expectedResultVal, result)
		mockSess.AssertExpectations(t) // Verify *both* calls were made
	})

	t.Run("handles error from GuildMembers", func(t *testing.T) {
		mockSess := new(mocks.DiscordSession) // Fix: Use mockSession instead of MockDiscordSession
		mockErr := errors.New("API error")
		mockSess.On("GuildMembers", guildID, "", DISCORD_GUILD_MEMBER_API_LIMIT).Return(nil, mockErr).Once() // Fix: Use mockSess variable

		_, err := GetUsersWithRole(mockSess, guildID, roleID) // Fix: Use mockSess variable

		assert.Error(t, err)
		assert.ErrorContains(t, err, mockErr.Error())
		mockSess.AssertExpectations(t) // Fix: Use mockSess variable
	})

	// utils/members_utils_test.go

	// Inside TestGetUsersWithRole
	t.Run("returns empty slice when no users have the role", func(t *testing.T) {
		mockSess := new(mocks.DiscordSession)        // Fix: Use mockSession instead of MockDiscordSession
		membersInput := []*discordgo.Member{member2} // Contains only member2 (ID "456", no target role)

		// --- Mock Expectations ---\n\t\t// 1. Expect the first call
		mockSess.On("GuildMembers", guildID, "", DISCORD_GUILD_MEMBER_API_LIMIT).Return(membersInput, nil).Once() // Fix: Use mockSess variable
		// 2. Expect the second call with after=ID of last member ("456")
		//    Mock it to return an empty list to terminate the loop.
		mockSess.On("GuildMembers", guildID, member2.User.ID, DISCORD_GUILD_MEMBER_API_LIMIT).Return(emptyMemberListPtr, nil).Once() // Fix: Use mockSess variable
		// --- End Mock Expectations ---\n
		result, err := GetUsersWithRole(mockSess, guildID, roleID) // Fix: Use mockSess variable

		assert.NoError(t, err)
		assert.Empty(t, result)        // The final result should still be empty
		mockSess.AssertExpectations(t) // Fix: Use mockSess variable // Verify *both* calls were made
	})

	t.Run("ignore invalid data during filtering", func(t *testing.T) {
		mockSess := new(mocks.DiscordSession)
		// Input mock still uses pointers, including nils
		membersInputPtr := []*discordgo.Member{member1, memberPtrNilUser, memberPtrNil, member3}
		// --- Mock Expectations ---
		mockSess.On("GuildMembers", guildID, "", 1000).Return(membersInputPtr, nil).Once()
		mockSess.On("GuildMembers", guildID, member3.User.ID, 1000).Return(emptyMemberListPtr, nil).Once()

		result, err := GetUsersWithRole(mockSess, guildID, roleID)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// --- Assert using IDs ---
		expectedIDs := []string{"123", "789"}
		var actualIDs []string
		for _, m := range result {
			// No need for nil check on m.User here, as the length
			// and expectedIDs already imply only valid ones should be in result.
			// If m itself could be a zero-value struct, might need a check.
			if m.User != nil { // Keep safety check just in case
				actualIDs = append(actualIDs, m.User.ID)
			}
		}
		assert.ElementsMatch(t, expectedIDs, actualIDs)
		// --- End Assert using IDs ---
		mockSess.AssertExpectations(t)
	})

	// ... rest of tests ...

	t.Run("handles empty member list from GuildMembers", func(t *testing.T) {
		mockSess := new(mocks.DiscordSession)                                                                           // Fix: Use mockSession instead of MockDiscordSession
		mockSess.On("GuildMembers", guildID, "", DISCORD_GUILD_MEMBER_API_LIMIT).Return(emptyMemberListPtr, nil).Once() // Fix: Use mockSess variable // Use var
		result, err := GetUsersWithRole(mockSess, guildID, roleID)                                                      // Fix: Use mockSess variable
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockSess.AssertExpectations(t) // Fix: Use mockSess variable
	})
	// utils/members_utils_test.go

	// Inside TestGetUsersWithRole
	// Verifies that the function correctly handles pagination by making multiple
	// calls to GuildMembers with the appropriate 'after' ID until an empty chunk
	// is returned, accumulating members with the target role across pages.
	t.Run("handles pagination correctly", func(t *testing.T) {
		// --- Arrange ---
		mockSess := new(mocks.DiscordSession) // Re-ensure mockSess is defined here
		// Use distinct IDs for clarity, ensure roleID exists in test data
		guildID := "paginationGuild"
		roleID := "pageRole"
		limit := DISCORD_GUILD_MEMBER_API_LIMIT // Match the limit used in the function

		// Define members for the first page (fewer than limit to test loop continuation)
		memberP1R1 := &discordgo.Member{User: &discordgo.User{ID: "p1u1"}, Roles: []string{roleID}}
		memberP1Other := &discordgo.Member{User: &discordgo.User{ID: "p1u2"}, Roles: []string{"other"}}
		memberP1R2 := &discordgo.Member{User: &discordgo.User{ID: "p1u3"}, Roles: []string{roleID, "another"}} // Last member of page 1
		membersPage1 := []*discordgo.Member{memberP1R1, memberP1Other, memberP1R2}

		// Define members for the second page
		memberP2R1 := &discordgo.Member{User: &discordgo.User{ID: "p2u1"}, Roles: []string{roleID}}
		memberP2R2 := &discordgo.Member{User: &discordgo.User{ID: "p2u2"}, Roles: []string{roleID}} // Last member of page 2
		membersPage2 := []*discordgo.Member{memberP2R1, memberP2R2}

		// Define an empty list for the final API call
		var emptyMemberList []*discordgo.Member

		// --- Mock Expectations ---
		// 1. Expect first call with after=\"\" -> returns page 1 members
		mockSess.On("GuildMembers", guildID, "", limit).Return(membersPage1, nil).Once()
		// 2. Expect second call with after=\"p1u3\" (last ID from page 1) -> returns page 2 members
		mockSess.On("GuildMembers", guildID, memberP1R2.User.ID, limit).Return(membersPage2, nil).Once()
		// 3. Expect third call with after=\"p2u2\" (last ID from page 2) -> returns empty list to stop loop
		mockSess.On("GuildMembers", guildID, memberP2R2.User.ID, limit).Return(emptyMemberList, nil).Once()

		// --- Act ---
		result, err := GetUsersWithRole(mockSess, guildID, roleID)

		// --- Assert ---
		assert.NoError(t, err)
		// Expect 4 members total (p1u1, p1u3 from page 1; p2u1, p2u2 from page 2)
		assert.Len(t, result, 4)

		// Verify the correct members were collected using ElementsMatch (ignores order)
		expectedIDs := []string{"p1u1", "p1u3", "p2u1", "p2u2"}
		var actualIDs []string
		for _, m := range result {
			if m.User != nil { // Safety check
				actualIDs = append(actualIDs, m.User.ID)
			}
		}
		assert.ElementsMatch(t, expectedIDs, actualIDs)

		// Verify that all expected mock calls were made
		mockSess.AssertExpectations(t)
	})

	// ... rest of tests ...

}

// TestFormatUserMentions tests the utility function for converting member objects
// into Discord mention strings.
func TestFormatUserMentions(t *testing.T) {
	t.Run("formats user Mentions correctly", func(t *testing.T) {
		members := []discordgo.Member{
			{User: &discordgo.User{ID: "123"}},
			{User: &discordgo.User{ID: "456"}},
		}

		mentions := FormatUserMentions(members)
		assert.Equal(t, []string{"<@123>", "<@456>"}, mentions)
	})
	t.Run("handles empty member list", func(t *testing.T) {
		var members []discordgo.Member
		mentions := FormatUserMentions(members)
		assert.Empty(t, mentions)
	})

	t.Run("handles nil members list", func(t *testing.T) {
		mentions := FormatUserMentions(nil)
		assert.Empty(t, mentions)
	})
	t.Run("skips members with nil User", func(t *testing.T) {
		members := []discordgo.Member{
			{User: &discordgo.User{ID: "123"}},
			{User: nil},
			{User: &discordgo.User{ID: "456"}},
		}
		mentions := FormatUserMentions(members)
		assert.Equal(t, []string{"<@123>", "<@456>"}, mentions)
		assert.Len(t, mentions, 2)
	})

}

// TestFormatMentionResponse tests the utility function for creating the final message
// content for the standard mention-each mode.
func TestFormatMentionResponse(t *testing.T) {
	t.Run("formats response with message and mentions", func(t *testing.T) {
		mentions := []string{"<@123>", "<@456>"}
		message := "Hello"
		response := FormatMentionResponse(mentions, message)
		assert.Equal(t, "Hello <@123> <@456>", response)
	})

	t.Run("formats response with only mentions", func(t *testing.T) {
		mentions := []string{"<@123>", "<@456>"}
		response := FormatMentionResponse(mentions, "")
		assert.Equal(t, "<@123> <@456>", response)
	})
}
func TestFormatUserListResponse(t *testing.T) {
	roleID := "123456789"
	roleMention := "<@&" + roleID + ">"

	t.Run("formats response with no users", func(t *testing.T) {
		response := FormatUserListResponse([]string{}, roleID)
		expected := fmt.Sprintf("Found 0 users with the %s role", roleMention)
		assert.Equal(t, expected, response)
	})

	t.Run("formats response with single user", func(t *testing.T) {
		mentions := []string{"<@123>"}
		response := FormatUserListResponse(mentions, roleID)
		expected := fmt.Sprintf("Found 1 user with the %s role: %s", roleMention, mentions[0])
		assert.Equal(t, expected, response)
	})

	t.Run("formats response with multiple users", func(t *testing.T) {
		roleID := "123456789"
		roleMention := "<@&" + roleID + ">"
		mentions := []string{"<@123>", "<@456>"}
		response := FormatUserListResponse(mentions, roleID)
		expected := fmt.Sprintf("Found %d users with the %s role: %s", len(mentions), roleMention, strings.Join(mentions, ", "))
		assert.Equal(t, expected, response)
	})

	t.Run("handles nil mentions", func(t *testing.T) {
		response := FormatUserListResponse(nil, roleID)
		expected := fmt.Sprintf("Found 0 users with the %s role", roleMention)
		assert.Equal(t, expected, response)
	})

	t.Run("handles empty role ID", func(t *testing.T) {
		mentions := []string{"<@123>"}
		emptyRoleMention := "<@&>"
		response := FormatUserListResponse([]string{"<@123>"}, "")
		expected := fmt.Sprintf("Found 1 user with the %s role: %s", emptyRoleMention, mentions[0])
		assert.Equal(t, expected, response)
	})
}
