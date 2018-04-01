package telegram

import (
	"errors"
	"fmt"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/tools/tracker"
	"github.com/tokenme/tokenmed/utils/token"
	"gopkg.in/telegram-bot-api.v4"
	"regexp"
)

type Bot struct {
	Service     *common.Service
	Config      common.Config
	TelegramBot *tgbotapi.BotAPI
	UpdatesCh   tgbotapi.UpdatesChannel
	Tracker     *tracker.Tracker
}

func New(service *common.Service, config common.Config, trackerService *tracker.Tracker) (*Bot, error) {
	if !config.EnableTelegramBot {
		return nil, errors.New("telemebot disabled")
	}
	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	bot.Debug = config.Debug
	return &Bot{
		Service:     service,
		Config:      config,
		TelegramBot: bot,
		Tracker:     trackerService,
	}, nil
}

func (this *Bot) Start() error {
	if this.TelegramBot == nil {
		return errors.New("missing telegram bot")
	}
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates, err := this.TelegramBot.GetUpdatesChan(updateConfig)
	if err != nil {
		return err
	}
	this.UpdatesCh = updates
	log.Info("TelegramBot Started")
	go func(updates tgbotapi.UpdatesChannel) {
		for update := range updates {
			this.MessageConsumer(update.Message)
		}
	}(updates)
	return nil
}

func (this *Bot) Stop() {
	log.Info("TelegramBot Stopped")
	if this.TelegramBot == nil {
		return
	}
	this.UpdatesCh.Clear()
}

func (this *Bot) MessageConsumer(message *tgbotapi.Message) {
	if message == nil {
		return
	}

	if message.From == nil || message.Chat == nil {
		return
	}

	reg := regexp.MustCompile(fmt.Sprintf(`^/(\w+)@%s`, this.Config.TelegramBotName))
	matches := reg.FindStringSubmatch(message.Text)
	if len(matches) != 2 {
		return
	}
	cmd := matches[1]
	var reply string
	verifyCode, err := token.Decode(cmd)
	if err != nil {
		reply = "Sorry, submitted verification code is invalid"
	}
	db := this.Service.Db
	rows, _, err := db.Query(`SELECT c.status, c.promotion_id, c.adzone_id, c.channel_id, c.promoter_id, c.airdrop_id, a.telegram_group, c.wallet FROM tokenme.codes AS c LEFT JOIN tokenme.airdrops AS a ON (a.id=c.airdrop_id) WHERE c.id=%d LIMIT 1`, verifyCode)
	if err != nil {
		reply = "Sorry, we have some internal server bug :("
	}
	if len(rows) == 0 {
		reply = "Sorry, submitted verification code is invalid or expired"
	}
	codeStatus := rows[0].Uint(0)
	proto := common.PromotionProto{
		Id:        rows[0].Uint64(1),
		AdzoneId:  rows[0].Uint64(2),
		ChannelId: rows[0].Uint64(3),
		UserId:    rows[0].Uint64(4),
		AirdropId: rows[0].Uint64(5),
	}
	telegramGroupName := rows[0].Str(6)
	wallet := rows[0].Str(7)
	if telegramGroupName != message.Chat.Title {
		reply = fmt.Sprintf("Sorry, you must submit your code in group @%s", telegramGroupName)
	} else if codeStatus == 0 {
		reply = "Sorry, maybe you didn't submit your wallet address?"
	} else if codeStatus == 1 {
		_, ret, err := db.Query(`INSERT IGNORE INTO tokenme.airdrop_submissions (promotion_id, adzone_id, channel_id, promoter_id, airdrop_id, verify_code, wallet, telegram_msg_id, telegram_chat_id, telegram_user_id, telegram_chat_title, telegram_username, telegram_user_firstname, telegram_user_lastname) VALUES (%d, %d, %d, %d, %d, %d, '%s', %d, %d, %d, '%s', '%s', '%s', '%s')`, proto.Id, proto.AdzoneId, proto.ChannelId, proto.UserId, proto.AirdropId, verifyCode, db.Escape(wallet), message.MessageID, message.Chat.ID, message.From.ID, db.Escape(message.Chat.Title), db.Escape(message.From.UserName), db.Escape(message.From.FirstName), db.Escape(message.From.LastName))
		if err != nil {
			reply = "Sorry, we have some internal server bug :("
		}
		_, _, err = db.Query(`UPDATE tokenme.codes SET status=2 WHERE id=%d`, verifyCode)
		if err != nil {
			reply = "Sorry, we have some internal server bug :("
		} else if ret.AffectedRows() == 0 {
			reply = "Sorry, you already submitted in this airdrop and could not submit again"
		} else {
			reply = "Great! please wait for the airdrop transaction complete"
			this.Tracker.Promotion.Submission(proto)
		}
	} else if codeStatus == 2 {
		reply = "Your airdrop is in the pool, please wait for the transaction complete"
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, reply)
	msg.ReplyToMessageID = message.MessageID
	this.TelegramBot.Send(msg)
}
