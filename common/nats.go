package common

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// ConnectNATS establishes a connection to a NATS server using NKey authentication.
func ConnectNATS(url string, seed string) (*nats.Conn, error) {
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, fmt.Errorf("failed to parse NKey seed: %w", err)
	}

	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key from seed: %w", err)
	}

	nc, err := nats.Connect(
		url,
		nats.Nkey(pub, func(nonce []byte) ([]byte, error) {
			return kp.Sign(nonce)
		}),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectJitter(1*time.Second, 5*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				slog.Warn("NATS disconnected", slog.String("error", err.Error()))
			} else {
				slog.Warn("NATS disconnected")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", slog.String("url", nc.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	slog.Info("Connected to NATS", slog.String("url", nc.ConnectedUrl()))
	return nc, nil
}

// PublishJSON marshals data to JSON and publishes it to the given NATS subject.
// If useGzip is true, the payload is gzip-compressed before publishing.
func PublishJSON(nc *nats.Conn, subject string, data any, useGzip bool) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	payload := jsonBytes
	if useGzip {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(jsonBytes); err != nil {
			return fmt.Errorf("failed to gzip payload: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
		payload = buf.Bytes()
	}

	if err := nc.Publish(subject, payload); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	slog.Debug("Published message", slog.String("subject", subject), slog.Int("bytes", len(payload)))
	return nil
}

// ReverseFQDN reverses the parts of an FQDN for use in NATS subject hierarchies.
// e.g., "server1.example.com" → "com.example.server1"
func ReverseFQDN(fqdn string) string {
	parts := strings.Split(fqdn, ".")
	slices.Reverse(parts)
	return strings.Join(parts, ".")
}
