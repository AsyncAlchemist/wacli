package app

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"

	"github.com/openclaw/wacli/internal/wa"
)

// MarkMessagesRead sends WhatsApp read receipts for the given message IDs in
// chat. This is a receipt to the other party, unlike MarkChatRead, which is an
// app-state patch that only syncs the chat's read flag to this account's own
// devices.
//
// The returned receipt type is what WhatsApp actually sent: types.ReceiptTypeRead
// notifies the sender (blue ticks); types.ReceiptTypeReadSelf is what whatsmeow
// substitutes when this account's read-receipts privacy setting is off (or the
// chat is a newsletter), and it only syncs to this account's own devices.
//
// Receipts apply to received messages: any ID that the local store knows as one
// of this account's own outgoing messages is rejected, on every path. IDs that
// are not in the store are accepted in direct chats and, with an explicit
// sender, in groups, so an unsynced received message can still be marked read.
//
// In a group chat the receipt is addressed to the participant who sent the
// messages. When sender is empty, it is looked up from the local store; every
// ID must then be stored and belong to the same participant.
func (a *App) MarkMessagesRead(ctx context.Context, chat types.JID, ids []string, sender types.JID) (types.ReceiptType, error) {
	msgIDs := make([]types.MessageID, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		msgIDs = append(msgIDs, types.MessageID(id))
	}
	if len(msgIDs) == 0 {
		return "", fmt.Errorf("at least one message id is required")
	}
	resolveSender := chat.Server == types.GroupServer && sender.IsEmpty()
	var stored types.JID
	for _, id := range msgIDs {
		msg, err := a.db.GetMessage(chat.String(), string(id))
		if err != nil {
			if resolveSender {
				return "", fmt.Errorf("message %s is not in the local store; pass --sender for group messages", id)
			}
			continue
		}
		if msg.FromMe {
			return "", fmt.Errorf("message %s was sent by you; read receipts apply to received messages", id)
		}
		if !resolveSender {
			continue
		}
		jid, err := wa.ParseUserOrJID(msg.SenderJID)
		if err != nil {
			return "", fmt.Errorf("message %s has no usable sender in the local store; pass --sender", id)
		}
		if !stored.IsEmpty() && stored.String() != jid.String() {
			return "", fmt.Errorf("messages %s and %s have different senders; mark them read separately", msgIDs[0], id)
		}
		stored = jid
	}
	if resolveSender {
		sender = stored
	}
	return a.wa.MarkRead(ctx, chat, sender, msgIDs)
}
