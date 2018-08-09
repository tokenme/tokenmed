package common

type Config struct {
	AppName                      string      `default:"tokenmed"`
	BaseUrl                      string      `default:"https://tokenmama.io"`
	CDNUrl                       string      `default:"https://static.tianxi100.com/"`
	RedPacketWechatShareLink     string      `default:"https://tmm.tianxi100.com/rp.html#/show/"`
	RedPacketShareLink           string      `default:"https://tokenmama.io/rp.html#/show/"`
	Port                         int         `default:"11151"`
	UI                           string      `default:"./ui/dist"`
	LogPath                      string      `default:"/tmp/tokenme"`
	Debug                        bool        `default:"false"`
	MySQL                        MySQLConfig `required:"true"`
	Redis                        RedisConfig
	Geth                         string `default:"https://mainnet.infura.io/NlT37dDxuLT2tlZNw3It"`
	EthplorerKey                 string `default:"freekey"`
	SlackToken                   string `required:"true"`
	SlackAdminChannelID          string `default:"G9Y7METUG"`
	GeoIP                        string `required:"true"`
	TokenSalt                    string `required:"true"`
	LinkSalt                     string `required:"true"`
	OutputKey                    string `required:"true"`
	OutputSalt                   string `required:"true"`
	TwilioToken                  string `required:"true"`
	TelegramBotToken             string `required:"true"`
	TelegramBotName              string `required:"true"`
	WXAppId                      string `required:"true"`
	WXSecret                     string `required:"true"`
    WXMPAppId                    string `required:"true"`
    WXMPSecret                   string `required:"true"`
    WXMPToken                    string `required:"true"`
    WXMPEncodingAESKey           string `required:"true"`
	SentryDSN                    string `default:"https://b7c6f2e4200a444c99f6b92aca5c372c:849b8bfea55c4d4cbca578ec68a861bb@sentry.io/994357"`
	AirdropCommissionFee         uint64 `default:"4"`
	RedPacketCommissionFee       uint64 `default:"50"`
	DealerIncomeWallet           string `required:"true"`
	DealerContractCreateGasPrice int64  `default:"8"`
	DealerContractCreateGasLimit uint64 `default:"210000"`
	RedPacketIncomeWallet        string `required:"true"`
	RedPacketOutputWallet        string `required:"true"`
	RedPacketGasPrice            uint64 `default:"8", required:"true"`
	RedPacketGasLimit            uint64 `default:"210000", required:"true"`
	CheckoutFee                  uint64 `default:"100000", required:"true"`
	EnableWeb                    bool
	EnableTelegramBot            bool
	EnableGC                     bool
	EnableDealer                 bool
	EnableDepositChecker         bool
	Mail                         MailConfig
}

type MySQLConfig struct {
	Host   string `required:"true"`
	User   string `required:"true"`
	Passwd string `required:"true"`
	DB     string `default:"tokenme"`
}

type RedisConfig struct {
	Master string `required:"true"`
	Slave  string
}

type MailConfig struct {
	FromAddr          string `default:"tokenme@xibao100.com"`
	FromName          string `default:"Tokenme.IO"`
	Server            string `default:"localhost"`
	Port              int    `default:"25"`
	Passwd            string
	ActivationSubject string `default:"Tokenme.io activation mail"`
	ActivationBodyTpl string `default:"<a href='https://%s/user/activate?email=%s&activation_code=%s'>Activate link (Expired in 2 hours)</a>"`
}
