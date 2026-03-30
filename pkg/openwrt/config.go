package openwrt

import "github.com/yukariin/external-dns-openwrt-webhook/pkg/lucirpc"

type Config struct {
	LuciRPC *lucirpc.Config `mapstructure:"lucirpc"`
}

func DefaultConfig() *Config {
	return &Config{
		LuciRPC: lucirpc.DefaultConfig(),
	}
}
