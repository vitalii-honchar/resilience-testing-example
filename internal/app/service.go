package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	subjectDeviceTelemetry = "device.%d.telemetry"
	stopTimeout            = 30 * time.Second
)

type (
	Service struct {
		Nc     *nats.Conn
		server *http.Server
	}

	TelemetryRequest struct {
		DeviceId int    `json:"device_id"`
		Health   string `json:"health"`
		GpsLevel int    `json:"gps_level"`
	}

	TelemetryMessage struct {
		DeviceId int    `json:"device_id"`
		Health   string `json:"health"`
		GpsLevel int    `json:"gps_level"`
	}
)

func NewService(cfg *Config) (*Service, error) {
	nc, err := nats.Connect(cfg.NatsUrl, cfg.NatsOptions...)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/telemetry", publishTelemetryEndpoint(nc))

	return &Service{
		Nc:     nc,
		server: &http.Server{Handler: mux, Addr: fmt.Sprintf(":%d", cfg.Port)},
	}, nil
}

func (s *Service) Start() {
	go func() {
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("http server error: %v\n", err)
		}
	}()

	log.Printf("service started")
}

func (s *Service) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()

	err := s.server.Shutdown(ctx)
	if err != nil {
		log.Printf("error during shutdown http server: %v\n", err)
	}

	err = s.Nc.Drain()
	if err != nil {
		log.Printf("error durign drain nats connection: %v\n", err)
	}
}

func publishTelemetryEndpoint(nc *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var req TelemetryRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Println("can't decode user request")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		msg := &TelemetryMessage{DeviceId: req.DeviceId, Health: req.Health, GpsLevel: req.GpsLevel}
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Println("can't encode nats message")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		subject := fmt.Sprintf(subjectDeviceTelemetry, msg.DeviceId)
		err = nc.Publish(subject, msgBytes)
		if err != nil {
			log.Println("can't publish nats message")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Println("successfully handled send telemetry")
	}
}
