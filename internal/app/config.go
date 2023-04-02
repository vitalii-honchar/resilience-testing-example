package app

import "github.com/nats-io/nats.go"

type Config struct {
	NatsUrl     string
	NatsOptions []nats.Option
	Port        int
}

func NewConfig(natsUrl string, port int) *Config {
	return &Config{NatsUrl: natsUrl, Port: port}
}
