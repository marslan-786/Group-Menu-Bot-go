package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

var client *whatsmeow.Client
var container *sqlstore.Container

func main() {
	fmt.Println("🚀 [System] Impossible Bot starting with VERBOSE LOGGING...")

	dbURL := os.Getenv("DATABASE_URL")
	dbType := "postgres"
	if dbURL == "" {
		dbURL = "file:impossible_session.db?_foreign_keys=on"
		dbType = "sqlite3"
	}

	dbLog := waLog.Stdout("Database", "INFO", true)
	var err error
	container, err = sqlstore.New(context.Background(), dbType, dbURL, dbLog)
	if err != nil { panic(err) }

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil { panic(err) }

	client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(eventHandler)

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.StaticFile("/", "./web/index.html")
	r.StaticFile("/pic.png", "./web/pic.png")

	r.POST("/api/pair", handlePairAPI)

	go func() {
		fmt.Printf("🌐 [Web] Dashboard running at: http://0.0.0.0:%s\n", port)
		r.Run(":" + port)
	}()

	if client.Store.ID != nil {
		fmt.Println("🔄 [Auth] Reconnecting existing session...")
		client.Connect()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	client.Disconnect()
}

// میسج سے ٹیکسٹ نکالنے کا تفصیلی طریقہ (Log Printing کے ساتھ)
func getDetailedText(msg *waProto.Message) string {
	if msg == nil { return "" }
	
	// ہر ٹائپ کے لیے لاگ پرنٹ کریں تاکہ پتہ چلے واٹس ایپ کیا بھیج رہا ہے
	if msg.Conversation != nil { 
		fmt.Println("🔍 [Parser] Type: Simple Conversation")
		return msg.GetConversation() 
	}
	if msg.ExtendedTextMessage != nil { 
		fmt.Println("🔍 [Parser] Type: Extended Text (Reply/Link)")
		return msg.ExtendedTextMessage.GetText() 
	}
	if msg.ImageMessage != nil { 
		fmt.Println("🔍 [Parser] Type: Image Caption")
		return msg.ImageMessage.GetCaption() 
	}
	if msg.VideoMessage != nil { 
		fmt.Println("🔍 [Parser] Type: Video Caption")
		return msg.VideoMessage.GetCaption() 
	}
	if msg.ViewOnceMessageV2 != nil { 
		fmt.Println("🔍 [Parser] Type: ViewOnce Message")
		return getDetailedText(msg.ViewOnceMessageV2.Message) 
	}
	
	fmt.Printf("🔍 [Parser] Unknown or unsupported message type: %T\n", msg)
	return ""
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe { return }

		// تفصیلی لاگنگ: میسج کہاں سے آیا اور کیا ہے
		sender := v.Info.Sender.User
		body := strings.TrimSpace(getDetailedText(v.Message))
		
		fmt.Printf("📩 [New Message] From: %s | Text: '%s' | ChatID: %s\n", sender, body, v.Info.Chat)

		if body == "#menu" {
			fmt.Printf("⚙️ [Action] Menu command detected! Sending response to %s\n", v.Info.Chat)
			
			// ری ایکشن بھیجیں
			err := client.SendMessage(context.Background(), v.Info.Chat, client.BuildReaction(v.Info.Chat, v.Info.Sender, v.Info.ID, "📜"))
			if err != nil { fmt.Printf("⚠️ [Log] Reaction failed: %v\n", err) }

			sendMenuWithImage(v.Info.Chat)
		}
	}
}

// مینیو بھیجنا (تصویر + بٹن + ٹیکسٹ)
func sendMenuWithImage(chat types.JID) {
	fmt.Println("🖼️ [Media] Reading pic.png for menu...")
	imgData, err := os.ReadFile("./web/pic.png")
	if err != nil {
		fmt.Printf("❌ [File Error] Could not read pic.png: %v\n", err)
		// اگر تصویر نہ ملے تو صرف ٹیکسٹ مینیو بھیجیں
		client.SendMessage(context.Background(), chat, &waProto.Message{Conversation: proto.String("*IMPOSSIBLE MENU*\n\n(Image missing in web/ folder)")})
		return
	}

	// بٹن سٹرکچر (List Message)
	listMsg := &waProto.ListMessage{
		Title:       proto.String("IMPOSSIBLE BOT"),
		Description: proto.String("Advanced Go Engine\nSelect a command below:"),
		ButtonText:  proto.String("MENU"),
		ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
		Sections: []*waProto.ListMessage_Section{
			{
				Title: proto.String("COMMANDS"),
				Rows: []*waProto.ListMessage_Row{
					{Title: proto.String("Ping Status"), RowID: proto.String("ping"), Description: proto.String("Check Bot Speed")},
					{Title: proto.String("User Info"), RowID: proto.String("id")},
				},
			},
		},
	}

	// تصویر والا میسج جس کے کیپشن میں مینیو ہوگا
	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Mimetype:      proto.String("image/png"),
			Caption:       proto.String("*📜 IMPOSSIBLE MENU*\n\nHi! I am alive. If buttons don't show below, it's a WhatsApp restriction on your account."),
			ContentLength: proto.Uint64(uint64(len(imgData))),
		},
		ListMessage: listMsg, // بٹن بھی ساتھ اٹیچ کر دیے
	}

	// تصویر کا ڈیٹا بھی ساتھ بھیجنا ضروری ہے
	fmt.Println("📤 [Network] Sending Menu Bundle to WhatsApp...")
	resp, sendErr := client.SendMessage(context.Background(), chat, msg)
	
	if sendErr != nil {
		fmt.Printf("❌ [Send Error] Menu failed to deliver: %v\n", sendErr)
	} else {
		fmt.Printf("✅ [Delivery] Menu sent successfully! MessageID: %s\n", resp.ID)
	}
}

// پیرنگ API لاجک
func handlePairAPI(c *gin.Context) {
	var req struct{ Number string `json:"number"` }
	c.BindJSON(&req)
	cleanNum := strings.ReplaceAll(req.Number, "+", "")
	
	fmt.Printf("🧹 [Security] Fresh pairing request for: %s\n", cleanNum)

	devices, _ := container.GetAllDevices(context.Background())
	for _, dev := range devices {
		if dev.ID != nil && strings.Contains(dev.ID.User, cleanNum) {
			container.DeleteDevice(context.Background(), dev)
			fmt.Printf("🗑️ [Cleanup] Deleted existing session for %s\n", cleanNum)
		}
	}

	newDevice := container.NewDevice()
	client = whatsmeow.NewClient(newDevice, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(eventHandler)
	client.Connect()
	
	time.Sleep(10 * time.Second)
	code, err := client.PairPhone(context.Background(), cleanNum, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": code})
}