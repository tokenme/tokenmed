package main

import (
	"flag"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	"github.com/fvbock/endless"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/configor"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/router"
	"github.com/tokenme/tokenmed/tools/airdrop"
	"github.com/tokenme/tokenmed/tools/gc"
	"github.com/tokenme/tokenmed/tools/redpacket"
	"github.com/tokenme/tokenmed/tools/telegram"
	"github.com/tokenme/tokenmed/tools/tracker"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var (
		config     common.Config
		configFlag common.Config
		configPath string
	)

	os.Setenv("CONFIGOR_ENV_PREFIX", "-")

	flag.StringVar(&configPath, "config", "config.toml", "configuration file")
	flag.IntVar(&configFlag.Port, "port", 11151, "set port")
	flag.StringVar(&configFlag.UI, "ui", "./ui/dist", "set web static file path")
	flag.StringVar(&configFlag.LogPath, "log", "/tmp/tokenmed", "set log file path without filename")
	flag.BoolVar(&configFlag.Debug, "debug", false, "set debug mode")
	flag.BoolVar(&configFlag.EnableWeb, "web", false, "enable http web server")
	flag.BoolVar(&configFlag.EnableTelegramBot, "telegrambot", false, "enable telegram bot")
	flag.BoolVar(&configFlag.EnableGC, "gc", false, "enable gc")
	flag.BoolVar(&configFlag.EnableDealer, "dealer", false, "enable dealer")
	flag.BoolVar(&configFlag.EnableDepositChecker, "deposit", false, "enable deposit checker")
	flag.Parse()

	configor.New(&configor.Config{Verbose: configFlag.Debug, ErrorOnUnmatchedKeys: true, Environment: "production"}).Load(&config, configPath)

	if configFlag.Port > 0 {
		config.Port = configFlag.Port
	}
	if configFlag.UI != "" {
		config.UI = configFlag.UI
	}
	if configFlag.LogPath != "" {
		config.LogPath = configFlag.LogPath
	}

	if configFlag.EnableWeb {
		config.EnableWeb = configFlag.EnableWeb
	}

	if configFlag.EnableTelegramBot {
		config.EnableTelegramBot = configFlag.EnableTelegramBot
	}

	if configFlag.EnableGC {
		config.EnableGC = configFlag.EnableGC
	}

	if configFlag.EnableDealer {
		config.EnableDealer = configFlag.EnableDealer
	}

	if configFlag.EnableDepositChecker {
		config.EnableDepositChecker = configFlag.EnableDepositChecker
	}

	if configFlag.Debug {
		config.Debug = configFlag.Debug
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Error(err.Error())
		return
	}

	var logPath string
	if path.IsAbs(config.LogPath) {
		logPath = config.LogPath
	} else {
		logPath = path.Join(wd, config.LogPath)
	}
	defer log.Uninit(log.InitMultiFileAndConsole(logPath, "tokenmed.log", log.LvERROR))

	raven.SetDSN(config.SentryDSN)
	service := common.NewService(config)
	defer service.Close()
	service.Db.Reconnect()

	trackerService := tracker.New()
	promotionTracker := tracker.NewPromotionLogService(service)
	trackerService.SetPromotion(promotionTracker)
	go trackerService.Start()

	telegramBot, err := telegram.New(service, config, trackerService)
	if err == nil {
		err = telegramBot.Start()
		if err != nil {
			log.Error(err.Error())
		}
	}

	gcHandler := gc.New(service, config)
	if config.EnableGC {
		go gcHandler.Start()
	}

	dealerContractDeployer := airdrop.NewDealerContractDeployer(service, config)
	allowanceChecker := airdrop.NewAllowanceChecker(service, config)
	airdropper := airdrop.NewAirdropper(service, config)
	airdropChecker := airdrop.NewAirdropChecker(service, config, trackerService)
	depositChecker := redpacket.NewDepositChecker(service, config)
	if config.EnableDealer {
		go dealerContractDeployer.Start()
		go allowanceChecker.Start()
		go airdropper.Start()
		go airdropChecker.Start()
	}

	if config.EnableDepositChecker {
		go depositChecker.Start()
	}

	if config.EnableWeb {
		handler.InitHandler(service, config, trackerService)
		if config.Debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		//gin.DisableBindValidation()
		var staticPath string
		if path.IsAbs(config.UI) {
			staticPath = config.UI
		} else {
			staticPath = path.Join(wd, config.UI)
		}
		log.Info("Static UI path: %s", staticPath)
		r := router.NewRouter(staticPath)
		log.Info("%s started at:0.0.0.0:%d", config.AppName, config.Port)
		defer log.Info("%s exit from:0.0.0.0:%d", config.AppName, config.Port)
		endless.ListenAndServe(fmt.Sprintf(":%d", config.Port), r)
	} else {
		exitChan := make(chan struct{}, 1)
		go func() {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGINT, syscall.SIGKILL, syscall.SIGQUIT, syscall.SIGSTOP, syscall.SIGTERM)
			<-ch
			exitChan <- struct{}{}
			close(ch)
		}()
		<-exitChan
	}
	trackerService.Stop()
	if telegramBot != nil {
		telegramBot.Stop()
	}
	dealerContractDeployer.Stop()
	allowanceChecker.Stop()
	airdropper.Stop()
	airdropChecker.Stop()
	gcHandler.Stop()
	depositChecker.Stop()
	trackerService.Flush()
}
