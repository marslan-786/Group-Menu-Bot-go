package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// گلوبل سیٹنگ: ایک وقت میں کتنے ری ایکٹ بھیجنے ہیں؟
const FloodCount = 20

// یہ فنکشن لنک کو بھی ہینڈل کرے گا اور اٹیک بھی کرے گا
func StartFloodAttack(client *whatsmeow.Client, v *events.Message) {
	// 1. کمانڈ اور لنک الگ کرنا
	args := strings.Fields(v.Message.GetConversation())
	if len(args) < 2 {
		fmt.Println("Please provide a WhatsApp Channel Post link.")
		return
	}

	link := args[1]
	// لنک کو توڑنا (Parsing)
	parts := strings.Split(link, "/")
	if len(parts) < 2 {
		fmt.Println("Invalid link format")
		return
	}

	// لنک سے کوڈ اور آئی ڈی نکالنا
	msgID := parts[len(parts)-1]
	inviteCode := parts[len(parts)-2]

	fmt.Printf("Resolving Channel: Code=%s, MsgID=%s\n", inviteCode, msgID)

	// 2. چینل کی معلومات حاصل کرنا (FIXED: Added context)
	metadata, err := client.GetNewsletterInfoWithInvite(context.Background(), inviteCode)
	if err != nil {
		fmt.Printf("Failed to resolve channel info: %v\n", err)
		return
	}

	// (FIXED: metadata.JID -> metadata.ID)
	targetJID := metadata.ID
	fmt.Printf("Target Resolved: %s\n", targetJID)

	// 3. فلڈ شروع کرنا (Attacking Logic)
	performFlood(client, targetJID, msgID)
}

// یہ اندرونی فنکشن ہے جو صرف لوپ چلائے گا
func performFlood(client *whatsmeow.Client, chatJID types.JID, msgID string) {
	var wg sync.WaitGroup
	emojis := []string{"❤️"}

	fmt.Printf(">>> Flooding %d reacts on Msg: %s\n", FloodCount, msgID)

	for i := 0; i < FloodCount; i++ {
		wg.Add(1)
		
		go func(idx int) {
			defer wg.Done()
			selectedEmoji := emojis[idx%len(emojis)]

			// (FIXED: Field Names Capitalization for Proto)
			reactionMsg := &waProto.Message{
				ReactionMessage: &waProto.ReactionMessage{
					Key: &waProto.MessageKey{
						RemoteJID: proto.String(chatJID.String()), // Fixed: RemoteJid -> RemoteJID
						FromMe:    proto.Bool(false),
						ID:        proto.String(msgID),            // Fixed: Id -> ID
					},
					Text:              proto.String(selectedEmoji),
					SenderTimestampMS: proto.Int64(time.Now().UnixMilli()), // Fixed: SenderTimestampMs -> SenderTimestampMS
				},
			}

			// بھیجنا
			_, err := client.SendMessage(context.Background(), chatJID, reactionMsg)
			if err != nil {
				// fmt.Println("Err:", err)
			}
		}(i)
	}

	wg.Wait()
	fmt.Println(">>> Flood execution finished.")
}
