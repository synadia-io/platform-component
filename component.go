package platform_component

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"

	"github.com/ConnectEverything/platform-component/pkg/keys"
)

const (
	DefaultURL        = "https://cloud.synadia.com"
	ConnectPath       = "api/core/beta/platform-components/connect"
	HeartbeatSubject  = "$PC.heartbeat"
	HeartbeatInterval = 2000 * time.Millisecond
)

type HeartbeatFunction func() string

type ConnectRequest struct {
	NKey string          `json:"nkey_public"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ConnectResponse struct {
	JWT     string          `json:"jwt"`
	Account string          `json:"account"`
	Server  string          `json:"server"`
	Config  json.RawMessage `json:"config"`
}

type Componenter interface {
	// Register the platform component with control plane and retrieve connection configuration
	// and credentials. Data is any additional component specific config that needs to be sent
	// to the server on registration, and config is the expected returned configuration response
	// from control plane
	Register(token string, opts ...RegisterOption) error

	// Start the platform component. Connects to nats and starts the heartbeat and logging services
	Start(ctx context.Context) error

	// Stop the platform component. Will drain and close the nats connection
	Stop() error

	// NatsConnection for the platform component
	NatsConnection() *nats.Conn

	// The decoded platform connect response
	Config() *ConnectResponse
}

func Component(cType string, logger *slog.Logger) Componenter {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	return &base{cType: cType, logger: logger}
}

type base struct {
	cType   string
	logger  *slog.Logger
	userKey *keys.Key
	cr      *ConnectResponse
	nc      *nats.Conn
	cancel  context.CancelFunc
	hf      HeartbeatFunction
}

func (b *base) Register(token string, opts ...RegisterOption) error {
	k, err := keys.KeyFor(nkeys.PrefixByteUser)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	b.userKey = k

	ro := &RegisterOptions{Token: token}
	for _, o := range opts {
		err = o(ro)
		if err != nil {
			return err
		}
	}

	if ro.URL == "" {
		ro.URL = DefaultURL
	}

	if ro.Heartbeat != nil {
		b.hf = ro.Heartbeat
	}

	creq := &ConnectRequest{NKey: k.Public, Data: ro.Data}
	crb, err := json.Marshal(creq)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}
	buf := bytes.NewBuffer(crb)

	b.logger.Info("connecting to platform", "server", ro.URL, "user", k.Public)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", ro.URL, ConnectPath), buf)
	if err != nil {
		return fmt.Errorf("create server request failed: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %s", token))
	req.Header.Add("Content-Type", "application/json")

	c := http.DefaultClient
	c.Timeout = 2 * time.Second

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	sc, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", resp.Status, string(sc))
	}

	cr := &ConnectResponse{}
	err = json.Unmarshal(sc, cr)
	if err != nil {
		return fmt.Errorf("unmarshal response failed: %w", err)
	}

	b.logger.Info("register request success!")

	if cr.Config != nil {
		if ro.Config == nil {
			b.logger.Warn("control plane returned config data but no destination supplied")
		} else {
			err = json.Unmarshal(cr.Config, ro.Config)
			if err != nil {
				return fmt.Errorf("unmarshal of config failed: %w", err)
			}
		}
	}

	b.cr = cr

	return nil
}

func (b *base) Start(ctx context.Context) error {
	if b.nc != nil && b.nc.IsConnected() {
		b.logger.Warn("platform component already connected")
	}

	b.logger.Info("connecting to nats", "server", b.cr.Server)

	nc, err := nats.Connect(b.cr.Server, nats.Name(fmt.Sprintf("platform component %s", b.cType)), nats.UserJWTAndSeed(b.cr.JWT, string(b.userKey.Seed)))
	if err != nil {
		return err
	}

	b.logger.Info("connected")
	b.nc = nc

	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	go b.heartbeat(ctx)

	return nil
}

func (b *base) Stop() error {
	b.logger.Info("stopping platform component")
	if b.cancel != nil {
		b.cancel()
	}

	closed := make(chan struct{})
	b.nc.Opts.ClosedCB = func(c *nats.Conn) {
		close(closed)
	}

	b.nc.Drain()

	select {
	case <-closed:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("timeout waiting for nats connection to drain and close")
	}
}

func (b *base) NatsConnection() *nats.Conn {
	return b.nc
}

func (b *base) Config() *ConnectResponse {
	return b.cr
}

func (b *base) heartbeat(ctx context.Context) {
	b.logger.Info("starting heartbeat")
	defer b.logger.Info("heartbeat stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(HeartbeatInterval):
			var data []byte
			if b.hf != nil {
				data = []byte(fmt.Sprintf(`{"msg":"%s", "id": "%s", "data": "%s"}`, "ping", b.userKey.Public, b.hf()))
			} else {
				data = []byte(fmt.Sprintf(`{"msg":"%s", "id": "%s"}`, "ping", b.userKey.Public))
			}
			_ = b.nc.Publish(fmt.Sprintf("%s.%s", HeartbeatSubject, b.cType), data)
		}
	}
}

type RegisterOptions struct {
	URL       string
	Token     string
	Data      json.RawMessage
	Config    interface{}
	Heartbeat HeartbeatFunction
}

type RegisterOption func(*RegisterOptions) error

// WithURL of the target Synadia Control Plane Server
func WithURL(url string) RegisterOption {
	return func(ro *RegisterOptions) error {
		ro.URL = url
		return nil
	}
}

// WithData to be included with the registration request. Some
// components require specific payloads when registering
func WithData(data interface{}) RegisterOption {
	return func(ro *RegisterOptions) error {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal request data: %s", err)
		}

		ro.Data = b
		return nil
	}
}

// WithConfig expected in the the registration reply. Config
// returned from the server will be unmarshalled into conf
func WithConfig(conf interface{}) RegisterOption {
	return func(ro *RegisterOptions) error {
		ro.Config = conf
		return nil
	}
}

// WithHeartbeatFunc that will be called ever HeartbeatInterval.
// returned string will be included in the `data` portion of the
// heartbeat ping message
func WithHearbeatFunc(f HeartbeatFunction) RegisterOption {
	return func(ro *RegisterOptions) error {
		ro.Heartbeat = f
		return nil
	}
}
