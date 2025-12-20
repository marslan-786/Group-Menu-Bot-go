FROM golang:1.24-alpine AS builder

# ضروری ٹولز انسٹال کریں
RUN apk add --no-cache gcc musl-dev git sqlite-dev ffmpeg-dev

WORKDIR /app

# سارا کوڈ کاپی کریں (یہ اہم ہے)
COPY . .

# پرانی فائلز ہٹا کر نیا ماڈیول شروع کریں (تاکہ کوئی پرانا کیش تنگ نہ کرے)
RUN rm -f go.mod go.sum || true
RUN go mod init impossible-bot

# تمام ضروری لائبریریز ڈاؤن لوڈ کریں
# 1. WhatsApp Library
RUN go get go.mau.fi/whatsmeow@latest
# 2. MongoDB Libraries
RUN go get go.mongodb.org/mongo-driver/mongo@latest
RUN go get go.mongodb.org/mongo-driver/bson@latest
# 3. Web Framework & Database Drivers
RUN go get github.com/gin-gonic/gin@latest
RUN go get github.com/mattn/go-sqlite3@latest
RUN go get github.com/lib/pq@latest
# 4. WebSocket Library (For Realtime UI)
RUN go get github.com/gorilla/websocket@latest

# موڈ ٹائیڈی کریں
RUN go mod tidy

# ایپ بلڈ کریں
RUN go build -o bot .

# -------------------
# فائنل سٹیج
# -------------------
FROM alpine:latest

# رن ٹائم کے پیکجز
RUN apk add --no-cache ca-certificates sqlite-libs ffmpeg

WORKDIR /app

# بلڈر سے فائلز کاپی کریں
COPY --from=builder /app/bot .
COPY --from=builder /app/web ./web

# اگر pic.png روٹ میں ہے تو اسے بھی کاپی کریں (ورنہ ایرر نہیں دے گا)
COPY --from=builder /app/pic.png ./pic.png || true 

# انوائرمنٹ اور پورٹ
ENV PORT=8080
EXPOSE 8080

CMD ["./bot"]