package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// ==================== ٹولز سسٹم ====================
func handleSticker(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🎨")
	
	msg := `╔═════════════════════╗
║   🎨 STICKER PROCESSING    
╠═════════════════════╣
║  ⏳ Creating sticker...    
║  Please wait...           
╚═════════════════════╝`
	replyMessage(client, v, msg)

	data, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔═════════════════╗
║  ❌ NO MEDIA FOUND       
╠═════════════════╣
║  Reply to an image or     
║  video to create sticker  
╚═════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	ioutil.WriteFile("temp.jpg", data, 0644)
	exec.Command("ffmpeg", "-y", "-i", "temp.jpg", "-vcodec", "libwebp", "temp.webp").Run()
	b, _ := ioutil.ReadFile("temp.webp")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaImage)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("image/webp"),
		},
	})

	os.Remove("temp.jpg")
	os.Remove("temp.webp")
}

func handleToImg(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🖼️")
	
	msg := `╔══════════════════╗
║ 🖼️ IMAGE CONVERSION      
╠══════════════════╣
║ ⏳ Converting to image... 
║       Please wait...           
╚══════════════════╝`
	replyMessage(client, v, msg)  // اب msg صحیح ہے

	data, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔══════════════════╗
║  ❌ NO STICKER FOUND     
╠══════════════════╣
║  Reply to a sticker to    
║  convert it to image      
╚══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	ioutil.WriteFile("temp.webp", data, 0644)
	exec.Command("ffmpeg", "-y", "-i", "temp.webp", "temp.png").Run()
	b, _ := ioutil.ReadFile("temp.png")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaImage)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("image/png"),
			Caption:       proto.String("✅ Converted to Image"),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})

	os.Remove("temp.webp")
	os.Remove("temp.png")
}

func handleToVideo(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🎥")
	
	msg := `╔═════════════════╗
║ 🎥 VIDEO CONVERSION      
╠═════════════════╣
║ ⏳ Converting to video... 
║       Please wait...           
╚═════════════════╝`
	replyMessage(client, v, msg)

	data, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔══════════════════╗
║  ❌ NO STICKER FOUND     
╠══════════════════╣
║  Reply to a sticker to    
║  convert it to video      
╚══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	ioutil.WriteFile("temp.webp", data, 0644)
	exec.Command("ffmpeg", "-y", "-i", "temp.webp", "temp.mp4").Run()
	d, _ := ioutil.ReadFile("temp.mp4")
	up, _ := client.Upload(context.Background(), d, whatsmeow.MediaVideo)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("video/mp4"),
			Caption:       proto.String("✅ Converted to Video"),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})

	os.Remove("temp.webp")
	os.Remove("temp.mp4")
}

func handleRemoveBG(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "✂️")
	
	msg := `╔════════════════════╗
║ ✂️ BACKGROUND REMOVAL     
╠════════════════════╣
║  ⏳ Removing background... 
║          Please wait...           
╚════════════════════╝`
	replyMessage(client, v, msg)

	d, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔═════════════════╗
║  ❌ NO IMAGE FOUND       
╠═════════════════╣
║  Reply to an image to     
║  remove background        
╚═════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	u := uploadToCatbox(d)
	imgURL := "https://bk9.fun/tools/removebg?url=" + u

	r, _ := http.Get(imgURL)
	imgData, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("image/png"),
			Caption:       proto.String("✂️ Background Removed\n\n✅ Successfully Processed"),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
}

func handleRemini(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "✨")
	
	msg := `╔═══════════════════╗
║ ✨ IMAGE ENHANCEMENT     
╠═══════════════════╣
║  ⏳ Enhancing image...     
║       Please wait...           
╚═══════════════════╝`
	replyMessage(client, v, msg)

	d, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔════════════════╗
║ ❌ NO IMAGE FOUND       
╠════════════════╣
║  Reply to an image to     
║  enhance quality          
╚════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	u := uploadToCatbox(d)
	type R struct {
		Url string `json:"url"`
	}
	var r R
	getJson("https://remini.mobilz.pw/enhance?url="+u, &r)

	if r.Url != "" {
		resp, _ := http.Get(r.Url)
		imgData, _ := ioutil.ReadAll(resp.Body)
		up, _ := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)

		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				FileEncSHA256: up.FileEncSHA256,
				FileSHA256:    up.FileSHA256,
				Mimetype:      proto.String("image/jpeg"),
				Caption:       proto.String("✨ Enhanced Image\n\n✅ Quality Improved"),
				ContextInfo: &waProto.ContextInfo{
					StanzaID:      proto.String(v.Info.ID),
					Participant:   proto.String(v.Info.Sender.String()),
					QuotedMessage: v.Message,
			},
			},
		})
	} else {
		errMsg := `╔═══════════════════╗
║ ❌ ENHANCEMENT FAILED   
╠═══════════════════╣
║  Could not enhance image  
║       Please try again         
╚═══════════════════╝`
		replyMessage(client, v, errMsg)
	}
}

func handleToURL(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🔗")
	
	msg := `╔══════════════════╗
║  🔗 UPLOADING MEDIA       
╠══════════════════╣
║ ⏳ Uploading to server... 
║         Please wait...           
╚══════════════════╝`
	replyMessage(client, v, msg)

	d, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔═════════════════╗
║  ❌ NO MEDIA FOUND       
╠═══════════════════╣
║ Reply to media to get URL
╚═══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	uploadURL := uploadToCatbox(d)
	
	resultMsg := fmt.Sprintf(`╔═════════════════╗
║  🔗 MEDIA UPLOADED        
╠═════════════════╣
║                           
║  📎 *Direct Link:*        
║  %s                       
║                           
║ ✅ *Successfully Uploaded*
║                           
╚═══════════════════╝`, uploadURL)

	replyMessage(client, v, resultMsg)
}

func handleWeather(client *whatsmeow.Client, v *events.Message, city string) {
	if city == "" {
		msg := `╔════════════════════╗
║🌤️ WEATHER INFORMATION   
╠════════════════════╣
║                           
║  Usage:                   
║  .weather <city>          
║                           
║  Example:                 
║  .weather Karachi         
║             .weather London          
║                           
╚════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "🌦️")
	
	r, err := http.Get("https://wttr.in/" + city + "?format=%C+%t")
	if err != nil {
		errMsg := `╔═══════════════════╗
║❌ WEATHER FETCH FAILED 
╠═══════════════════╣
║   Could not get weather    
║   Please check city name   
╚═══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	d, _ := ioutil.ReadAll(r.Body)
	weatherInfo := string(d)

	msg := fmt.Sprintf(`╔═══════════════╗
║ 🌤️ WEATHER INFO          
╠═══════════════╣
║                           
║  📍 *City:* %s            
║  🌡️ *Info:* %s            
║                           
╚═══════════════╝`, city, weatherInfo)

	replyMessage(client, v, msg)
}

func handleTranslate(client *whatsmeow.Client, v *events.Message, args []string) {
	react(client, v.Info.Chat, v.Info.ID, "🌍")

	t := strings.Join(args, " ")
	if t == "" {
		if v.Message.ExtendedTextMessage != nil {
			q := v.Message.ExtendedTextMessage.GetContextInfo().GetQuotedMessage()
			if q != nil {
				t = q.GetConversation()
			}
		}
	}

	if t == "" {
		msg := `╔══════════════╗
║   🌍 TRANSLATOR            
╠══════════════╣
║                           
║  Usage:                   
║  .tr <text>               
║                           
║  Or reply to message with:
║  .tr                      
║                           
╚═══════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	r, _ := http.Get(fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=ur&dt=t&q=%s", url.QueryEscape(t)))
	var res []interface{}
	json.NewDecoder(r.Body).Decode(&res)

	if len(res) > 0 {
		translated := res[0].([]interface{})[0].([]interface{})[0].(string)
		msg := fmt.Sprintf(`╔═══════════════════╗
║ 🌍 TRANSLATION RESULT    
╠═══════════════════╣
║                           
║  📝 *Original:*           
║  %s                       
║                           
║  📝 *Translated:*         
║  %s                       
║                           
╚════════════════════╝`, t, translated)

		replyMessage(client, v, msg)
	} else {
		errMsg := `╔══════════════════╗
║ ❌ TRANSLATION FAILED    
╠══════════════════╣
║  Could not translate text 
║  Please try again         
╚══════════════════╝`
		replyMessage(client, v, errMsg)
	}
}

func handleVV(client *whatsmeow.Client, v *events.Message) {
	// 1. React to the command
	react(client, v.Info.Chat, v.Info.ID, "🫣")

	// 2. Identify the quoted message
	if v.Message.GetExtendedTextMessage().GetContextInfo() == nil {
		replyMessage(client, v, "╔═══════════════════╗\n║   ⚠️  REPLY NEEDED  \n╠═══════════════════╣\n║ Please reply to a \n║ media message.    \n╚═══════════════════╝")
		return
	}

	quoted := v.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	if quoted == nil {
		return
	}

	// 3. Media Extraction (Simple & Direct)
	var (
		imgMsg *waProto.ImageMessage
		vidMsg *waProto.VideoMessage
		audMsg *waProto.AudioMessage
	)

	// Check if it's a normal message or wrapped in ViewOnce
	if quoted.ImageMessage != nil {
		imgMsg = quoted.ImageMessage
	} else if quoted.VideoMessage != nil {
		vidMsg = quoted.VideoMessage
	} else if quoted.AudioMessage != nil {
		audMsg = quoted.AudioMessage
	} else if quoted.ViewOnceMessage != nil {
		imgMsg = quoted.ViewOnceMessage.Message.ImageMessage
		vidMsg = quoted.ViewOnceMessage.Message.VideoMessage
	} else if quoted.ViewOnceMessageV2 != nil {
		imgMsg = quoted.ViewOnceMessageV2.Message.ImageMessage
		vidMsg = quoted.ViewOnceMessageV2.Message.VideoMessage
	}

	// 4. Download and Upload Logic
	ctx := context.Background()
	var (
		data []byte
		err  error
		finalMsg = &waProto.Message{}
		caption  = "📂 *MEDIA RETRIEVED*\n\n✅ Successfully copied the message"
	)

	if imgMsg != nil {
		fmt.Println("📸 [VV-LOG] Downloading Image...")
		data, err = client.Download(ctx, imgMsg)
		if err == nil {
			up, _ := client.Upload(ctx, data, whatsmeow.MediaImage)
			finalMsg.ImageMessage = &waProto.ImageMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				Mimetype:      proto.String("image/jpeg"),
				FileEncSHA256: up.FileEncSHA256,
				FileSHA256:    up.FileSHA256,
				Caption:       proto.String(caption),
			}
		}
	} else if vidMsg != nil {
		fmt.Println("🎥 [VV-LOG] Downloading Video...")
		data, err = client.Download(ctx, vidMsg)
		if err == nil {
			up, _ := client.Upload(ctx, data, whatsmeow.MediaVideo)
			finalMsg.VideoMessage = &waProto.VideoMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				Mimetype:      proto.String("video/mp4"),
				FileEncSHA256: up.FileEncSHA256,
				FileSHA256:    up.FileSHA256,
				Caption:       proto.String(caption),
			}
		}
	} else if audMsg != nil {
		fmt.Println("🎤 [VV-LOG] Downloading Audio...")
		data, err = client.Download(ctx, audMsg)
		if err == nil {
			up, _ := client.Upload(ctx, data, whatsmeow.MediaAudio)
			finalMsg.AudioMessage = &waProto.AudioMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				Mimetype:      proto.String("audio/ogg; codecs=opus"),
				FileEncSHA256: up.FileEncSHA256,
				FileSHA256:    up.FileSHA256,
				PTT:           proto.Bool(false),
			}
		}
	}

	// 5. Final Send (Clean Send)
	if finalMsg.ImageMessage == nil && finalMsg.VideoMessage == nil && finalMsg.AudioMessage == nil {
		fmt.Println("❌ [VV-LOG] Media extraction failed or unsupported type.")
		replyMessage(client, v, "╔═══════════════════╗\n║  ❌ NOT SUPPORTED   \n╠═══════════════════╣\n║ This media type is \n║ not supported.     \n╚═══════════════════╝")
		return
	}

	// We are NOT adding ContextInfo with QuotedMessage here to ensure delivery
	_, sendErr := client.SendMessage(ctx, v.Info.Chat, finalMsg)
	if sendErr != nil {
		fmt.Printf("❌ [VV-LOG] Send error: %v\n", sendErr)
	} else {
		fmt.Println("🚀 [VV-LOG] Success! Media sent.")
	}
}


// ==================== میڈیا ہیلپرز ====================
func downloadMedia(client *whatsmeow.Client, m *waProto.Message) ([]byte, error) {
	var d whatsmeow.DownloadableMessage
	if m.ImageMessage != nil {
		d = m.ImageMessage
	} else if m.VideoMessage != nil {
		d = m.VideoMessage
	} else if m.DocumentMessage != nil {
		d = m.DocumentMessage
	} else if m.StickerMessage != nil {
		d = m.StickerMessage
	} else if m.ExtendedTextMessage != nil && m.ExtendedTextMessage.ContextInfo != nil {
		q := m.ExtendedTextMessage.ContextInfo.QuotedMessage
		if q != nil {
			if q.ImageMessage != nil {
				d = q.ImageMessage
			} else if q.VideoMessage != nil {
				d = q.VideoMessage
			} else if q.StickerMessage != nil {
				d = q.StickerMessage
			}
		}
	}
	if d == nil {
		return nil, fmt.Errorf("no media")
	}
	return client.Download(context.Background(), d)
}

func uploadToCatbox(d []byte) string {
	b := new(bytes.Buffer)
	w := multipart.NewWriter(b)
	p, _ := w.CreateFormFile("fileToUpload", "f.jpg")
	p.Write(d)
	w.WriteField("reqtype", "fileupload")
	w.Close()
	r, _ := http.Post("https://catbox.moe/user/api.php", w.FormDataContentType(), b)
	res, _ := ioutil.ReadAll(r.Body)
	return string(res)
}