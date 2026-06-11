package events

import (
	"context"
	"encoding/json"
	"fmt"
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

	TypeDealUpdated     = "crm.deal.updated"
	TypeDealWon         = "crm.deal.won"
	TypeLeadConverted   = "crm.lead.converted"
	TypeBridgeSynced    = "crm.bridge.synced"
	TypeTicketCreated   = "crm.ticket.created"
	TypeTierRulesSaved  = "crm.loyalty.tier_rules.saved"
	TypeQuoteSent       = "crm.quote.sent"
	TypeQuoteSigned     = "crm.quote.signed"
	TypeAccountCreated  = "crm.account.created"
	TypeContactCreated  = "crm.contact.created"
	TypeCampaignLaunched = "crm.campaign.launched"
	TypeJourneyEnrolled  = "crm.journey.enrolled"
	TypeJourneyCompleted = "crm.journey.completed"
	TypeAlertRaised      = "crm.alert.raised"
)

type Bus struct {
	writer  *kafka.Writer
	enabled bool
	store   outboxEnqueuer
}

type outboxEnqueuer interface {
	Enqueue(ctx context.Context, eventType, key string, payload any) error
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

func (b *Bus) SetOutbox(store outboxEnqueuer) {
	if b == nil {
		return
	}
	b.store = store
}

func (b *Bus) UsesOutbox() bool { return b != nil && b.store != nil }

type PlatformEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Time        string         `json:"time"`
	Source      string         `json:"source"`
	SpecVersion string         `json:"specversion"`
	Data        map[string]any `json:"data"`
}

func finalizeEvent(evt PlatformEvent) PlatformEvent {
	if evt.ID == "" {
		evt.ID = uuid.NewString()
	}
	if evt.Time == "" {
		evt.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if evt.Source == "" {
		evt.Source = Source
	}
	if evt.SpecVersion == "" {
		evt.SpecVersion = SpecVersion
	}
	return evt
}

func eventKey(explicit string, evt PlatformEvent) string {
	if explicit != "" {
		return explicit
	}
	return evt.ID
}

func (b *Bus) publish(ctx context.Context, evt PlatformEvent, key string) error {
	if !b.enabled || b.writer == nil {
		return nil
	}
	evt = finalizeEvent(evt)
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return b.writer.WriteMessages(ctx, kafka.Message{
		Topic: TopicCommercial,
		Key:   []byte(eventKey(key, evt)),
		Value: body,
		Headers: []kafka.Header{
			{Key: "ce-type", Value: []byte(evt.Type)},
			{Key: "ce-source", Value: []byte(evt.Source)},
		},
	})
}

// DispatchOutbox writes a pre-finalized outbox row to Kafka.
func (b *Bus) DispatchOutbox(ctx context.Context, eventType string, eventKey string, payload []byte) error {
	if !b.enabled || b.writer == nil {
		return nil
	}
	var evt PlatformEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("decode outbox payload: %w", err)
	}
	if evt.Type == "" {
		evt.Type = eventType
	}
	evt = finalizeEvent(evt)
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	key := eventKey
	if key == "" {
		key = evt.ID
	}
	return b.writer.WriteMessages(ctx, kafka.Message{
		Topic: TopicCommercial,
		Key:   []byte(key),
		Value: body,
		Headers: []kafka.Header{
			{Key: "ce-type", Value: []byte(evt.Type)},
			{Key: "ce-source", Value: []byte(evt.Source)},
		},
	})
}

func (b *Bus) PublishCommercial(ctx context.Context, eventType string, data map[string]any, key string) {
	if !b.enabled {
		return
	}
	evt := finalizeEvent(PlatformEvent{Type: eventType, Data: data})
	if b.store != nil {
		if err := b.store.Enqueue(ctx, eventType, eventKey(key, evt), evt); err != nil {
			slog.Warn("crm event enqueue failed", "type", eventType, "err", err)
		}
		return
	}
	if err := b.publish(ctx, evt, key); err != nil {
		slog.Warn("crm event publish failed", "type", eventType, "err", err)
	}
}

// PublishAlert emits crm.alert.raised for iag-notifications policy consumers,
// using the shared {channel,recipient,templateId,variables} envelope.
func (b *Bus) PublishAlert(ctx context.Context, channel, recipient, templateID string, variables map[string]string, key string) {
	if !b.enabled || recipient == "" || templateID == "" {
		return
	}
	vars := map[string]any{}
	for k, v := range variables {
		vars[k] = v
	}
	if channel == "" {
		channel = defaultNotifyChannel()
	}
	b.PublishCommercial(ctx, TypeAlertRaised, map[string]any{
		"channel":    channel,
		"recipient":  recipient,
		"templateId": templateID,
		"variables":  vars,
	}, key)
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

func defaultNotifyChannel() string {
	if ch := strings.TrimSpace(os.Getenv("NOTIFY_CHANNEL")); ch != "" {
		return ch
	}
	return "email"
}

// DefaultNotifyRecipient is the fallback recipient (e.g. the support/sales
// desk) used when an alert has no specific user to address.
func DefaultNotifyRecipient() string {
	return strings.TrimSpace(os.Getenv("NOTIFY_DEFAULT_RECIPIENT"))
}
