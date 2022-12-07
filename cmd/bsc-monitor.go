package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bttcprotocol/bsc-monitor/conf"
	"github.com/bttcprotocol/bsc-monitor/util"
	"github.com/maticnetwork/bor/common"
	"github.com/maticnetwork/bor/ethclient"
	"github.com/maticnetwork/bor/rpc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	RootCmd = &cobra.Command{
		Use:   "bttc-monitor",
		Short: "bttc-monitor a monitor for bttc",
		Run: func(cmd *cobra.Command, args []string) {
			conf.LoadConfig()
			BscMonitor()
		},
	}

	heightMap     map[string]uint64
	heightMapLock sync.RWMutex
)

func init() {
	RootCmd.PersistentFlags().StringP(conf.ConfFileFlag, "c", "config.toml", "config file ")
	err := viper.BindPFlag(conf.ConfFileFlag, RootCmd.PersistentFlags().Lookup(conf.ConfFileFlag))
	if err != nil {
		panic(err)
	}
	heightMap = make(map[string]uint64)
}

func getHeight(url string) (uint64, bool) {
	heightMapLock.RLock()
	defer heightMapLock.RUnlock()

	if height, ok := heightMap[url]; ok {
		return height, true
	}
	return 0, false
}

func setHeight(url string, height uint64) {
	heightMapLock.Lock()
	defer heightMapLock.Unlock()

	heightMap[url] = height
}

func BscMonitor() {
	checkInterval := conf.GetConfig().CheckInterval

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(1)
	isFirstTick := true
	ctx := context.Background()
	for {
		select {
		case <-ticker.C:
			if isFirstTick {
				isFirstTick = false
				ticker.Reset(checkInterval)
			}
			go checkBscUrls(ctx)

		case <-quit:
			fmt.Println("quit")
			ctx.Done()
			return
		}
	}
}

func checkBscUrl(ctx context.Context, targetUrl string, errorUrls *ErrorUrls) {
	u, _ := url.Parse(targetUrl)
	// test height growth
	bscRPCClient, err := rpc.Dial(u.String())
	if err != nil {
		errorUrls.lock.Lock()
		defer errorUrls.lock.Unlock()
		errorUrls.errorUrls = append(errorUrls.errorUrls, u.Host+"#rpc")
		return
	}

	bscChainClient := ethclient.NewClient(bscRPCClient)
	block, err := bscChainClient.BlockByNumber(ctx, nil)
	if err != nil {
		errorUrls.lock.Lock()
		defer errorUrls.lock.Unlock()
		errorUrls.errorUrls = append(errorUrls.errorUrls, u.Host+"#block")
		return
	}

	height := block.Number()
	if orgHeight, ok := getHeight(u.String()); ok {
		if height.Uint64() == orgHeight {
			errorUrls.lock.Lock()
			defer errorUrls.lock.Unlock()
			errorUrls.errorUrls = append(errorUrls.errorUrls, u.Host+"#height")
			return
		}
	}
	setHeight(u.String(), height.Uint64())

	// test call contract
	ci, err := util.NewRootchain(common.HexToAddress(conf.GetConfig().RootChainContract), bscChainClient)
	if err != nil {
		errorUrls.lock.Lock()
		defer errorUrls.lock.Unlock()
		errorUrls.errorUrls = append(errorUrls.errorUrls, u.Host+"#roochain")
		return
	}
	_, err = ci.GetLastChildBlock(nil)
	if err != nil {
		errorUrls.lock.Lock()
		defer errorUrls.lock.Unlock()
		errorUrls.errorUrls = append(errorUrls.errorUrls, u.Host+"#contract")
		return
	}
}

type ErrorUrls struct {
	errorUrls []string
	lock      sync.RWMutex
}

func TrimRright(str string, c string) string {
	lastIndex := strings.LastIndex(str, "#")
	if lastIndex == -1 {
		return str
	}
	return str[:lastIndex]
}

func checkBscUrls(ctx context.Context) {
	errorUrls := ErrorUrls{}

	config := conf.GetConfig()
	wg := sync.WaitGroup{}
	wg.Add(len(config.BscUrls))
	for _, url := range config.BscUrls {
		go func(url string) {
			defer wg.Done()
			checkBscUrl(ctx, url, &errorUrls)
		}(url)
	}
	wg.Wait()
	errorMap := make(map[string]string)
	for _, url := range errorUrls.errorUrls {
		host := TrimRright(url, "#")
		errorMap[host] = url
	}

	reportErrorUrl := []string{}
	for _, u := range conf.GetConfig().BscUrls {
		URL, _ := url.Parse(u)
		if _, ok := errorMap[URL.Host]; ok {
			reportErrorUrl = append(reportErrorUrl, errorMap[URL.Host])
		}
	}

	sort.Strings(errorUrls.errorUrls)
	fmt.Printf("%v error urls:%v\n", time.Now().Format("2006-01-02 15:04:05"), reportErrorUrl)
}
