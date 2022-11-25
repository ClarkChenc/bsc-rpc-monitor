package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

	heightMap map[string]uint64
)

func init() {
	RootCmd.PersistentFlags().StringP(conf.ConfFileFlag, "c", "config.toml", "config file ")
	err := viper.BindPFlag(conf.ConfFileFlag, RootCmd.PersistentFlags().Lookup(conf.ConfFileFlag))
	if err != nil {
		panic(err)
	}
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

func checkBscUrl(ctx context.Context, url string, errorUrls []string) {
	// test height growth
	bscRPCClient, err := rpc.Dial(url)
	if err != nil {
		errorUrls = append(errorUrls, url)
		return
	}

	bscChainClient := ethclient.NewClient(bscRPCClient)
	block, err := bscChainClient.BlockByNumber(ctx, nil)
	if err != nil {
		errorUrls = append(errorUrls, url)
	}
	height := block.NumberU64()
	if orgHeight, ok := heightMap[url]; ok {
		if height == orgHeight {
			errorUrls = append(errorUrls, url)
			return
		}
	}
	heightMap[url] = height

	// test call contract
	ci, err := util.NewRootchain(common.HexToAddress(conf.GetConfig().RootChainContract), bscChainClient)
	if err != nil {
		errorUrls = append(errorUrls, url)
		return
	}
	_, err = ci.GetLastChildBlock(nil)
	if err != nil {
		errorUrls = append(errorUrls, url)
		return
	}
}

func checkBscUrls(ctx context.Context) {
	errorUrls := make([]string, 0)

	config := conf.GetConfig()
	wg := sync.WaitGroup{}
	wg.Add(len(config.BscUrls))
	for _, url := range config.BscUrls {
		go func(url string) {
			defer wg.Done()
			checkBscUrl(ctx, url, errorUrls)
		}(url)
	}
	wg.Wait()
	fmt.Println("error urls:", errorUrls)
}
