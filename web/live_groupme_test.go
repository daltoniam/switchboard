package web

import (
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/groupme"
)

// GroupMe is a hosted service with no container image, so this spec reaches
// a real account through stored sandbox credentials and runs read tools only:
//
//	GROUPME_LIVE_ACCESS_TOKEN  access token from dev.groupme.com
//	GROUPME_LIVE_GROUP_ID      a group the account belongs to
//	GROUPME_LIVE_OTHER_USER_ID a user the account has a direct message chat with
func init() {
	sends := "sends a real message other people see; request shapes are covered by fixture tests in integrations/groupme"
	registerLiveSpec(liveSpec{
		Integration: "groupme",
		Tier:        liveSandbox,
		Adapter:     groupme.New(),
		Provision:   liveGroupMeProvision,
		Cases: []liveCase{
			{Tool: "groupme_get_me", Expect: []string{`"id"`, `"name"`}},
			{Tool: "groupme_list_groups", Args: map[string]any{"per_page": 5}, Expect: []string{"${group_id}"}},
			{Tool: "groupme_get_group", Args: map[string]any{"group_id": "${group_id}"}, Expect: []string{"${group_id}", "members"}},
			{Tool: "groupme_list_group_messages", Args: map[string]any{"group_id": "${group_id}", "limit": 3}, Expect: []string{`"messages"`, `"count"`}},
			{Tool: "groupme_list_chats", Args: map[string]any{"per_page": 5}, Expect: []string{"${other_user_id}"}},
			{Tool: "groupme_list_direct_messages", Args: map[string]any{"other_user_id": "${other_user_id}"}, Expect: []string{`"direct_messages"`, `"count"`}},
			{Tool: "groupme_list_bots", Expect: []string{"["}},
		},
		Skips: map[mcp.ToolName]string{
			"groupme_send_group_message":  sends,
			"groupme_send_direct_message": sends,
			"groupme_post_bot_message":    sends,
		},
	})
}

func liveGroupMeProvision(t *testing.T) liveProvision {
	token := os.Getenv("GROUPME_LIVE_ACCESS_TOKEN")
	groupID := os.Getenv("GROUPME_LIVE_GROUP_ID")
	otherUserID := os.Getenv("GROUPME_LIVE_OTHER_USER_ID")
	if token == "" || groupID == "" || otherUserID == "" {
		t.Skip("set GROUPME_LIVE_ACCESS_TOKEN, GROUPME_LIVE_GROUP_ID, and GROUPME_LIVE_OTHER_USER_ID to reach a real GroupMe account; it has no container image")
	}
	return liveProvision{
		Credentials: map[string]string{"access_token": token},
		Values:      map[string]string{"group_id": groupID, "other_user_id": otherUserID},
	}
}
