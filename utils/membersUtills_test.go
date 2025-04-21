package utils

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDiscordSession struct {
	mock.Mock
}

func (m *MockDiscordSession) GuildMembers(guildID, after string, limit int) ([]*discordgo.Member, error) {
	args := m.Called(guildID, after, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if members, ok := args.Get(0).([]*discordgo.Member); ok {
		return members, args.Error(1)
	}
	panic(fmt.Sprintf("mock return value for GuildMembers is not []*discordgo.Member: %T", args.Get(0)))
}

func (m *MockDiscordSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	args := m.Called(channelID, content)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if msg, ok := args.Get(0).(*discordgo.Message); ok {
		return msg, args.Error(1)
	}
	panic(fmt.Sprintf("mock return value for ChannelMessageSend is not *discordgo.Message: %T", args.Get(0)))
}

// TestGetUsersWithRole tests the GetUsersWithRole function which is responsible for
// fetching members with a specific role, handling pagination implicitly via the session interface.
// It uses MockDiscordSession to simulate responses from the Discord API (GuildMembers call).
func TestGetUsersWithRole(t *testing.T) {
	guildID := "testGuild"
	roleID := "testRole"

	member1 := &discordgo.Member{User: &discordgo.User{ID: "123"}, Roles: []string{roleID}}
	member2 := &discordgo.Member{User: &discordgo.User{ID: "456"}, Roles: []string{"otherRole"}}
	member3 := &discordgo.Member{User: &discordgo.User{ID: "789"}, Roles: []string{roleID, "anotherRole"}}

	var memberNilUser *discordgo.Member = &discordgo.Member{User: nil, Roles: []string{roleID}}
	var memberNilRoles *discordgo.Member = &discordgo.Member{User: &discordgo.User{ID: "abc"}, Roles: nil}
	var memberNil *discordgo.Member = nil
	var emptyMemberList []*discordgo.Member

	// utils/members_utils_test.go

	// Inside TestGetUsersWithRole
	t.Run("returns single user with matching role", func(t *testing.T) {
		mockSession := new(MockDiscordSession)
		membersInput := []*discordgo.Member{member1, member2} // member1.ID="123", member2.ID="456"
		var emptyMemberList []*discordgo.Member

		// --- Mock Expectations ---
		// 1. Expect the first call
		mockSession.On("GuildMembers", guildID, "", 1000).Return(membersInput, nil).Once()
		// 2. Expect the second call (after processing page 1) with after=ID of last member ("456")
		//    Mock it to return an empty list to terminate the loop.
		mockSession.On("GuildMembers", guildID, member2.User.ID, 1000).Return(emptyMemberList, nil).Once()
		// --- End Mock Expectations ---

		result, err := GetUsersWithRole(mockSession, guildID, roleID)

		assert.NoError(t, err)
		assert.Len(t, result, 1) // Still expect only member1 to be filtered in
		if len(result) > 0 {
			assert.Equal(t, "123", result[0].User.ID)
		}
		mockSession.AssertExpectations(t) // Verify *both* calls were made
	})

	// ... rest of tests ...

	t.Run("returns multiple users with matching role", func(t *testing.T) {
		mockSession := new(MockDiscordSession)
		membersInput := []*discordgo.Member{member1, member2, member3} // IDs: "123", "456", "789"
		var emptyMemberList []*discordgo.Member

		// --- Mock Expectations ---
		// 1. Expect the first call
		mockSession.On("GuildMembers", guildID, "", 1000).Return(membersInput, nil).Once()
		// 2. Expect the second call with after=ID of last member ("789")
		//    Mock it to return an empty list to terminate the loop.
		mockSession.On("GuildMembers", guildID, member3.User.ID, 1000).Return(emptyMemberList, nil).Once()
		// --- End Mock Expectations ---

		result, err := GetUsersWithRole(mockSession, guildID, roleID)

		assert.NoError(t, err)
		assert.Len(t, result, 2) // Expect 2 members (member1, member3)
		// Use ElementsMatch for concise checking of slice contents regardless of order
		expectedIDs := []string{"123", "789"}
		var actualIDs []string
		for _, m := range result {
			if m != nil && m.User != nil {
				actualIDs = append(actualIDs, m.User.ID)
			}
		}
		assert.ElementsMatch(t, expectedIDs, actualIDs, "Should find members 123 and 789 only")

		mockSession.AssertExpectations(t) // Verify *both* calls were made
	})

	t.Run("handles error from GuildMembers", func(t *testing.T) {
		mockSession := new(MockDiscordSession)
		mockErr := errors.New("API error")
		mockSession.On("GuildMembers", guildID, "", 1000).Return(nil, mockErr).Once()

		_, err := GetUsersWithRole(mockSession, guildID, roleID)

		assert.Error(t, err)
		assert.ErrorContains(t, err, mockErr.Error())
		mockSession.AssertExpectations(t)
	})

	// utils/members_utils_test.go

	// Inside TestGetUsersWithRole
	t.Run("returns empty slice when no users have the role", func(t *testing.T) {
		mockSession := new(MockDiscordSession)
		membersInput := []*discordgo.Member{member2} // Contains only member2 (ID "456", no target role)
		var emptyMemberList []*discordgo.Member

		// --- Mock Expectations ---
		// 1. Expect the first call
		mockSession.On("GuildMembers", guildID, "", 1000).Return(membersInput, nil).Once()
		// 2. Expect the second call with after=ID of last member ("456")
		//    Mock it to return an empty list to terminate the loop.
		mockSession.On("GuildMembers", guildID, member2.User.ID, 1000).Return(emptyMemberList, nil).Once()
		// --- End Mock Expectations ---

		result, err := GetUsersWithRole(mockSession, guildID, roleID)

		assert.NoError(t, err)
		assert.Empty(t, result)           // The final result should still be empty
		mockSession.AssertExpectations(t) // Verify *both* calls were made
	})

	// ... rest of tests ...

	t.Run("handles empty member list from GuildMembers", func(t *testing.T) {
		mockSession := new(MockDiscordSession)
		mockSession.On("GuildMembers", guildID, "", 1000).Return(emptyMemberList, nil).Once() // Use var
		result, err := GetUsersWithRole(mockSession, guildID, roleID)
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockSession.AssertExpectations(t)
	})

	t.Run("skips members with nil User or nil Roles safely", func(t *testing.T) {
		mockSession := new(MockDiscordSession)
		membersInput := []*discordgo.Member{member1, memberNilUser, memberNilRoles, memberNil, member3} // Last ID is "789"
		var emptyMemberList []*discordgo.Member

		// --- Mock Expectations for this test too ---
		// 1. Expect first call
		mockSession.On("GuildMembers", guildID, "", 1000).Return(membersInput, nil).Once()
		// 2. Expect second call with after = ID of last valid member ("789")
		mockSession.On("GuildMembers", guildID, member3.User.ID, 1000).Return(emptyMemberList, nil).Once()

		result, err := GetUsersWithRole(mockSession, guildID, roleID)

		assert.NoError(t, err)
		assert.Len(t, result, 2) // Should only find member1 and member3
		expectedIDs := []string{"123", "789"}
		var actualIDs []string
		for _, m := range result {
			if m != nil && m.User != nil {
				actualIDs = append(actualIDs, m.User.ID)
			}
		}
		assert.ElementsMatch(t, expectedIDs, actualIDs)
		mockSession.AssertExpectations(t) // Verify *both* calls were made
	})
	// utils/members_utils_test.go

	// Inside TestGetUsersWithRole
	// Verifies that the function correctly handles pagination by making multiple
	// calls to GuildMembers with the appropriate 'after' ID until an empty chunk
	// is returned, accumulating members with the target role across pages.
	t.Run("handles pagination correctly", func(t *testing.T) {
		// --- Arrange ---
		mockSession := new(MockDiscordSession)
		// Use distinct IDs for clarity, ensure roleID exists in test data
		guildID := "paginationGuild"
		roleID := "pageRole"
		limit := 1000 // Match the limit used in the function

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
		// 1. Expect first call with after="" -> returns page 1 members
		mockSession.On("GuildMembers", guildID, "", limit).Return(membersPage1, nil).Once()
		// 2. Expect second call with after="p1u3" (last ID from page 1) -> returns page 2 members
		mockSession.On("GuildMembers", guildID, memberP1R2.User.ID, limit).Return(membersPage2, nil).Once()
		// 3. Expect third call with after="p2u2" (last ID from page 2) -> returns empty list to stop loop
		mockSession.On("GuildMembers", guildID, memberP2R2.User.ID, limit).Return(emptyMemberList, nil).Once()

		// --- Act ---
		result, err := GetUsersWithRole(mockSession, guildID, roleID)

		// --- Assert ---
		assert.NoError(t, err)
		// Expect 4 members total (p1u1, p1u3 from page 1; p2u1, p2u2 from page 2)
		assert.Len(t, result, 4)

		// Verify the correct members were collected using ElementsMatch (ignores order)
		expectedIDs := []string{"p1u1", "p1u3", "p2u1", "p2u2"}
		var actualIDs []string
		for _, m := range result {
			if m != nil && m.User != nil { // Safety check
				actualIDs = append(actualIDs, m.User.ID)
			}
		}
		assert.ElementsMatch(t, expectedIDs, actualIDs)

		// Verify that all expected mock calls were made
		mockSession.AssertExpectations(t)
	})

	// ... rest of tests ...

}

// TestFormatUserMentions tests the utility function for converting member objects
// into Discord mention strings.
func TestFormatUserMentions(t *testing.T) {
	t.Run("formats user Mentions correctly", func(t *testing.T) {
		members := []*discordgo.Member{
			{User: &discordgo.User{ID: "123"}},
			{User: &discordgo.User{ID: "456"}},
		}

		mentions := FormatUserMentions(members)
		assert.Equal(t, []string{"<@123>", "<@456>"}, mentions)
	})
	t.Run("handles empty member list", func(t *testing.T) {
		var members []*discordgo.Member
		mentions := FormatUserMentions(members)
		assert.Empty(t, mentions)
	})

	t.Run("handles nil members list", func(t *testing.T) {
		mentions := FormatUserMentions(nil)
		assert.Empty(t, mentions)
	})
	t.Run("skips members with nil User", func(t *testing.T) {
		members := []*discordgo.Member{
			{User: &discordgo.User{ID: "123"}},
			{User: nil},
			{User: &discordgo.User{ID: "456"}},
		}
		mentions := FormatUserMentions(members)
		assert.Equal(t, []string{"<@123>", "<@456>"}, mentions)
		assert.Len(t, mentions, 2)
	})

	t.Run("skips nil member in list", func(t *testing.T) {
		members := []*discordgo.Member{
			{User: &discordgo.User{ID: "123"}},
			nil,
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
	t.Run("formats response with only mentions", func(t *testing.T) {
		mentions := []string{"<@123>", "<@456>"}
		response := FormatMentionResponse(mentions, "")
		assert.Equal(t, "<@123> <@456>", response)
	})
}
func TestFormatDevTitleResponse(t *testing.T) {
	roleID := "123456789"
	roleMention := "<@&" + roleID + ">"

	t.Run("formats response with no users", func(t *testing.T) {
		response := FormatDevTitleResponse([]string{}, roleID)
		expected := fmt.Sprintf("Found 0 users with the %s role", roleMention)
		assert.Equal(t, expected, response)
	})

	t.Run("formats response with single user", func(t *testing.T) {
		mentions := []string{"<@123>"}
		response := FormatDevTitleResponse(mentions, roleID)
		expected := fmt.Sprintf("Found 1 user with the %s role: %s", roleMention, mentions[0])
		assert.Equal(t, expected, response)
	})

	t.Run("formats response with multiple users", func(t *testing.T) {
		roleID := "123456789"
		roleMention := "<@&" + roleID + ">"
		mentions := []string{"<@123>", "<@456>"}
		response := FormatDevTitleResponse(mentions, roleID)
		expected := fmt.Sprintf("Found %d users with the %s role: %s", len(mentions), roleMention, strings.Join(mentions, ", "))
		assert.Equal(t, expected, response)
	})

	t.Run("handles nil mentions", func(t *testing.T) {
		response := FormatDevTitleResponse(nil, roleID)
		expected := fmt.Sprintf("Found 0 users with the %s role", roleMention)
		assert.Equal(t, expected, response)
	})

	t.Run("handles empty role ID", func(t *testing.T) {
		mentions := []string{"<@123>"}
		emptyRoleMention := "<@&>"
		response := FormatDevTitleResponse([]string{"<@123>"}, "")
		expected := fmt.Sprintf("Found 1 user with the %s role: %s", emptyRoleMention, mentions[0])
		assert.Equal(t, expected, response)
	})
}

// ... MockDiscordSession ...
// ... TestGetUsersWithRole ...
// ... TestFormatUserMentions ...
// ... TestFormatRoleMention ...
// ... TestFormatMentionResponse ...
// ... TestFormatDevTitleResponse ...

// TestHasRole tests the HasRole helper function.
func TestHasRole(t *testing.T) {
	roleID := "testRole123"
	otherRole := "otherRole456"
	whitespaceRole := "   " // Role consisting of only whitespace
	emptyRole := ""         // Empty string role

	// Test cases using assert
	t.Run("member is nil", func(t *testing.T) {
		var memberNil *discordgo.Member = nil
		assert.False(t, HasRole(memberNil, roleID))
	})

	t.Run("member roles slice is nil", func(t *testing.T) {
		memberNilRoles := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: nil}
		assert.False(t, HasRole(memberNilRoles, roleID))
	})

	t.Run("member roles slice is empty", func(t *testing.T) {
		memberEmptyRoles := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{}}
		assert.False(t, HasRole(memberEmptyRoles, roleID))
	})

	t.Run("member has the target role", func(t *testing.T) {
		memberWithRole := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{otherRole, roleID}}
		assert.True(t, HasRole(memberWithRole, roleID))
	})

	t.Run("member does not have the target role", func(t *testing.T) {
		memberWithoutRole := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{otherRole, "anotherRole"}}
		assert.False(t, HasRole(memberWithoutRole, roleID))
	})

	// --- Tests for Whitespace Handling (NEW) ---

	t.Run("member has whitespace role, searching for normal role", func(t *testing.T) {
		memberWithWhitespace := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{roleID, whitespaceRole}}
		// Should ignore the whitespaceRole and find the target roleID
		assert.True(t, HasRole(memberWithWhitespace, roleID))
		// Should ignore the whitespaceRole and NOT find otherRole
		assert.False(t, HasRole(memberWithWhitespace, otherRole))
	})

	t.Run("member has only whitespace role, searching for normal role", func(t *testing.T) {
		memberWithOnlyWhitespace := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{whitespaceRole}}
		// Should ignore the whitespaceRole and not find the target roleID
		assert.False(t, HasRole(memberWithOnlyWhitespace, roleID))
	})

	t.Run("member has only empty string role, searching for normal role", func(t *testing.T) {
		memberWithOnlyEmpty := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{emptyRole}}
		// Should ignore the emptyRole and not find the target roleID
		assert.False(t, HasRole(memberWithOnlyEmpty, roleID))
	})

	t.Run("member has whitespace role, searching specifically for whitespace role", func(t *testing.T) {
		memberWithWhitespace := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{roleID, whitespaceRole}}
		// Should perform exact match and FIND the whitespaceRole when it's the target
		assert.True(t, HasRole(memberWithWhitespace, whitespaceRole))
	})

	t.Run("member has empty string role, searching specifically for empty string role", func(t *testing.T) {
		memberWithEmpty := &discordgo.Member{User: &discordgo.User{ID: "u1"}, Roles: []string{roleID, emptyRole}}
		// Should perform exact match and FIND the emptyRole when it's the target
		assert.True(t, HasRole(memberWithEmpty, emptyRole))
	})

	// --- End Whitespace Handling Tests ---
}
