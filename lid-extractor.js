const { Client } = require('pg');
const fs = require('fs');

async function startLidSystem() {
    console.log("\n" + "═".repeat(50));
    console.log("🚀 [LID SYSTEM] PostgreSQL کنکشن شروع کیا جا رہا ہے...");
    console.log("═".repeat(50));

    const dbConfig = {
        connectionString: process.env.DATABASE_URL,
        ssl: { rejectUnauthorized: false }
    };

    const client = new Client(dbConfig);

    try {
        // 1. لنک چیک کریں
        await client.connect();
        console.log("✅ [LINKED] پوسٹ گریس کے ساتھ لنک کامیابی سے ہو گیا ہے!");

        // 2. سیشن ٹیبل تلاش کریں
        console.log("⏳ [SEARCHING] ڈیٹا بیس سے سیشنز تلاش کر رہے ہیں...");
        const query = 'SELECT jid FROM whatsmeow_device;';
        const res = await client.query(query);

        if (res.rows.length === 0) {
            console.log("⚠️ [EMPTY] ڈیٹا بیس میں کوئی سیشن نہیں ملا۔ شاید بوٹ ابھی پیئر نہیں ہوا۔");
            process.exit(0);
        }

        console.log(`📂 [SESSION] کل ${res.rows.length} سیشنز مل گئے ہیں۔`);
        
        let botData = {};

        // 3. ڈیٹا نکالیں اور پرنٹ کریں
        console.log("\n" + "─".repeat(40));
        res.rows.forEach((row, index) => {
            const fullJid = row.jid;
            if (fullJid) {
                // نمبر اور آئی ڈی الگ کریں
                const parts = fullJid.split('@')[0].split(':');
                const number = parts[0];
                const identity = parts[0]; // آئی ڈی وہی نمبر یا LID ہوتا ہے

                console.log(`[BOT ${index + 1}]`);
                console.log(`📱 نمبر: ${number}`);
                console.log(`🆔 آئی ڈی: ${fullJid}`);
                console.log(`✨ اسٹیٹس: LID کامیابی سے نکال لی گئی ہے!`);
                console.log("─".repeat(40));

                // وہی پرانا اسٹرکچر
                botData[number] = {
                    phone: number,
                    lid: fullJid,
                    extractedAt: new Date().toISOString()
                };
            }
        });

        // 4. جیسن میں سیو کریں
        const finalJson = {
            timestamp: new Date().toISOString(),
            count: res.rows.length,
            bots: botData
        };

        fs.writeFileSync('./lid_data.json', JSON.stringify(finalJson, null, 2));
        
        console.log("\n✅ [SUCCESS] سارا ڈیٹا 'lid_data.json' میں سیو کر دیا گیا ہے۔");
        console.log("📁 فائل اسٹرکچر: وہی پرانا اسٹرکچر استعمال کیا گیا ہے۔");

    } catch (err) {
        console.error("\n❌ [ERROR] پوسٹ گریس کے ساتھ لنک فیل ہو گیا:");
        console.error(`   میج: ${err.message}`);
    } finally {
        await client.end();
        console.log("\n🏁 [FINISHED] ایکسٹریکٹر کا کام مکمل ہوا۔");
        console.log("═".repeat(50) + "\n");
        process.exit(0);
    }
}

startLidSystem();