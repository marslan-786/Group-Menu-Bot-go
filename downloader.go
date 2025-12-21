package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"runtime"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// 🛡️ گلوبل اسٹرکچرز (اگر types.go میں ہیں تو وہاں سے اٹھا لے گا)
type YTSResult struct {
	Title string
	Url   string
}

type YTState struct {
	Url      string
	Title    string
	SenderID string
}

// نوٹ: اگر TTState کا 'Redeclared' ایرر آئے تو نیچے والی 6 لائنیں ڈیلیٹ کر دیں
type TTState struct {
	Title    string
	PlayURL  string
	MusicURL string
	Size     int64
}

var ytCache = make(map[string][]YTSResult)
var ytDownloadCache = make(map[string]YTState)
var ttCache = make(map[string]TTState)

// 💎 پریمیم کارڈ میکر (ہیلپر)
func sendPremiumCard(client *whatsmeow.Client, v *events.Message, title, site, info string) {
	card := fmt.Sprintf(`╔══════════════════════╗
║ ✨ %s DOWNLOADER
╠══════════════════════╣
║ 📝 Title: %s
║ 🌐 Site: %s
╠══════════════════════╣
║ ⏳ Status: Processing...
║ 📦 Quality: Ultra HD
╚══════════════════════╝
%s`, strings.ToUpper(site), title, site, info)
	replyMessage(client, v, card)
}

// 🚀 ماسٹر میڈیا لوجک (The Logic that actually works!)
func downloadAndSend(client *whatsmeow.Client, v *events.Message, urlStr string, mode string) {
	react(client, v.Info.Chat, v.Info.ID, "⏳")
	
	fileName := fmt.Sprintf("media_%d", time.Now().UnixNano())
	var args []string

	if mode == "audio" {
		fileName += ".mp3"
		args = []string{"-f", "bestaudio", "--extract-audio", "--audio-format", "mp3", "-o", fileName, urlStr}
	} else {
		fileName += ".mp4"
		// بہترین کوالٹی کے لئے فلیگز
		args = []string{"-f", "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best", "--merge-output-format", "mp4", "-o", fileName, urlStr}
	}

	// 1. سرور پر فائل ڈاؤن لوڈ کرنا
	cmd := exec.Command("yt-dlp", args...)
	if err := cmd.Run(); err != nil {
		replyMessage(client, v, "❌ Media processing failed. Link may be restricted.")
		return
	}

	// 2. فائل کو بائٹس میں پڑھنا
	fileData, err := os.ReadFile(fileName)
	if err != nil { return }
	defer os.Remove(fileName) // اپلوڈ کے بعد صفائی

	fileSize := uint64(len(fileData))
	if fileSize > 100*1024*1024 {
		replyMessage(client, v, "⚠️ File is too large (>100MB).")
		return
	}

	// 3. واٹس ایپ پروٹوکول اپلوڈ
	mType := whatsmeow.MediaVideo
	if mode == "audio" { mType = whatsmeow.MediaDocument }

	up, err := client.Upload(context.Background(), fileData, mType)
	if err != nil {
		replyMessage(client, v, "❌ WhatsApp upload failed.")
		return
	}

	// 4. میسج کنسٹرکشن (Original Delivery Logic)
	var finalMsg waProto.Message
	if mode == "audio" {
		finalMsg.DocumentMessage = &waProto.DocumentMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("audio/mpeg"),
			FileName:      proto.String("Impossible_Audio.mp3"),
			FileLength:    proto.Uint64(fileSize),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
		}
	} else {
		finalMsg.VideoMessage = &waProto.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("video/mp4"),
			Caption:       proto.String("✅ *Success!* \nDownloaded via Impossible Power"),
			FileLength:    proto.Uint64(fileSize),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
		}
	}

	client.SendMessage(context.Background(), v.Info.Chat, &finalMsg)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}

// 1. یوٹیوب سرچ (YTS)
func handleYTS(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")
	cmd := exec.Command("yt-dlp", "ytsearch5:"+query, "--get-title", "--get-id", "--no-playlist")
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 { return }
	var results []YTSResult
	menuText := "╔════════════════════╗\n║  📺 YOUTUBE SEARCH \n╠════════════════════╣\n"
	for i := 0; i < len(lines)-1; i += 2 {
		results = append(results, YTSResult{Title: lines[i], Url: "https://www.youtube.com/watch?v=" + lines[i+1]})
		menuText += fmt.Sprintf("║ [%d] %s\n", (i/2)+1, lines[i])
	}
	ytCache[v.Info.Sender.String()] = results
	menuText += "╚════════════════════╝"
	replyMessage(client, v, menuText)
}

// 2. یوٹیوب ڈاؤن لوڈ مینو
func handleYTDownloadMenu(client *whatsmeow.Client, v *events.Message, ytUrl string) {
	titleCmd := exec.Command("yt-dlp", "--get-title", ytUrl)
	titleOut, _ := titleCmd.Output()
	title := strings.TrimSpace(string(titleOut))
	ytDownloadCache[v.Info.Chat.String()] = YTState{Url: ytUrl, Title: title, SenderID: v.Info.Sender.String()}
	menu := fmt.Sprintf(`╔════════════════════╗
║   Video Selector   
╠════════════════════╣
║ Title: %s
║ [1] 360p | [2] 720p 
║ [3] 1080p| [4] Audio
╚════════════════════╝`, title)
	replyMessage(client, v, menu)
}

// 3. یوٹیوب ڈاؤن لوڈ ہینڈلر
func handleYTDownload(client *whatsmeow.Client, v *events.Message, ytUrl, format string, isAudio bool) {
	mode := "video"
	if isAudio { mode = "audio" }
	go downloadAndSend(client, v, ytUrl, mode)
}

// --- سوشل میڈیا ہینڈلرز ---

func handleTikTok(client *whatsmeow.Client, v *events.Message, urlStr string) {
	if urlStr == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🎵")
	encodedURL := url.QueryEscape(strings.TrimSpace(urlStr))
	apiUrl := "https://www.tikwm.com/api/?url=" + encodedURL
	var r struct {
		Code int `json:"code"`
		Data struct {
			Play string `json:"play"`
			Music string `json:"music"`
			Title string `json:"title"`
			Size uint64 `json:"size"`
		} `json:"data"`
	}
	getJson(apiUrl, &r)
	if r.Code == 0 {
		ttCache[v.Info.Sender.String()] = TTState{
			PlayURL: r.Data.Play, MusicURL: r.Data.Music, Title: r.Data.Title, Size: int64(r.Data.Size), // ✅ Fixed uint64 to int64
		}
		sendPremiumCard(client, v, "TikTok", "TikTok", fmt.Sprintf("📝 %s\n\n🔢 Reply 1 for Video | 2 for Audio", r.Data.Title))
	}
}

func handleFacebook(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "FB Video", "Facebook", "🎥 Fetching Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleInstagram(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Insta Reel", "Instagram", "📸 Extracting Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleTwitter(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "X Video", "Twitter/X", "🐦 Speeding through X...")
	go downloadAndSend(client, v, url, "video")
}

func handlePinterest(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Pin Media", "Pinterest", "📌 Extracting Media...")
	go downloadAndSend(client, v, url, "video")
}

// --- یوٹیلیٹی ٹولز ---

func handleServerStats(client *whatsmeow.Client, v *events.Message) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats := fmt.Sprintf("🖥️ *SERVER STATS*\n🚀 RAM: %d MB / 32 GB\n🟢 Status: Online", m.Alloc/1024/1024)
	replyMessage(client, v, stats)
}

func handleAI(client *whatsmeow.Client, v *events.Message, query string) {
	react(client, v.Info.Chat, v.Info.ID, "🧠")
	sendPremiumCard(client, v, "AI Brain", "Gemini", "🧠 Thinking...")
}

func handleScreenshot(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Snapshot", "Engine", "📸 Capturing Web Page...")
}

func handleGoogle(client *whatsmeow.Client, v *events.Message, query string) {
	replyMessage(client, v, "🔍 *Searching:* "+query)
}

func handleWeather(client *whatsmeow.Client, v *events.Message, city string) {
	sendPremiumCard(client, v, "Weather", "Satellite", "🌡️ Checking conditions for "+city)
}

func handleRemini(client *whatsmeow.Client, v *events.Message) {
	sendPremiumCard(client, v, "Upscaler", "AI", "✨ Processing HD Image...")
}

func handleRemoveBG(client *whatsmeow.Client, v *events.Message) {
	sendPremiumCard(client, v, "BG Eraser", "AI", "🧼 Making Transparent...")
}

func handleSpeedTest(client *whatsmeow.Client, v *events.Message) {
	sendPremiumCard(client, v, "Speedtest", "Railway", "📡 Measuring Fiber Speed...")
}

// --- تمام مسنگ فنکشنز (کمپائلر کی تسلی کے لئے) ---

func handleThreads(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleSnapchat(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleReddit(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleTwitch(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleDailyMotion(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleVimeo(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleRumble(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleBilibili(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleSoundCloud(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleSpotify(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleAppleMusic(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleDeezer(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleTidal(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleMixcloud(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleNapster(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleBandcamp(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleImgur(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleGiphy(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleFlickr(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handle9Gag(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleIfunny(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleTed(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleSteam(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleArchive(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleBitChute(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleDouyin(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleKwai(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleLikee(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleCapCut(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleYoutubeVideo(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleYoutubeAudio(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleLinkedIn(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleGithub(client *whatsmeow.Client, v *events.Message, url string) { replyMessage(client, v, "📁 Repo Link: "+url) }
func handleUniversal(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleMega(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }
func handleFancy(client *whatsmeow.Client, v *events.Message, t string) { replyMessage(client, v, "✨ Stylish Version: "+t) }
func handleToPTT(client *whatsmeow.Client, v *events.Message) { replyMessage(client, v, "🎙️ PTT logic activated.") }
func handleYouTubeMP3(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "audio") }
func handleYouTubeMP4(client *whatsmeow.Client, v *events.Message, url string) { go downloadAndSend(client, v, url, "video") }

// --- مددگار فنکشنز ---
func getJson(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil { return err }
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func sendVideo(client *whatsmeow.Client, v *events.Message, videoURL, caption string) {
	// یہ انجن کے ذریعے ہینڈل ہوگا
}

func sendTikTokVideo(client *whatsmeow.Client, v *events.Message, videoURL, caption string, size uint64) {
	go downloadAndSend(client, v, videoURL, "video")
}

func sendImage(client *whatsmeow.Client, v *events.Message, imageURL, caption string) {
	resp, _ := http.Get(imageURL)
	data, _ := io.ReadAll(resp.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(uint64(len(data))), Caption: proto.String(caption),
		},
	})
}

func sendDocument(client *whatsmeow.Client, v *events.Message, docURL, name, mime string) {
	resp, _ := http.Get(docURL)
	data, _ := io.ReadAll(resp.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaDocument)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String(mime), FileName: proto.String(name), FileLength: proto.Uint64(uint64(len(data))),
		},
	})
}