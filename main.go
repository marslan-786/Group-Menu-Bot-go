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

func main() {
	fmt.Println("🚀 [System] Starting Engine with Full Debug Logging...")

	dbURL := os.Getenv("DATABASE_URL")
	dbType := "postgres"
	if dbURL == "" {
		fmt.Println("⚠️ [DB] DATABASE_URL missing, using local SQLite.")
		dbURL = "file:impossible_session.db?_foreign_keys=on"
		dbType = "sqlite3"
	}

	dbLog := waLog.Stdout("Database", "DEBUG", true) // ڈیبگ موڈ آن
	container, err := sqlstore.New(context.Background(), dbType, dbURL, dbLog)
	if err != nil {
		fmt.Printf("❌ [DB ERROR] %v\n", err)
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "DEBUG", true)) // کلائنٹ ڈیبگ آن
	client.AddEventHandler(eventHandler)

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.StaticFile("/", "./web/index.html")
	r.StaticFile("/pic.png", "./web/pic.png")

	// پیرنگ لاجک بمعہ تفصیلی لاگز
	r.POST("/api/pair", func(c *gin.Context) {
		var req struct{ Number string `json:"number"` }
		c.BindJSON(&req)
		
		fmt.Printf("📲 [Request] Pairing request for number: %s\n", req.Number)

		if !client.IsConnected() {
			fmt.Println("🌐 [Network] Connecting to WhatsApp...")
			err := client.Connect()
			if err != nil {
				fmt.Printf("❌ [Network Error] Connection failed: %v\n", err)
				c.JSON(500, gin.H{"error": "WhatsApp link failure"})
				return
			}
			time.Sleep(7 * time.Second) // واٹس ایپ کو مستحکم ہونے کے لیے زیادہ وقت دیں
		}

		fmt.Println("🔑 [Auth] Requesting Pairing Code from Server...")
		code, err := client.PairPhone(context.Background(), req.Number, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		
		if err != nil {
			fmt.Printf("❌ [Pairing Error Detail] %v\n", err) // اصل ایرر یہاں پرنٹ ہوگا
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed: %v", err)})
			return
		}

		fmt.Printf("✅ [Success] Generated Code: %s\n", code)
		c.JSON(200, gin.H{"code": code})
	})

	go func() {
		fmt.Printf("🌐 [Web] Dashboard: http://0.0.0.0:%s\n", port)
		r.Run(":" + port)
	}()

	if client.Store.ID != nil {
		fmt.Println("🔄 [Session] Restoring existing login...")
		client.Connect()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	client.Disconnect()
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		body := v.Message.GetConversation()
		if body == "" { body = v.Message.GetExtendedTextMessage().GetText() }
		if strings.TrimSpace(body) == "#menu" {
			sendOfficialMenu(v.Info.Chat)
		}
	}
}

func sendOfficialMenu(chat types.JID) {
	listMsg := &waProto.ListMessage{
		Title:       proto.String("IMPOSSIBLE MENU"),
		Description: proto.String("Select category"),
		ButtonText:  proto.String("MENU"),
		ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
		Sections: []*waProto.ListMessage_Section{
			{
				Title: proto.String("TOOLS"),
				Rows: []*waProto.ListMessage_Row{
					{Title: proto.String("Ping"), RowID: proto.String("ping")},
				},
			},
		},
	}
	client.SendMessage(context.Background(), chat, &waProto.Message{ListMessage: listMsg})
}