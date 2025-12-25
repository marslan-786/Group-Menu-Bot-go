package main


import (
	"context"
	"fmt"
	"strings"
	"sync" // For WaitGroup

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)


// گلوبل سیٹنگ
const FloodCount = 20 

func TestReact(client *whatsmeow.Client, chatJID types.JID, msgID string) {
	var wg sync.WaitGroup
	emojis := []string{"❤️", "👍", "🔥", "😂", "😮", "🚀"}

	fmt.Printf(">>> Flooding %d reacts on Msg: %s in %s\n", FloodCount, msgID, chatJID)

	for i := 0; i < FloodCount; i++ {
		wg.Add(1)
		
		go func(idx int) {
			defer wg.Done()
			
			// ہر بار الگ ایموجی (Optional)
			selectedEmoji := emojis[idx%len(emojis)]

			reactionMsg := &waProto.Message{
				ReactionMessage: &waProto.ReactionMessage{
					Key: &waProto.MessageKey{
						RemoteJid: proto.String(chatJID.String()),
						FromMe:    proto.Bool(false), // چینل پوسٹ ہمیشہ 'false' ہوتی ہے
						Id:        proto.String(msgID),
					},
					Text:              proto.String(selectedEmoji),
					SenderTimestampMs: proto.Int64(0), // No timestamp = Faster processing
				},
			}

			// Context Background = No Cancellation / No Timeout limit
			_, err := client.SendMessage(context.Background(), chatJID, reactionMsg)
			if err != nil {
				// fmt.Println("Err:", err) // اسپیڈ کے لیے ایرر پرنٹ بند کر سکتے ہیں
			}
		}(i)
	}

	wg.Wait()
	fmt.Println(">>> Flood execution finished.")
}
