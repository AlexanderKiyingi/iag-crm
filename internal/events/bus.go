package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

const (
	SpecVersion = "1.0"
	Source      = "iag.crm"
	TopicCommercial = "iag.commercial"

	TypeDealUpdated    = "crm.deal.updated"
	TypeDealWon        = "crm.deal.won"
	TypeLeadConverted  = "crm.lead.converted"
	TypeBridgeSynced   = "crm.bridge.synced"
	TypeTicketCreated  = "crm.ticket.created"
	TypeTierRulesSaved = "crm.loyalty.tier_rules.saved"
)

type Bus struct {
	writer  *kafka.Writer
	enabled bool
}

type Config struct {
	Brokers []string
	Enabled bool
}

func NewFromEnv() *Bus {
	return New(Config{
		Brokers: ParseBrokers(os.Getenv("KAFKA_BROKERS")),
		Enabled: strings.EqualFold(os.Getenv("EVENT_BUS_ENABLED"), "true"),
	})
}

func New(cfg Config) *Bus {
	if !cfg.Enabled || len(cfg.Brokers) == 0 {
		return &Bus{enabled: false}
	}
	return &Bus{
		enabled: true,
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Topic:        TopicCommercial,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			Transport:    &kafka.Transport{ClientID: Source},
		},
	}
}

func (b *Bus) Close() error {
	if b == nil || !b.enabled || b.writer == nil {
		return nil
	}
	return b.writer.Close()
}

func (b *Bus) Enabled() bool { return b != nil && b.enabled }

type PlatformEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Time        string         `json:"time"`
	Source      string         `json:"source"`
	SpecVersion string         `json:"specversion"`
	Data        map[string]any `json:"data"`
}

func (b *Bus) PublishCommercial(ctx context.Context, eventType string, data map[string]any, key string) {
	if !b.enabled || b.writer == nil {
		return
	}
	evt := PlatformEvent{
		ID:          uuid.NewString(),
		Type:        eventType,
		Time:        time.Now().UTC().Format(time.RFC3339Nano),
		Source:      Source,
		SpecVersion: SpecVersion,
		Data:        data,
	}
	body, err := json.Marshal(evt)
	if err != nil {
		slog.Warn("crm event marshal failed", "type", eventType, "err", err)
		return
	}
	if key == "" {
		key = evt.ID
	}
	if err := b.writer.WriteMessages(ctx, kafka.Message{
		Topic: TopicCommercial,
		Key:   []byte(key),
		Value: body,
		Headers: []kafka.Header{
			{Key: "ce-type", Value: []byte(eventType)},
			{Key: "ce-source", Value: []byte(Source)},
		},
	}); err != nil {
		slog.Warn("crm event publish failed", "type", eventType, "err", err)
	}
}

func ParseBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
