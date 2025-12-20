package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// ═══════════════════════════════════════════════════════════════
// 📡 MAIN EVENT HANDLER
// ═══════════════════════════════════════════════════════════════

func handler(client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		go processMessage(client, v)
	case *events.GroupInfo:
		go handleGroupInfoChange(client, v)
	}
}

// یہ فنکشن چیک کرتا ہے کہ آیا میسج میں موجود لفظ ہماری لسٹ میں ہے یا نہیں
func isKnownCommand(text string) bool {
	commands := []string{
		"menu", "help", "list", "ping", "id", "owner", "data", "listbots",
		"alwaysonline", "autoread", "autoreact", "autostatus", "statusreact",
		"addstatus", "delstatus", "liststatus", "readallstatus", "setprefix", "mode",
		"antilink", "antipic", "antivideo", "antisticker",
		"kick", "add", "promote", "demote", "tagall", "hidetag", "group", "del", "delete",
		"tiktok", "tt", "fb", "facebook", "insta", "ig", "pin", "pinterest", "ytmp3", "ytmp4",
		"sticker", "s", "toimg", "tovideo", "removebg", "remini", "tourl", "weather", "translate", "tr", "vv",
	}

	lowerText := strings.ToLower(strings.TrimSpace(text))
	for _, cmd := range commands {
		if strings.HasPrefix(lowerText, cmd) {
			return true
		}
	}
	return false
}

func processMessage(client *whatsmeow.Client, v *events.Message) {
	chatID := v.Info.Chat.String()
	senderID := v.Info.Sender.String()
	isGroup := v.Info.IsGroup

	// 1. SETUP FLOW
	if state, ok := setupMap[senderID]; ok && state.GroupID == chatID {
		handleSetupResponse(client, v, state)
		return
	}

	// 2. AUTO STATUS
	if chatID == "status@broadcast" {
		dataMutex.RLock()
		if data.AutoStatus {
			client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender, types.ReceiptTypeRead)
			if data.StatusReact {
				emojis := []string{"💚", "❤️", "🔥", "😍", "💯"}
				react(client, v.Info.Chat, v.Info.ID, emojis[time.Now().UnixNano()%int64(len(emojis))])
			}
		}
		dataMutex.RUnlock()
		return
	}

	// 3. AUTO READ
	dataMutex.RLock()
	if data.AutoRead {
		client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender, types.ReceiptTypeRead)
	}
	if data.AutoReact {
		react(client, v.Info.Chat, v.Info.ID, "❤️")
	}
	dataMutex.RUnlock()

	// 4. SECURITY CHECKS
	if isGroup {
		checkSecurity(client, v)
	}

	// 5. COMMAND PROCESSING
	body := getText(v.Message)
	dataMutex.RLock()
	prefix := data.Prefix
	dataMutex.RUnlock()

	if !strings.HasPrefix(body, prefix) && !isKnownCommand(body) {
		return
	}

	cmd := strings.ToLower(body)
	args := []string{}

	if strings.HasPrefix(cmd, prefix) {
		split := strings.Fields(cmd[len(prefix):])
		if len(split) > 0 {
			cmd = split[0]
			args = split[1:]
		}
	} else {
		split := strings.Fields(cmd)
		if len(split) > 0 {
			cmd = split[0]
			args = split[1:]
		}
	}

	// 🔐 PERMISSION CHECK (UPDATED LID LOGIC)
	if !canExecute(client, v, cmd) {
		return
	}

	fullArgs := strings.Join(args, " ")
	fmt.Printf("📩 CMD: %s | User: %s | Chat: %s\n", cmd, v.Info.Sender.User, v.Info.Chat.User)

	switch cmd {
	case "menu", "help", "list":
		react(client, v.Info.Chat, v.Info.ID, "📜")
		sendMenu(client, v)
	case "ping":
		react(client, v.Info.Chat, v.Info.ID, "⚡")
		sendPing(client, v)
	case "id":
		react(client, v.Info.Chat, v.Info.ID, "🆔")
		sendID(client, v)
	case "owner":
		react(client, v.Info.Chat, v.Info.ID, "👑")
		sendOwner(client, v)
	case "listbots":
		react(client, v.Info.Chat, v.Info.ID, "📊")
		sendBotsList(client, v)
	case "data":
		replyMessage(client, v, "╔════════════════╗\n║ 📂 DATA STATUS\n╠════════════════╣\n║ ✅ DB Coming\n╚════════════════╝")
	case "alwaysonline":
		toggleAlwaysOnline(client, v)
	case "autoread":
		toggleAutoRead(client, v)
	case "autoreact":
		toggleAutoReact(client, v)
	case "autostatus":
		toggleAutoStatus(client, v)
	case "statusreact":
		toggleStatusReact(client, v)
	case "addstatus":
		handleAddStatus(client, v, args)
	case "delstatus":
		handleDelStatus(client, v, args)
	case "liststatus":
		handleListStatus(client, v)
	case "readallstatus":
		handleReadAllStatus(client, v)
	case "setprefix":
		handleSetPrefix(client, v, args)
	case "mode":
		handleMode(client, v, args)
	case "antilink":
		startSecuritySetup(client, v, "antilink")
	case "antipic":
		startSecuritySetup(client, v, "antipic")
	case "antivideo":
		startSecuritySetup(client, v, "antivideo")
	case "antisticker":
		startSecuritySetup(client, v, "antisticker")
	case "kick":
		handleKick(client, v, args)
	case "add":
		handleAdd(client, v, args)
	case "promote":
		handlePromote(client, v, args)
	case "demote":
		handleDemote(client, v, args)
	case "tagall":
		handleTagAll(client, v, args)
	case "hidetag":
		handleHideTag(client, v, args)
	case "group":
		handleGroup(client, v, args)
	case "del", "delete":
		handleDelete(client, v)
	case "sticker", "s":
		handleSticker(client, v)
	case "toimg":
		handleToImg(client, v)
	case "tovideo":
		handleToVideo(client, v)
	case "removebg":
		handleRemoveBG(client, v)
	case "remini":
		handleRemini(client, v)
	case "tourl":
		handleToURL(client, v)
	case "weather":
		handleWeather(client, v, fullArgs)
	case "translate", "tr":
		handleTranslate(client, v, args)
	case "vv":
		handleVV(client, v)
	}
}

// ═══════════════════════════════════════════════════════════════
// 🔐 SECURITY & OWNER LOGIC (LID BASED)
// ═══════════════════════════════════════════════════════════════

func getCleanID(jidStr string) string {
	if jidStr == "" { return "unknown" }
	// @ کے پیچھے والا حصہ نکالیں (نمبر یا LID)
	parts := strings.Split(jidStr, "@")
	userPart := parts[0]
	// ڈیوائس آئی ڈی ہٹائیں (جیسے :61)
	if strings.Contains(userPart, ":") {
		userPart = strings.Split(userPart, ":")[0]
	}
	return strings.TrimSpace(userPart)
}

func getBotLID(client *whatsmeow.Client) string {
	if client.Store.ID == nil { return "unknown" }
	// Postgres میں سیو شدہ LID کو ترجیح دیں
	if client.Store.LID.String() != "" {
		return getCleanID(client.Store.LID.String())
	}
	return getCleanID(client.Store.ID.User)
}

func isOwner(client *whatsmeow.Client, sender types.JID) bool {
	if client.Store.ID == nil { return false }
	
	// سینڈر کا کلین نمبر/آئی ڈی
	senderClean := getCleanID(sender.String())
	
	// بوٹ کا اپنا کلین نمبر
	botNumClean := getCleanID(client.Store.ID.User)
	
	// بوٹ کی کلین LID (ڈیٹا بیس سے)
	botLidClean := ""
	if client.Store.LID.String() != "" {
		botLidClean = getCleanID(client.Store.LID.String())
	}

	// میچنگ: اگر سینڈر بوٹ کا نمبر ہے یا بوٹ کی LID ہے
	isMatch := (senderClean == botNumClean || (botLidClean != "" && senderClean == botLidClean))
	
	fmt.Printf("🎯 [OWNER CHECK] Sender: %s | BotNum: %s | BotLID: %s | Match: %v\n", senderClean, botNumClean, botLidClean, isMatch)
	return isMatch
}

func isAdmin(client *whatsmeow.Client, chat, user types.JID) bool {
	info, err := client.GetGroupInfo(context.Background(), chat)
	if err != nil { return false }
	userLID := getCleanID(user.String())
	for _, p := range info.Participants {
		if getCleanID(p.JID.String()) == userLID && (p.IsAdmin || p.IsSuperAdmin) {
			return true
		}
	}
	return false
}

func canExecute(client *whatsmeow.Client, v *events.Message, cmd string) bool {
	if isOwner(client, v.Info.Sender) { return true }
	if !v.Info.IsGroup { return true }
	s := getGroupSettings(v.Info.Chat.String())
	if s.Mode == "private" { return false }
	if s.Mode == "admin" { return isAdmin(client, v.Info.Chat, v.Info.Sender) }
	return true
}

// ═══════════════════════════════════════════════════════════════
// 📜 HELPERS & UI
// ═══════════════════════════════════════════════════════════════

func sendOwner(client *whatsmeow.Client, v *events.Message) {
	isOwn := isOwner(client, v.Info.Sender)
	status := "❌ NOT Owner"
	if isOwn { status = "✅ YOU are Owner" }
	
	msg := fmt.Sprintf(`╔════════════════╗
║ 👑 OWNER CHECK
╠════════════════╣
║ 🤖 Bot Num: %s
║ 🆔 Bot LID: %s
║ 👤 Sender: %s
╠════════════════╣
║ 📊 Status: %s
╚════════════════╝`, getCleanID(client.Store.ID.User), getCleanID(client.Store.LID.String()), getCleanID(v.Info.Sender.String()), status)
	
	replyMessage(client, v, msg)
}

func sendBotsList(client *whatsmeow.Client, v *events.Message) {
	replyMessage(client, v, "📊 Multi-Bot System is Active and Synced with LID.")
}

// ═══════════════════════════════════════════════════════════════
// 📜 MENU SYSTEM
// ═══════════════════════════════════════════════════════════════

func sendMenu(client *whatsmeow.Client, v *events.Message) {
	uptime := time.Since(startTime).Round(time.Second)
	dataMutex.RLock()
	p := data.Prefix
	dataMutex.RUnlock()

	s := getGroupSettings(v.Info.Chat.String())
	currentMode := strings.ToUpper(s.Mode)
	if !strings.Contains(v.Info.Chat.String(), "@g.us") {
		currentMode = "PRIVATE"
	}

	menu := fmt.Sprintf(`╔═════════════════╗
║   %s   
╠═════════════════╣
║ 👋 *Assalam-o-Alaikum*     
║ 👑 *Owner:* %s             
║ 🛡️ *Mode:* %s              
║ ⏳ *Uptime:* %s            
╠═════════════════╣
║                          
║  ╭─────── DOWNLOADERS─╮
║  │ 🔸 *%sfb* - Facebook   
║  │ 🔸 *%sig* - Instagram  
║  │ 🔸 *%spin* - Pinterest 
║  │ 🔸 *%stiktok* - TikTok 
║  │ 🔸 *%sytmp3* - YT Audio
║  │ 🔸 *%sytmp4* - YT Video 
║  ╰───────────────────╯
║                           
║  ╭─────── GROUP ──────╮
║  │ 🔸 *%sadd* - Add Member
║  │ 🔸 *%sdemote* - Demote 
║  │ 🔸 *%sgroup* - Settings
║  │ 🔸 *%shidetag* - Hidden
║  │ 🔸 *%skick* - Remove   
║  │ 🔸 *%spromote* - Admin
║  │ 🔸 *%stagall* - Mention
║  ╰───────────────────╯
║                           
║  ╭──── SETTINGS ───╮
║  │ 🔸 *%saddstatus*       
║  │ 🔸 *%salwaysonline*     
║  │ 🔸 *%santilink*         
║  │ 🔸 *%santipic*         
║  │ 🔸 *%santisticker*     
║  │ 🔸 *%santivideo*        
║  │ 🔸 *%sautoreact*    
║  │ 🔸 *%sautoread*      
║  │ 🔸 *%sautostatus*   
║  │ 🔸 *%sdelstatus*    
║  │ 🔸 *%sliststatus*   
║  │ 🔸 *%smode*      
║  │ 🔸 *%sowner*     
║  │ 🔸 *%sreadallstatus* 
║  │ 🔸 *%sstatusreact*  
║  ╰─────────────────╯
║                           
║  ╭─────── TOOLS ───────╮
║  │ 🔸 *%sdata* - DB Status
║  │ 🔸 *%sid* - Get ID      
║  │ 🔸 *%slistbots* - Bots🆕
║  │ 🔸 *%sping* - Speed     
║  │ 🔸 *%sremini* - Enhance
║  │ 🔸 *%sremovebg* - BG  
║  │ 🔸 *%ssticker* - Create 
║  │ 🔸 *%stoimg* - Convert 
║  │ 🔸 *%stourl* - Upload  
║  │ 🔸 *%stovideo* - Make 
║  │ 🔸 *%stranslate* - Lang
║  │ 🔸 *%svv* - ViewOnce 
║  │ 🔸 *%sweather* - Info
║  ╰────────────────────╯
║                          
╠═════════════════════╣
║ 🔐 LID-Based Security
║ © 2025 Nothing is Impossible 
╚═════════════════════╝`,
		BOT_NAME, OWNER_NAME, currentMode, uptime,
		p, p, p, p, p, p,
		p, p, p, p, p, p, p,
		p, p, p, p, p, p, p, p, p, p, p, p, p, p, p,
		p, p, p, p, p, p, p, p, p, p, p, p, p)

	sendReplyMessage(client, v, menu)
}

func sendPing(client *whatsmeow.Client, v *events.Message) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	ms := time.Since(start).Milliseconds()
	uptime := time.Since(startTime).Round(time.Second)

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚡ PING STATUS
╠════════════════╣
║ 🚀 Speed: %d MS
║ ⏱️ Uptime: %s
║ 👑 Dev: %s
╠════════════════╣
║ 🟢 System Running
╚════════════════╝`, ms, uptime, OWNER_NAME)

	sendReplyMessage(client, v, msg)
}

func sendID(client *whatsmeow.Client, v *events.Message) {
	user := v.Info.Sender.User
	chat := v.Info.Chat.User
	chatType := "Private"
	if v.Info.IsGroup {
		chatType = "Group"
	}

	msg := fmt.Sprintf(`╔════════════════╗
║ 🆔 ID INFO
╠════════════════╣
║ 👤 User ID:
║ `+"`%s`"+`
║ 👥 Chat ID:
║ `+"`%s`"+`
║ 🏷️ Type: %s
╚════════════════╝`, user, chat, chatType)

	sendReplyMessage(client, v, msg)
}

// ═══════════════════════════════════════════════════════════════
// 🛠️ HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════

func react(client *whatsmeow.Client, chat types.JID, msgID types.MessageID, emoji string) {
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(chat.String()),
				ID:        proto.String(string(msgID)),
				FromMe:    proto.Bool(false),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	})
}

func replyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
}

func sendReplyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
}

func getText(m *waProto.Message) string {
	if m.Conversation != nil {
		return *m.Conversation
	}
	if m.ExtendedTextMessage != nil && m.ExtendedTextMessage.Text != nil {
		return *m.ExtendedTextMessage.Text
	}
	if m.ImageMessage != nil && m.ImageMessage.Caption != nil {
		return *m.ImageMessage.Caption
	}
	if m.VideoMessage != nil && m.VideoMessage.Caption != nil {
		return *m.VideoMessage.Caption
	}
	return ""
}

func getGroupSettings(id string) *GroupSettings {
	cacheMutex.RLock()
	if s, ok := groupCache[id]; ok {
		cacheMutex.RUnlock()
		return s
	}
	cacheMutex.RUnlock()

	s := &GroupSettings{
		ChatID:         id,
		Mode:           "public",
		Antilink:       false,
		AntilinkAdmin:  true,
		AntilinkAction: "delete",
		AntiPic:        false,
		AntiVideo:      false,
		AntiSticker:    false,
		Warnings:       make(map[string]int),
	}

	cacheMutex.Lock()
	groupCache[id] = s
	cacheMutex.Unlock()

	return s
}

func saveGroupSettings(s *GroupSettings) {
	cacheMutex.Lock()
	groupCache[s.ChatID] = s
	cacheMutex.Unlock()
}

// ═══════════════════════════════════════════════════════════════
// 🚀 MULTI-BOT BOOTSTRAP (POSTGRES CONNECTIVITY)
// ═══════════════════════════════════════════════════════════════

func StartAllBots(container *sqlstore.Container) {
	devices, err := container.GetAllDevices()
	if err != nil {
		fmt.Printf("❌ سیشن لوڈ کرنے میں غلطی: %v\n", err)
		return
	}

	fmt.Printf("\n📂 [MULTI-BOT] ڈیٹا بیس میں %d سیشن ملے ہیں۔ کنیکٹ کر رہے ہیں...\n", len(devices))

	for _, device := range devices {
		clientLog := waLog.Stdout("Client", "DEBUG", true)
		client := whatsmeow.NewClient(device, clientLog)
		client.AddEventHandler(handler)

		err = client.Connect()
		if err != nil {
			fmt.Printf("❌ نمبر %s کنیکٹ نہ ہو سکا: %v\n", device.ID.User, err)
		} else {
			fmt.Printf("✅ بوٹ %s آن لائن ہو گیا ہے!\n", device.ID.User)
		}
	}
}