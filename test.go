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

const FloodCount = 50
const TargetEmoji = "❤️" 

func GetMessageContent(msg *waProto.Message) string {
	if msg == nil { return "" }
	if msg.Conversation != nil { return *msg.Conversation }
	if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil { return *msg.ExtendedTextMessage.Text }
	if msg.ImageMessage != nil && msg.ImageMessage.Caption != nil { return *msg.ImageMessage.Caption }
	return ""
}

func replyToUser(client *whatsmeow.Client, chatJID types.JID, text string) {
	msg := &waProto.Message{Conversation: proto.String(text)}
	client.SendMessage(context.Background(), chatJID, msg)
}

func StartFloodAttack(client *whatsmeow.Client, v *events.Message) {
	userChat := v.Info.Chat
	fullText := GetMessageContent(v.Message)
	args := strings.Fields(fullText)

	if len(args) < 2 {
		replyToUser(client, userChat, "❌ لنک مہیا کریں۔")
		return
	}

	link := args[1]
	parts := strings.Split(link, "/")
	
	if len(parts) < 2 {
		replyToUser(client, userChat, "❌ غلط لنک۔")
		return
	}

	lastPart := parts[len(parts)-1]
	msgID := strings.Split(lastPart, "?")[0]
	inviteCode := parts[len(parts)-2]

	fmt.Printf("Debug: Invite=%s, MsgID=%s\n", inviteCode, msgID)
	replyToUser(client, userChat, "🔍 چینل ڈیٹا اٹھا رہا ہوں...")

	// 1. چینل کی معلومات
	metadata, err := client.GetNewsletterInfoWithInvite(context.Background(), inviteCode)
	if err != nil {
		replyToUser(client, userChat, fmt.Sprintf("❌ چینل نہیں ملا: %v", err))
		return
	}

	targetJID := metadata.ID
	
	// --- FIX IS HERE: metadata.Name.Text instead of ThreadMetadata ---
	channelName := "Unknown"
	if metadata.Name != nil {
		channelName = metadata.Name.Text
	}

	replyToUser(client, userChat, fmt.Sprintf("✅ چینل: %s\nID: %s\n ٹیسٹ شاٹ لے رہا ہوں...", channelName, msgID))

	// 2. TEST SHOT
	testReaction := &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(targetJID.String()),
				FromMe:    proto.Bool(false), 
				ID:        proto.String(msgID),
			},
			Text:              proto.String(TargetEmoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()), 
		},
	}

	resp, err := client.SendMessage(context.Background(), targetJID, testReaction)
	if err != nil {
		fmt.Println("Reaction Error:", err)
		replyToUser(client, userChat, fmt.Sprintf("❌ ری ایکٹ فیل ہوگیا!\nوجہ: %v", err))
		return
	}

	fmt.Println("Test Shot Success. Server ID:", resp.ID)
	replyToUser(client, userChat, "✅ ٹیسٹ کامیاب! اب فلڈ کر رہا ہوں... 🚀")

	// 3. FLOOD
	performFlood(client, targetJID, msgID)
	
	replyToUser(client, userChat, "✅ مشن مکمل۔")
}

func performFlood(client *whatsmeow.Client, chatJID types.JID, msgID string) {
	var wg sync.WaitGroup
	fmt.Printf(">>> Stacking %s on Msg: %s\n", TargetEmoji, msgID)

	for i := 0; i < FloodCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reactionMsg := &waProto.Message{
				ReactionMessage: &waProto.ReactionMessage{
					Key: &waProto.MessageKey{
						RemoteJID: proto.String(chatJID.String()),
						FromMe:    proto.Bool(false),
						ID:        proto.String(msgID),
					},
					Text:              proto.String(TargetEmoji),
					SenderTimestampMS: proto.Int64(time.Now().UnixMilli()), 
				},
			}
			_, err := client.SendMessage(context.Background(), chatJID, reactionMsg)
			if err != nil {
				fmt.Printf("Flood Err %d: %v\n", idx, err)
			}
		}(i)
	}
	wg.Wait()
}