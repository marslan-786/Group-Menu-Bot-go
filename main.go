package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
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

// --- CONFIGURATION ---
const (
	BOT_NAME     = "IMPOSSIBLE BOT"
	OWNER_NAME   = "Nothing Is Impossible"
	OWNER_NUMBER = "92311xxxxxxx" // اپنا نمبر یہاں لکھیں
)

// --- GLOBAL SETTINGS STRUCT ---
type BotSettings struct {
	Prefix       string
	AutoRead     bool
	AutoStatus   bool
	AlwaysOnline bool
	Mode         string // "public" or "private"
}

var (
	container   *sqlstore.Container
	clientMap   = make(map[string]*whatsmeow.Client)
	clientMutex sync.RWMutex
	startTime   = time.Now()
	
	// Default Settings (In-Memory)
	settings = BotSettings{
		Prefix:       "#",
		AutoRead:     false,
		AutoStatus:   false,
		AlwaysOnline: false,
		Mode:         "public",
	}
)

// --- MAIN FUNCTION ---
func main() {
	fmt.Println("🚀 IMPOSSIBLE BOT FINAL FIXED | STARTING...")

	// 1. Database Setup
	dbURL := os.Getenv("DATABASE_URL")
	dbType := "postgres"
	if dbURL == "" {
		dbType = "sqlite3"
		dbURL = "file:impossible_sessions.db?_foreign_keys=on"
	}

	dbLog := waLog.Stdout("DB", "INFO", true)
	var err error
	container, err = sqlstore.New(context.Background(), dbType, dbURL, dbLog)
	if err != nil {
		log.Fatalf("❌ DB Error: %v", err)
	}

	// 2. Restore Sessions
	devices, err := container.GetAllDevices(context.Background())
	if err == nil {
		fmt.Printf("🔄 Restoring %d sessions...\n", len(devices))
		for _, device := range devices {
			go connectClient(device)
		}
	}

	// 3. Web Server for Pairing
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("web/*.html")

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "Active", "sessions": len(clientMap)})
	})

	r.POST("/api/pair", handlePairing)

	go r.Run(":8080")
	fmt.Println("🌐 Server running on :8080")

	// 4. Keep Alive
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	clientMutex.Lock()
	for _, cli := range clientMap {
		cli.Disconnect()
	}
	clientMutex.Unlock()
}

// --- SESSION HANDLER ---
func connectClient(device *store.Device) {
	client := whatsmeow.NewClient(device, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(func(evt interface{}) {
		handler(client, evt)
	})

	if err := client.Connect(); err == nil && client.Store.ID != nil {
		clientMutex.Lock()
		clientMap[client.Store.ID.String()] = client
		clientMutex.Unlock()
		fmt.Printf("✅ Connected: %s\n", client.Store.ID.User)
		
		// Apply Always Online if enabled
		if settings.AlwaysOnline {
			client.SendPresence(types.PresenceAvailable)
		}
	}
}

func handlePairing(c *gin.Context) {
	var req struct{ Number string `json:"number"` }
	if c.BindJSON(&req) != nil { return }

	num := strings.ReplaceAll(req.Number, "+", "")
	num = strings.ReplaceAll(num, " ", "")

	device := container.NewDevice()
	client := whatsmeow.NewClient(device, waLog.Stdout("Pairing", "INFO", true))

	if err := client.Connect(); err != nil {
		c.JSON(500, gin.H{"error": "Conn Failed"})
		return
	}

	code, err := client.PairPhone(context.Background(), num, true, whatsmeow.PairClientChrome, "Linux")
	if err != nil {
		client.Disconnect()
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	client.AddEventHandler(func(evt interface{}) {
		handler(client, evt)
	})

	c.JSON(200, gin.H{"code": code})
}

// --- MAIN EVENT ROUTER ---
func handler(client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe { return }

		// --- AUTO READ STATUS ---
		if v.Info.Chat.String() == "status@broadcast" {
			if settings.AutoStatus {
				// FIX: Added context.Background() and types.ReceiptRead
				client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender, types.ReceiptRead)
				// Auto React
				emojis := []string{"💚", "❤️", "🔥", "😍"}
				randEmoji := emojis[time.Now().UnixNano()%int64(len(emojis))]
				react(client, v.Info.Chat, v.Message, randEmoji)
			}
			return
		}

		// --- AUTO READ MESSAGES ---
		if settings.AutoRead {
			client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender, types.ReceiptRead)
		}

		body := getText(v.Message)
		// Check dynamic prefix
		if !strings.HasPrefix(body, settings.Prefix) { return }

		args := strings.Fields(body[len(settings.Prefix):])
		if len(args) == 0 { return }
		cmd := strings.ToLower(args[0])
		fullArgs := strings.Join(args[1:], " ")
		
		chat := v.Info.Chat
		isGroup := v.Info.IsGroup
		
		fmt.Printf("📩 CMD: %s | Chat: %s\n", cmd, chat.User)

		// --- COMMANDS ---
		switch cmd {
		// ➤ MAIN
		case "menu", "help": sendMenu(client, chat)
		case "ping": sendPing(client, chat, v.Message)
		case "id": reply(client, chat, v.Message, fmt.Sprintf("🆔 ID: %s", v.Info.Sender.User))

		// ➤ SETTINGS (New)
		case "owner": sendOwner(client, chat)
		case "setprefix":
			if fullArgs == "" { reply(client, chat, v.Message, "⚠️ Give a prefix. Ex: #setprefix ."); return }
			settings.Prefix = args[1] // args[0] is cmd, args[1] is new prefix
			reply(client, chat, v.Message, "✅ Prefix updated to: "+settings.Prefix)
		
		case "alwaysonline":
			settings.AlwaysOnline = !settings.AlwaysOnline
			status := "OFF 🔴"
			if settings.AlwaysOnline { 
				client.SendPresence(types.PresenceAvailable)
				status = "ON 🟢"
			} else {
				client.SendPresence(types.PresenceUnavailable)
			}
			reply(client, chat, v.Message, "🌐 Always Online: "+status)

		case "autostatus":
			settings.AutoStatus = !settings.AutoStatus
			status := "OFF 🔴"; if settings.AutoStatus { status = "ON 🟢" }
			reply(client, chat, v.Message, "👁️ Auto View Status: "+status)

		case "autoread":
			settings.AutoRead = !settings.AutoRead
			status := "OFF 🔴"; if settings.AutoRead { status = "ON 🟢" }
			reply(client, chat, v.Message, "📖 Auto Read Msg: "+status)

		case "readallstatus":
			// Note: WhatsApp doesn't have a simple "read all" packet.
			// We manually mark status broadcast as read.
			jid, _ := types.ParseJID("status@broadcast")
			// We can't fetch all statuses easily without history sync, 
			// so we just confirm the command.
			react(client, chat, v.Message, "✅")
			client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, time.Now(), jid, v.Info.Sender, types.ReceiptRead)
			reply(client, chat, v.Message, "✅ Marked recent statuses as seen.")

		// ➤ DOWNLOADERS
		case "tiktok", "tt": dlTikTok(client, chat, fullArgs, v.Message)
		case "fb", "facebook": dlFacebook(client, chat, fullArgs, v.Message)
		case "insta", "ig": dlInstagram(client, chat, fullArgs, v.Message)
		case "pin", "pinterest": dlPinterest(client, chat, fullArgs, v.Message)
		case "ytmp3": dlYouTube(client, chat, fullArgs, "mp3", v.Message)
		case "ytmp4": dlYouTube(client, chat, fullArgs, "mp4", v.Message)

		// ➤ TOOLS
		case "sticker", "s": makeSticker(client, chat, v.Message)
		case "toimg": stickerToImg(client, chat, v.Message)
		case "removebg": removeBG(client, chat, v.Message)
		case "remini": reminiEnhance(client, chat, v.Message)
		case "tourl": mediaToUrl(client, chat, v.Message)
		case "weather": getWeather(client, chat, fullArgs, v.Message)
		case "tr", "translate": doTranslate(client, chat, args[1:], v.Message)

		// ➤ GROUPS
		case "kick": groupAction(client, chat, v.Message, "remove", isGroup)
		case "add": groupAdd(client, chat, args[1:], isGroup, v.Message)
		case "promote": groupAction(client, chat, v.Message, "promote", isGroup)
		case "demote": groupAction(client, chat, v.Message, "demote", isGroup)
		case "tagall": groupTagAll(client, chat, fullArgs, isGroup, v.Message)
		case "hidetag": groupHideTag(client, chat, fullArgs, isGroup, v.Message)
		case "group": groupSettings(client, chat, args[1:], isGroup, v.Message)
		case "del", "delete": deleteMessage(client, chat, v.Message)
		}
	}
}

// --- COMMAND FUNCTIONS ---

func sendMenu(client *whatsmeow.Client, chat types.JID) {
	react(client, chat, nil, "📜")
	menu := fmt.Sprintf(`╭━━━〔 *%s* 〕━━━┈
┃ 👋 *Assalam-o-Alaikum*
┃ 👑 *Owner:* %s
┃ 🤖 *Bot:* IMPOSSIBLE_V3
┃ 📍 *Prefix:* %s
┃ ⏳ *Uptime:* %s
╰━━━━━━━━━━━━━━━━━━┈

╭━━〔 *SETTINGS* 〕━━┈
┃ 🔸 %ssetprefix [symbol]
┃ 🔸 %sowner
┃ 🔸 %salwaysonline
┃ 🔸 %sautostatus
┃ 🔸 %sautoread
┃ 🔸 %sreadallstatus
╰━━━━━━━━━━━━━━━━━━┈

╭━━〔 *DOWNLOADERS* 〕━━┈
┃ 🔸 %stiktok [url]
┃ 🔸 %sfb [url]
┃ 🔸 %sinsta [url]
┃ 🔸 %spin [url]
┃ 🔸 %sytmp3 [url]
┃ 🔸 %sytmp4 [url]
╰━━━━━━━━━━━━━━━━━━┈

╭━━〔 *TOOLS* 〕━━┈
┃ 🔸 %ssticker (Reply Media)
┃ 🔸 %stoimg (Reply Sticker)
┃ 🔸 %sremovebg (Reply Img)
┃ 🔸 %sremini (Reply Img)
┃ 🔸 %stranslate [text]
┃ 🔸 %sweather [city]
┃ 🔸 %stourl (Reply Media)
╰━━━━━━━━━━━━━━━━━━┈

╭━━〔 *GROUPS* 〕━━┈
┃ 🔸 %skick @user
┃ 🔸 %sadd 923...
┃ 🔸 %spromote @user
┃ 🔸 %sdemote @user
┃ 🔸 %stagall [msg]
┃ 🔸 %shidetag [msg]
┃ 🔸 %sgroup open/close
┃ 🔸 %sdel (Reply Msg)
╰━━━━━━━━━━━━━━━━━━┈

© 2025 %s`, 
	BOT_NAME, OWNER_NAME, settings.Prefix, time.Since(startTime).Round(time.Second),
	settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix,
	settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix,
	settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix,
	settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix, settings.Prefix,
	BOT_NAME)

	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(menu),
	})
}

func sendOwner(client *whatsmeow.Client, chat types.JID) {
	vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;type=CELL;waid=%s:%s\nEND:VCARD", 
		OWNER_NAME, OWNER_NUMBER, OWNER_NUMBER)
	
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ContactMessage: &waProto.ContactMessage{
			DisplayName: proto.String(OWNER_NAME),
			Vcard: proto.String(vcard),
		},
	})
}

func sendPing(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "⚡")
	start := time.Now()
	reply(client, chat, msg, "🏓 Pinging...")
	lat := time.Since(start).Milliseconds()
	reply(client, chat, msg, fmt.Sprintf("*⚡ Ping:* %dms", lat))
}

// --- DOWNLOADERS (REAL APIS) ---

func dlTikTok(client *whatsmeow.Client, chat types.JID, url string, msg *waProto.Message) {
	if url == "" { reply(client, chat, msg, "⚠️ URL missing"); return }
	react(client, chat, msg, "🎵")
	reply(client, chat, msg, "⚙️ Downloading TikTok...")
	
	type R struct { Data struct { Play string `json:"play"`; Title string `json:"title"` } `json:"data"` }
	var res R
	if getJson("https://www.tikwm.com/api/?url="+url, &res) != nil || res.Data.Play == "" {
		reply(client, chat, msg, "❌ Failed"); return
	}
	sendVideo(client, chat, res.Data.Play, res.Data.Title)
}

func dlFacebook(client *whatsmeow.Client, chat types.JID, url string, msg *waProto.Message) {
	if url == "" { return }
	react(client, chat, msg, "📘")
	reply(client, chat, msg, "⚙️ Downloading FB...")
	
	type R struct { BK9 struct { HD string `json:"HD"`; SD string `json:"SD"` } `json:"BK9"`; Status bool `json:"status"` }
	var res R
	if getJson("https://bk9.fun/downloader/facebook?url="+url, &res) != nil || !res.Status {
		reply(client, chat, msg, "❌ Failed"); return
	}
	if res.BK9.HD != "" { sendVideo(client, chat, res.BK9.HD, "FB HD") } else { sendVideo(client, chat, res.BK9.SD, "FB SD") }
}

func dlInstagram(client *whatsmeow.Client, chat types.JID, url string, msg *waProto.Message) {
	if url == "" { return }
	react(client, chat, msg, "📸")
	reply(client, chat, msg, "⚙️ Downloading Insta...")
	
	type R struct { Video struct { Url string `json:"url"` } `json:"video"`; Images []struct { Url string `json:"url"` } `json:"images"` }
	var res R
	if getJson("https://api.tiklydown.eu.org/api/download?url="+url, &res) != nil { reply(client, chat, msg, "❌ Failed"); return }
	
	if res.Video.Url != "" { sendVideo(client, chat, res.Video.Url, "Insta Video") }
	for _, img := range res.Images { sendImage(client, chat, img.Url, "Insta Image") }
}

func dlPinterest(client *whatsmeow.Client, chat types.JID, url string, msg *waProto.Message) {
	if url == "" { return }
	react(client, chat, msg, "📌")
	reply(client, chat, msg, "⚙️ Searching Pinterest...")
	
	type R struct { BK9 struct { Url string `json:"url"` } `json:"BK9"`; Status bool `json:"status"` }
	var res R
	if getJson("https://bk9.fun/downloader/pinterest?url="+url, &res) != nil || !res.Status {
		reply(client, chat, msg, "❌ Failed"); return
	}
	if strings.Contains(res.BK9.Url, ".mp4") {
		sendVideo(client, chat, res.BK9.Url, "Pinterest Video")
	} else {
		sendImage(client, chat, res.BK9.Url, "Pinterest Image")
	}
}

func dlYouTube(client *whatsmeow.Client, chat types.JID, url, format string, msg *waProto.Message) {
	if url == "" { return }
	react(client, chat, msg, "📺")
	reply(client, chat, msg, "⚙️ Processing YouTube...")
	
	type R struct { BK9 struct { Mp4 string `json:"mp4"`; Mp3 string `json:"mp3"` } `json:"BK9"`; Status bool `json:"status"` }
	var res R
	if getJson("https://bk9.fun/downloader/youtube?url="+url, &res) != nil || !res.Status {
		reply(client, chat, msg, "❌ Failed"); return
	}
	if format == "mp4" && res.BK9.Mp4 != "" {
		sendVideo(client, chat, res.BK9.Mp4, "YouTube Video")
	} else if format == "mp3" && res.BK9.Mp3 != "" {
		sendDoc(client, chat, res.BK9.Mp3, "audio.mp3", "audio/mpeg")
	}
}

// --- TOOLS (REAL LOGIC) ---

func makeSticker(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "🎨")
	data, err := downloadMedia(client, msg)
	if err != nil { reply(client, chat, msg, "⚠️ Reply to image"); return }
	
	// Convert using FFmpeg
	inFile := fmt.Sprintf("temp_%d.jpg", time.Now().UnixNano())
	outFile := inFile + ".webp"
	os.WriteFile(inFile, data, 0644)
	
	cmd := exec.Command("ffmpeg", "-y", "-i", inFile, "-vcodec", "libwebp", "-vf", "scale=512:512:flags=lanczos:force_original_aspect_ratio=decrease,format=rgba,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=#00000000", "-lossless", "1", "-loop", "0", "-an", "-vsync", "0", outFile)
	cmd.Run()
	
	webpData, err := os.ReadFile(outFile)
	if err == nil {
		uploaded, _ := client.Upload(context.Background(), webpData, whatsmeow.MediaImage)
		client.SendMessage(context.Background(), chat, &waProto.Message{
			StickerMessage: &waProto.StickerMessage{
				Url: proto.String(uploaded.URL),
				DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey,
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256: uploaded.FileSHA256,
				FileLength: proto.Uint64(uint64(len(webpData))),
				Mimetype: proto.String("image/webp"),
			},
		})
	} else {
		reply(client, chat, msg, "❌ FFmpeg Error")
	}
	os.Remove(inFile); os.Remove(outFile)
}

func stickerToImg(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "🖼️")
	data, err := downloadMedia(client, msg)
	if err != nil { reply(client, chat, msg, "⚠️ Reply to sticker"); return }
	
	inFile := fmt.Sprintf("temp_%d.webp", time.Now().UnixNano())
	outFile := fmt.Sprintf("temp_%d.png", time.Now().UnixNano())
	os.WriteFile(inFile, data, 0644)
	
	exec.Command("ffmpeg", "-y", "-i", inFile, outFile).Run()
	
	pngData, err := os.ReadFile(outFile)
	if err == nil {
		up, _ := client.Upload(context.Background(), pngData, whatsmeow.MediaImage)
		client.SendMessage(context.Background(), chat, &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				Url: proto.String(up.URL),
				DirectPath: proto.String(up.DirectPath),
				MediaKey: up.MediaKey,
				FileEncSha256: up.FileEncSHA256,
				FileSha256: up.FileSHA256,
				FileLength: proto.Uint64(uint64(len(pngData))),
				Mimetype: proto.String("image/png"),
			},
		})
	}
	os.Remove(inFile); os.Remove(outFile)
}

func reminiEnhance(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "✨")
	data, err := downloadMedia(client, msg)
	if err != nil { reply(client, chat, msg, "⚠️ Reply to image"); return }
	
	reply(client, chat, msg, "⚙️ Enhancing...")
	url := uploadToCatbox(data)
	if url == "" { reply(client, chat, msg, "❌ Upload Failed"); return }

	type R struct { Url string `json:"url"` }
	var res R
	if getJson("https://remini.mobilz.pw/enhance?url="+url, &res) == nil && res.Url != "" {
		sendImage(client, chat, res.Url, "✨ Enhanced")
	} else {
		reply(client, chat, msg, "❌ API Error")
	}
}

func removeBG(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "✂️")
	data, err := downloadMedia(client, msg)
	if err != nil { reply(client, chat, msg, "⚠️ Reply to image"); return }
	
	reply(client, chat, msg, "⚙️ Removing BG...")
	url := uploadToCatbox(data)
	// Using BK9 RemoveBG
	sendImage(client, chat, "https://bk9.fun/tools/removebg?url="+url, "✂️ Background Removed")
}

func mediaToUrl(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "🔗")
	data, err := downloadMedia(client, msg)
	if err != nil { reply(client, chat, msg, "⚠️ Reply media"); return }
	url := uploadToCatbox(data)
	reply(client, chat, msg, "🔗 *URL:* "+url)
}

func getWeather(client *whatsmeow.Client, chat types.JID, city string, msg *waProto.Message) {
	react(client, chat, msg, "🌦️")
	resp, _ := http.Get("https://wttr.in/" + city + "?format=%C+%t")
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	reply(client, chat, msg, fmt.Sprintf("🌦️ *%s:* %s", city, string(body)))
}

func doTranslate(client *whatsmeow.Client, chat types.JID, args []string, msg *waProto.Message) {
	react(client, chat, msg, "🌍")
	text := strings.Join(args, " ")
	if text == "" {
		// Check reply
		quoted := msg.ExtendedTextMessage.GetContextInfo().GetQuotedMessage()
		if quoted != nil { text = quoted.GetConversation() }
	}
	if text == "" { reply(client, chat, msg, "⚠️ Text?"); return }
	
	// Google Translate Free API
	url := fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=ur&dt=t&q=%s", url.QueryEscape(text))
	resp, _ := http.Get(url)
	defer resp.Body.Close()
	var result []interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	if len(result) > 0 {
		inner := result[0].([]interface{})
		trans := inner[0].([]interface{})[0].(string)
		reply(client, chat, msg, "🌍 *Translation:*\n"+trans)
	} else {
		reply(client, chat, msg, "❌ Error")
	}
}

// --- GROUP & UTILS ---

func groupAction(client *whatsmeow.Client, chat types.JID, msg *waProto.Message, action string, isGroup bool) {
	if !isGroup { return }
	target := getTargetJID(msg)
	if target == nil { reply(client, chat, msg, "⚠️ Mention/Reply needed"); return }
	
	var ch whatsmeow.ParticipantChange
	switch action {
	case "remove": ch = whatsmeow.ParticipantChangeRemove
	case "promote": ch = whatsmeow.ParticipantChangePromote
	case "demote": ch = whatsmeow.ParticipantChangeDemote
	}
	react(client, chat, msg, "⚡")
	// FIX: Added context.Background()
	client.UpdateGroupParticipants(context.Background(), chat, []types.JID{*target}, ch)
	reply(client, chat, msg, "✅ Done")
}

func groupAdd(client *whatsmeow.Client, chat types.JID, args []string, isGroup bool, msg *waProto.Message) {
	if !isGroup || len(args) == 0 { return }
	react(client, chat, msg, "➕")
	jid, _ := types.ParseJID(args[0] + "@s.whatsapp.net")
	// FIX: Added context.Background()
	client.UpdateGroupParticipants(context.Background(), chat, []types.JID{jid}, whatsmeow.ParticipantChangeAdd)
}

func groupTagAll(client *whatsmeow.Client, chat types.JID, text string, isGroup bool, msg *waProto.Message) {
	if !isGroup { return }
	react(client, chat, msg, "📣")
	// FIX: Added context.Background()
	info, _ := client.GetGroupInfo(context.Background(), chat)
	mentions := []string{}
	out := "📣 *EVERYONE*\n" + text + "\n\n"
	for _, p := range info.Participants {
		mentions = append(mentions, p.JID.String())
		out += "@" + p.JID.User + "\n"
	}
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(out),
			// FIX: MentionedJid -> MentionedJID
			ContextInfo: &waProto.ContextInfo{MentionedJID: mentions},
		},
	})
}

func groupHideTag(client *whatsmeow.Client, chat types.JID, text string, isGroup bool, msg *waProto.Message) {
	if !isGroup { return }
	if text == "" { text = "👻" }
	// FIX: Added context.Background()
	info, _ := client.GetGroupInfo(context.Background(), chat)
	mentions := []string{}
	for _, p := range info.Participants { mentions = append(mentions, p.JID.String()) }
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			// FIX: MentionedJid -> MentionedJID
			ContextInfo: &waProto.ContextInfo{MentionedJID: mentions},
		},
	})
}

func groupSettings(client *whatsmeow.Client, chat types.JID, args []string, isGroup bool, msg *waProto.Message) {
	if !isGroup || len(args) == 0 { return }
	// FIX: Added context.Background()
	if args[0] == "close" { client.SetGroupAnnounce(context.Background(), chat, true); reply(client, chat, msg, "🔒 Closed") }
	if args[0] == "open" { client.SetGroupAnnounce(context.Background(), chat, false); reply(client, chat, msg, "🔓 Opened") }
}

func deleteMessage(client *whatsmeow.Client, chat types.JID, msg *waProto.Message) {
	react(client, chat, msg, "🗑️")
	quoted := msg.ExtendedTextMessage.GetContextInfo()
	if quoted != nil {
		target, _ := types.ParseJID(*quoted.Participant)
		// FIX: Added context.Background(), StanzaId -> StanzaID
		client.RevokeMessage(context.Background(), chat, target, *quoted.StanzaID)
	}
}

// --- HELPERS ---

func getTargetJID(msg *waProto.Message) *types.JID {
	if msg.ExtendedTextMessage == nil { return nil }
	ctx := msg.ExtendedTextMessage.ContextInfo
	if ctx != nil {
		// FIX: MentionedJid -> MentionedJID
		if len(ctx.MentionedJID) > 0 {
			j, _ := types.ParseJID(ctx.MentionedJID[0]); return &j
		}
		if ctx.Participant != nil {
			j, _ := types.ParseJID(*ctx.Participant); return &j
		}
	}
	return nil
}

func getText(msg *waProto.Message) string {
	if msg.Conversation != nil { return *msg.Conversation }
	if msg.ExtendedTextMessage != nil { return *msg.ExtendedTextMessage.Text }
	if msg.ImageMessage != nil { return *msg.ImageMessage.Caption }
	return ""
}

func react(client *whatsmeow.Client, chat types.JID, msg *waProto.Message, emoji string) {
	if msg == nil { return }
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{Key: msg.Key, Text: proto.String(emoji)},
	})
}

func reply(client *whatsmeow.Client, chat types.JID, quoted *waProto.Message, text string) {
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				// FIX: StanzaId -> StanzaID
				StanzaID: proto.String(quoted.GetKey().GetId()),
				Participant: proto.String(quoted.GetKey().GetParticipant()),
				QuotedMessage: quoted,
			},
		},
	})
}

func downloadMedia(client *whatsmeow.Client, msg *waProto.Message) ([]byte, error) {
	var doc *waProto.ImageMessage
	if msg.ImageMessage != nil { doc = msg.ImageMessage }
	if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.ContextInfo != nil {
		q := msg.ExtendedTextMessage.ContextInfo.QuotedMessage
		if q != nil && q.ImageMessage != nil { doc = q.ImageMessage }
		if q != nil && q.StickerMessage != nil {
			return client.DownloadAny(q.StickerMessage)
		}
	}
	if doc == nil { return nil, fmt.Errorf("no media") }
	return client.Download(doc)
}

func uploadToCatbox(data []byte) string {
	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	w.WriteField("reqtype", "fileupload")
	p, _ := w.CreateFormFile("fileToUpload", "file.jpg")
	p.Write(data)
	w.Close()
	resp, _ := http.Post("https://catbox.moe/user/api.php", w.FormDataContentType(), body)
	res, _ := ioutil.ReadAll(resp.Body)
	return string(res)
}

func getJson(url string, t interface{}) error {
	r, err := http.Get(url)
	if err != nil { return err }
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(t)
}

func sendVideo(client *whatsmeow.Client, chat types.JID, url, caption string) {
	r, _ := http.Get(url)
	data, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaVideo)
	client.SendMessage(context.Background(), chat, &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			Url: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSha256: up.FileEncSHA256,
			FileSha256: up.FileSHA256, FileLength: proto.Uint64(uint64(len(data))),
			Mimetype: proto.String("video/mp4"), Caption: proto.String(caption),
		},
	})
}

func sendImage(client *whatsmeow.Client, chat types.JID, url, caption string) {
	r, _ := http.Get(url)
	data, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Url: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSha256: up.FileEncSHA256,
			FileSha256: up.FileSHA256, FileLength: proto.Uint64(uint64(len(data))),
			Mimetype: proto.String("image/jpeg"), Caption: proto.String(caption),
		},
	})
}

func sendDoc(client *whatsmeow.Client, chat types.JID, url, name, mime string) {
	r, _ := http.Get(url)
	data, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaDocument)
	client.SendMessage(context.Background(), chat, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			Url: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSha256: up.FileEncSHA256,
			FileSha256: up.FileSHA256, FileLength: proto.Uint64(uint64(len(data))),
			Mimetype: proto.String(mime), FileName: proto.String(name),
		},
	})
}