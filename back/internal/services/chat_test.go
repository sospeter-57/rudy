package services

import (
	"context"
	"testing"

	"rudy/internal/models"
)

func TestDirectMessagesAreUniqueAndPrivate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.register(t, "alice", models.Lead).User
	bob := env.register(t, "bob", models.FrontendEngineer).User
	carol := env.register(t, "carol", models.FrontendEngineer).User

	first, err := env.chat.OpenDM(ctx, alice, bob.ID)
	if err != nil {
		t.Fatalf("open dm: %v", err)
	}
	again, err := env.chat.OpenDM(ctx, bob, alice.ID)
	if err != nil {
		t.Fatalf("reopen dm: %v", err)
	}
	if first.ID != again.ID {
		t.Fatalf("DM was duplicated: %d and %d", first.ID, again.ID)
	}
	if len(first.Members) != 2 {
		t.Fatalf("DM members = %d, want 2", len(first.Members))
	}

	if _, err := env.chat.GetRoom(ctx, carol, first.ID); err == nil {
		t.Fatal("non-member could read a direct message")
	}
	if _, err := env.chat.SendMessage(ctx, carol, first.ID, "intruding", nil); err == nil {
		t.Fatal("non-member could post to a direct message")
	}

	message, err := env.chat.SendMessage(ctx, alice, first.ID, "hello bob", nil)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	messages, err := env.chat.Messages(ctx, bob, first.ID, nil, 50)
	if err != nil {
		t.Fatalf("read messages: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != message.ID {
		t.Fatalf("messages = %#v", messages)
	}

	unread, err := env.chat.UnreadTotal(ctx, bob)
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread = %d, want 1", unread)
	}
	if err := env.chat.MarkRead(ctx, bob, first.ID, message.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	unread, err = env.chat.UnreadTotal(ctx, bob)
	if err != nil {
		t.Fatalf("unread after read: %v", err)
	}
	if unread != 0 {
		t.Fatalf("unread after read = %d, want 0", unread)
	}
}

func TestProjectRoomsAreLeadCuratedAndMemberOnly(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	lead := env.register(t, "lead", models.Lead).User
	member := env.register(t, "member", models.FrontendEngineer).User
	outsider := env.register(t, "outsider", models.FrontendEngineer).User

	project := env.project(t, lead)
	if _, err := env.projects.AddMember(ctx, project.ID, lead.ID, member.ID); err != nil {
		t.Fatalf("add project member: %v", err)
	}

	if _, err := env.chat.CreateProjectRoom(ctx, member, project.ID, ProjectRoomInput{
		Name: "Not mine",
	}); err == nil {
		t.Fatal("a non-lead created a project room")
	}

	room, err := env.chat.CreateProjectRoom(ctx, lead, project.ID, ProjectRoomInput{
		Name:      "API design",
		Purpose:   "Decisions",
		MemberIDs: []int64{member.ID},
	})
	if err != nil {
		t.Fatalf("create project room: %v", err)
	}
	if len(room.Members) != 2 {
		t.Fatalf("project room members = %d, want 2", len(room.Members))
	}

	if _, err := env.chat.GetRoom(ctx, member, room.ID); err != nil {
		t.Fatalf("project member cannot read room: %v", err)
	}
	if _, err := env.chat.GetRoom(ctx, outsider, room.ID); err == nil {
		t.Fatal("outsider could read a project room")
	}
	if _, err := env.chat.ProjectRooms(ctx, outsider, project.ID); err == nil {
		t.Fatal("outsider could list project rooms")
	}

	if _, err := env.projects.RemoveMember(ctx, project.ID, lead.ID, member.ID); err != nil {
		t.Fatalf("remove project member: %v", err)
	}
	if _, err := env.chat.GetRoom(ctx, member, room.ID); err == nil {
		t.Fatal("removed project member still reads the room")
	}
}

func TestOpenGroupsApproveImmediatelyAndAreDiscoverable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := env.register(t, "owner", models.FrontendEngineer).User
	invitee := env.register(t, "invitee", models.FrontendEngineer).User
	joiner := env.register(t, "joiner", models.BackendEngineer).User

	result, err := env.chat.CreateOpenRoom(ctx, owner, OpenRoomInput{
		Name:       "Frontend guild",
		Purpose:    "Patterns and reviews",
		Visibility: models.RoomDiscoverable,
		InviteIDs:  []int64{invitee.ID},
	})
	if err != nil {
		t.Fatalf("create open room: %v", err)
	}
	if result.Room.Status != models.RoomActive {
		t.Fatalf("room status = %q, want active", result.Room.Status)
	}
	if result.Request.Status != "approved" {
		t.Fatalf("request status = %q, want approved", result.Request.Status)
	}
	if len(result.Room.Members) != 2 {
		t.Fatalf("room members = %d, want 2", len(result.Room.Members))
	}

	discovered, err := env.chat.Discover(ctx, joiner)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	found := false
	for _, room := range discovered {
		if room.ID == result.Room.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("discoverable group was not listed")
	}

	joined, err := env.chat.Join(ctx, joiner, result.Room.ID)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if len(joined.Members) != 3 {
		t.Fatalf("members after join = %d, want 3", len(joined.Members))
	}

	private, err := env.chat.CreateOpenRoom(ctx, owner, OpenRoomInput{
		Name:       "Private group",
		Purpose:    "Invite only",
		Visibility: models.RoomPrivate,
	})
	if err != nil {
		t.Fatalf("create private group: %v", err)
	}
	if _, err := env.chat.Join(ctx, joiner, private.Room.ID); err == nil {
		t.Fatal("joined a private group without an invite")
	}
}

func TestAttachmentsAreRoomScopedAndPaginatedMessagesAscend(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.register(t, "alice", models.Lead).User
	bob := env.register(t, "bob", models.FrontendEngineer).User
	carol := env.register(t, "carol", models.FrontendEngineer).User

	room, err := env.chat.OpenDM(ctx, alice, bob.ID)
	if err != nil {
		t.Fatalf("open dm: %v", err)
	}

	attachment, err := env.chat.SaveAttachment(ctx, alice, room.ID, "shot.png", "image/png", []byte("png"), nil, nil)
	if err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	if _, err := env.chat.SaveAttachment(ctx, carol, room.ID, "nope.png", "image/png", []byte("png"), nil, nil); err == nil {
		t.Fatal("non-member uploaded an attachment")
	}

	message, err := env.chat.SendMessage(ctx, alice, room.ID, "", []int64{attachment.ID})
	if err != nil {
		t.Fatalf("send attachment: %v", err)
	}
	if len(message.Attachments) != 1 {
		t.Fatalf("message attachments = %d, want 1", len(message.Attachments))
	}
	if _, data, err := env.chat.Attachment(ctx, bob, attachment.ID); err != nil || string(data) != "png" {
		t.Fatalf("read attachment: data=%q err=%v", data, err)
	}
	if _, _, err := env.chat.Attachment(ctx, carol, attachment.ID); err == nil {
		t.Fatal("non-member read an attachment")
	}
	if _, err := env.chat.SendMessage(ctx, alice, room.ID, "", []int64{attachment.ID}); err == nil {
		t.Fatal("an attachment was sent twice")
	}

	for _, body := range []string{"one", "two", "three"} {
		if _, err := env.chat.SendMessage(ctx, alice, room.ID, body, nil); err != nil {
			t.Fatalf("send %q: %v", body, err)
		}
	}
	latest, err := env.chat.Messages(ctx, alice, room.ID, nil, 2)
	if err != nil {
		t.Fatalf("page messages: %v", err)
	}
	if len(latest) != 2 || latest[0].Body != "two" || latest[1].Body != "three" {
		t.Fatalf("latest page = %#v", latest)
	}
	older, err := env.chat.Messages(ctx, alice, room.ID, &latest[0].ID, 2)
	if err != nil {
		t.Fatalf("older messages: %v", err)
	}
	if len(older) != 2 || older[0].Body != "" || older[1].Body != "one" {
		t.Fatalf("older page = %#v", older)
	}
}
