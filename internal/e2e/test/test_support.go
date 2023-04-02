package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"toxiproxy-poc/internal/app"

	toxiproxy "github.com/Shopify/toxiproxy/v2/client"
	"github.com/nats-io/nats.go"
)

const natsProxyPort = 24222

type Stage struct {
	nc  *nats.Conn
	cfg *app.Config
	svc *app.Service
	t   *testing.T

	natsProxy *toxiproxy.Proxy

	expected   *expected
	httpActual *httpActual
	natsActual *natsActual
}

type expected struct {
	deviceId int
	gpsLevel int
}

type httpActual struct {
	statusCode int
	err        error
}

type natsActual struct {
	msg map[string]any
}

var random = rand.New(rand.NewSource(time.Now().UnixMilli()))

func NewStage(t *testing.T) *Stage {
	return newStage(t)
}

func NewStageWith3NatsReconnect(t *testing.T) *Stage {
	return newStage(t, func(o *nats.Options) error {
		o.MaxReconnect = 3
		return nil
	})
}

func newStage(t *testing.T, natsOptions ...nats.Option) *Stage {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Fatal(err)
	}

	natsProxy := getOrCreateNATSProxy()
	err = natsProxy.Enable()
	if err != nil {
		t.Fatal(err)
	}

	cfg := app.NewConfig(fmt.Sprintf("nats://127.0.0.1:%d", natsProxyPort), 9000+random.Intn(2000))
	cfg.NatsOptions = natsOptions
	svc, err := app.NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}

	svc.Start()

	t.Cleanup(func() {
		natsProxy.Delete()
		svc.Stop()
		nc.Drain()
	})

	stage := &Stage{nc: nc, cfg: cfg, svc: svc, t: t, natsProxy: natsProxy}

	nc.Subscribe("device.*.telemetry", func(msg *nats.Msg) {
		var body map[string]any
		if err := json.Unmarshal(msg.Data, &body); err != nil {
			stage.t.Fatal(err)
		}
		stage.natsActual = &natsActual{msg: body}
	})

	return stage
}

func (s *Stage) SendTelemetryRequest() *Stage {
	s.expected = &expected{
		deviceId: random.Intn(1000),
		gpsLevel: random.Intn(1000),
	}

	body := map[string]any{
		"device_id": s.expected.deviceId,
		"health":    "HEALTHTY",
		"gps_level": s.expected.gpsLevel,
	}

	req, err := json.Marshal(body)
	if err != nil {
		s.t.Fatal(err)
	}

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/telemetry", s.cfg.Port), "application/json", bytes.NewBuffer(req))

	s.httpActual = &httpActual{err: err}

	if resp != nil {
		s.httpActual.statusCode = resp.StatusCode
	}
	return s
}

func (s *Stage) TheResponseIsSuccessful() *Stage {
	if s.httpActual.err != nil {
		s.t.Fatalf("error should be nil, but it is %v", s.httpActual.err)
	}
	if s.httpActual.statusCode != http.StatusOK {
		s.t.Errorf("status code should be 200 OK, but it is %d", s.httpActual.statusCode)
	}
	return s
}

func (s *Stage) TheResponseIsInternalServerError() *Stage {
	if s.httpActual.err != nil {
		s.t.Fatalf("error should be nil, but it is %v", s.httpActual.err)
	}
	if s.httpActual.statusCode != http.StatusInternalServerError {
		s.t.Errorf("status code should be 500, but it is %d", s.httpActual.statusCode)
	}
	return s
}

func (s *Stage) NatsIsUnavailable() *Stage {
	err := s.natsProxy.Disable()
	if err != nil {
		s.t.Fatal(err)
	}
	return s
}

func (s *Stage) RestartService() *Stage {
	s.svc.Stop()

	svc, err := app.NewService(s.cfg)
	if err != nil {
		s.t.Fatal(err)
	}
	s.svc = svc
	s.svc.Start()
	return s
}

func (s *Stage) NatsIsAvailable() *Stage {
	err := s.natsProxy.Enable()
	if err != nil {
		s.t.Fatal(err)
	}
	return s
}

func (s *Stage) After5Seconds() *Stage {
	time.Sleep(5 * time.Second)
	return s
}

func (s *Stage) After10Seconds() *Stage {
	time.Sleep(10 * time.Second)
	return s
}

func (s *Stage) TheNatsMessageWasPublished() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		if s.natsActual != nil {
			s.verifyNatsMessage()
			return
		}

		select {
		case <-ctx.Done():
			s.t.Error("no nats message receive")
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Stage) TheNatsMessageWasNotPublished() *Stage {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		if s.natsActual != nil {
			s.t.Error("received nats message")
			return s
		}

		select {
		case <-ctx.Done():
			return s
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Stage) verifyNatsMessage() {
	deviceId, ok := s.natsActual.msg["device_id"]
	if !ok {
		s.t.Fatal("device id is not present")
	}
	if int(deviceId.(float64)) != s.expected.deviceId {
		s.t.Errorf("device id should be %d, but it is %v", s.expected.deviceId, deviceId)
	}

	gpsLevel, ok := s.natsActual.msg["gps_level"]
	if !ok {
		s.t.Fatal("gps level is not present")
	}

	if int(gpsLevel.(float64)) != s.expected.gpsLevel {
		s.t.Errorf("gps level should be %d, but it is %v", s.expected.gpsLevel, gpsLevel)
	}
}

func getOrCreateNATSProxy() *toxiproxy.Proxy {
	toxiClient := toxiproxy.NewClient("http://localhost:8474")

	p, err := toxiClient.Proxy("nats")
	if err == nil {
		return p
	}
	p, err = toxiClient.CreateProxy("nats", fmt.Sprintf("0.0.0.0:%d", natsProxyPort), "nats:4222")
	if err != nil {
		panic(err)
	}
	return p
}
