package main

import (
	"flag"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/configor"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/router"
	"github.com/tokenme/tokenmed/tools/airdrop"
	"github.com/tokenme/tokenmed/tools/gc"
	"github.com/tokenme/tokenmed/tools/telegram"
	"github.com/tokenme/tokenmed/tools/tracker"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
)

var (
	configFlag = flag.String("config", "config.yml", "configuration file")
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var config common.Config

	flag.IntVar(&config.Port, "port", 11151, "set port")
	flag.StringVar(&config.UI, "ui", "./ui/dist", "set web static file path")
	flag.StringVar(&config.LogPath, "log", "/tmp/tokenmed", "set log file path without filename")
	flag.BoolVar(&config.Debug, "debug", false, "set debug mode")
	flag.BoolVar(&config.EnableWeb, "web", false, "enable http web server")
	flag.BoolVar(&config.EnableTelegramBot, "telegrambot", false, "enable telegram bot")
	flag.BoolVar(&config.EnableGC, "gc", false, "enable gc")
	flag.BoolVar(&config.EnableDealer, "dealer", false, "enable dealer")
	flag.Parse()

	os.Setenv("CONFIGOR_ENV_PREFIX", "-")
	configor.Load(&config, *configFlag)

	wd, err := os.Getwd()
	if err != nil {
		log.Error(err.Error())
		return
	}

	defer log.Uninit(log.InitMultiFileAndConsole(path.Join(wd, config.LogPath), "tokenmed.log", log.LvERROR))

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
	if config.EnableDealer {
		go dealerContractDeployer.Start()
		go allowanceChecker.Start()
		go airdropper.Start()
		go airdropChecker.Start()
	}

	if config.EnableWeb {
		handler.InitHandler(service, config, trackerService)
		if config.Debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		//gin.DisableBindValidation()
		staticPath := path.Join(wd, config.UI)
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
	trackerService.Flush()
}
