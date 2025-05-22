package logger

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
)

const (
	PlatformLogSubject = "$PC.%s.Logs"
)

// Logger returns an slog.Logger configured to write output
// as nats messages over the standard subject on the
// provided nats connection
func Logger(nc *nats.Conn, cType string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(&NatsLogger{nc: nc, OutTopic: fmt.Sprintf(PlatformLogSubject, cType)}, nil))
}

type NatsLogger struct {
	nc       *nats.Conn
	OutTopic string
}

func NewNatsLogger(nc *nats.Conn, topic string) *NatsLogger {
	return &NatsLogger{
		OutTopic: topic,
		nc:       nc,
	}
}

func (nl *NatsLogger) Write(p []byte) (int, error) {
	t := strings.TrimSpace(string(p))
	err := nl.nc.Publish(nl.OutTopic, []byte(t))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
