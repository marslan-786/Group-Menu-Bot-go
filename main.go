package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

const (
	BOT_TAG  = "IMPOSSIBLE_STABLE_V1"
	DEV_NAME = "Nothing Is Impossible"
)

var (
	client    *whatsmeow.Client
	container *sqlstore.Container
	startTime = time.Now()
)

func main() {
	fmt.Println("🚀 IMPOSSIBLE BOT | START")

	// ------------------- DB SETUP -------------------
	dbURL := os.Getenv("DATABASE_URL")
	dbType := "postgres"
	if dbURL == "" {
		dbType = "sqlite3"
		dbURL = "file:impossible.db?_foreign_keys=on"
	}

	var err error
	container, err = sqlstore.New(
		context.Background(),
		dbType,
		dbURL,
		waLog.Stdout("DB", "INFO", true),
	)
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}

	// ------------------- DEVICE SETUP -------------------
	var device *store.Device
	devices, _ := container.GetAllDevices(context.Background())
	
	// Get the most recent device (last paired)
	if len(devices) > 0 {
		device = devices[len(devices)-1]
		fmt.Printf("📱 Found existing device: %s\n", device.PushName)
	}
	
	if device == nil {
		device = container.NewDevice()
		device.PushName = BOT_TAG
		fmt.Println("🆕 New session created")
	}

	client = whatsmeow.NewClient(device, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(eventHandler)

	// Auto-connect if session exists
	if client.Store.ID != nil {
		fmt.Println("🔄 Restoring previous session...")
		err = client.Connect()
		if err != nil {
			log.Printf("⚠️ Connection error: %v", err)
			fmt.Println("💡 Use website to pair again")
		} else {
			fmt.Println("✅ Session restored and connected!")
		}
	} else {
		fmt.Println("⏳ No active session - Use website to pair")
	}

	// ------------------- WEB SERVER -------------------
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	
	// Check if web folder exists, if not use default handling
	if _, err := os.Stat("web"); !os.IsNotExist(err) {
		r.LoadHTMLGlob("web/*.html")
		r.Static("/pic.png", "./web/pic.png")
	}
	
	// Try serving from root if not in web folder
	if _, err := os.Stat("pic.png"); !os.IsNotExist(err) {
		r.StaticFile("/pic.png", "./pic.png")
	}

	// Home page
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"paired": client.Store.ID != nil,
		})
	})

	// API to get pairing code
	r.POST("/api/pair", handlePairAPI)

	go r.Run(":8080")
	fmt.Println("🌐 Web server running on port 8080")

	// ------------------- GRACEFUL SHUTDOWN -------------------
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	client.Disconnect()
}

// ================= EVENTS =================

func eventHandler(evt interface{}) {
	// FIX: Use v directly or ignore if not needed, handled type switch correctly
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe {
			return
		}

		text := strings.ToLower(strings.TrimSpace(getText(v.Message)))
		
		fmt.Printf("📩 Msg: %s | From: %s\n", text, v.Info.Sender.User)

		// Handle text commands
		switch text {
		case "#menu", "menu":
			sendMenu(v.Info.Chat)
		case "#ping", "ping":
			sendPing(v.Info.Chat)
		case "#info", "info":
			sendInfo(v.Info.Chat)
		}
	
	case *events.Connected:
		fmt.Println("🟢 BOT CONNECTED")
	case *events.Disconnected:
		fmt.Println("🔴 BOT DISCONNECTED")
	}
}

func getText(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Conversation != nil {
		return *msg.Conversation
	}
	if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
		return *msg.ExtendedTextMessage.Text
	}
	return ""
}

// ================= MENU =================

func sendMenu(chat types.JID) {
	menuText := `╔════════════════════╗
║  🚀 IMPOSSIBLE BOT
║  📋 MAIN MENU
╠════════════════════╣
║ ⚡ *#ping*
║ ℹ️ *#info*
║ 📋 *#menu*
╚════════════════════╝`

	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(menuText),
	})
}

// ================= PING =================

func sendPing(chat types.JID) {
	start := time.Now()
	// Fake latency for effect
	time.Sleep(50 * time.Millisecond) 
	ms := time.Since(start).Milliseconds()
	uptime := time.Since(startTime).Round(time.Second)

	msg := fmt.Sprintf("⚡ PONG: %dms\n⏱ Uptime: %s", ms, uptime)

	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(msg),
	})
}

// ================= INFO =================

func sendInfo(chat types.JID) {
	uptime := time.Since(startTime).Round(time.Second)
	msg := fmt.Sprintf("🤖 IMPOSSIBLE BOT v4\n👨‍💻 Dev: %s\n⏱ Uptime: %s", DEV_NAME, uptime)

	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(msg),
	})
}

// ================= PAIR API (YOUR LOGIC) =================

func handlePairAPI(c *gin.Context) {
	var req struct {
		Number string `json:"number"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	number := strings.ReplaceAll(req.Number, "+", "")
	number = strings.TrimSpace(number)

	// Create NEW device for this pairing
	newDevice := container.NewDevice()
	newDevice.PushName = BOT_TAG
	
	// Create temporary client for pairing
	tempClient := whatsmeow.NewClient(newDevice, waLog.Stdout("Pairing", "INFO", true))
	
	fmt.Println("🔌 Connecting for pairing...")
	err := tempClient.Connect()
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		c.JSON(500, gin.H{"error": "Failed to connect: " + err.Error()})
		return
	}

	// Wait for stable connection (YOUR LOGIC)
	fmt.Println("⏳ Waiting 5s for socket stability...")
	time.Sleep(5 * time.Second)

	fmt.Printf("📱 Generating pairing code for %s...\n", number)
	code, err := tempClient.PairPhone(
		context.Background(),
		number,
		true,
		whatsmeow.PairClientChrome,
		"Linux",
	)
	
	if err != nil {
		fmt.Printf("❌ Pairing failed: %v\n", err)
		tempClient.Disconnect()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("✅ Code generated: %s\n", code)
	
	// Keep temp client connected until paired
	go func() {
		// Give user 60 seconds to pair
		for i := 0; i < 60; i++ {
			if tempClient.Store.ID != nil {
				fmt.Println("✅ Pairing successful!")
				
				// Disconnect old client
				if client != nil {
					client.Disconnect()
				}
				
				// Swap clients
				client = tempClient
				client.AddEventHandler(eventHandler)
				return
			}
			time.Sleep(1 * time.Second)
		}
		fmt.Println("❌ Pairing timed out")
		tempClient.Disconnect()
	}()
	
	c.JSON(200, gin.H{"code": code})
}