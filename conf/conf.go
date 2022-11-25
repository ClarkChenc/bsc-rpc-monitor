package conf

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	BscUrls           []string      `json:"bscUrls"`
	CheckInterval     time.Duration `json:"checkInterval"`
	RootChainContract string        `json:"rootChainContract"`
}

var (
	config Config
)

func GetConfig() *Config {
	return &config
}

func LoadConfig() *Config {
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("toml")   // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")      // optionally look for config in the working directory

	if viper.IsSet(ConfFileFlag) {
		confFile := viper.GetString(ConfFileFlag)
		file, err := os.Open(confFile) // For read access.
		if err != nil {
			panic(fmt.Errorf("fatal error open config file: %w", err))
		}
		err = file.Close()
		if err != nil {
			panic(fmt.Errorf("error close config file: %s\n", err))
		}
		viper.SetConfigFile(confFile)
	}

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		panic(fmt.Errorf("fatal Unmarshal config file: %w", err))
	}
	return &config
}
