package store

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/iag/crm/backend/internal/models"
)

type seriesBucket struct {
	trunc    string
	span     time.Duration
	step     time.Duration
	points   int
	labelKey string
}

func seriesConfig(rangeKey string) seriesBucket {
	switch rangeKey {
	case "today":
		return seriesBucket{trunc: "hour", span: 12 * time.Hour, step: time.Hour, points: 12, labelKey: "hour"}
	case "month":
		return seriesBucket{trunc: "week", span: 12 * 7 * 24 * time.Hour, step: 7 * 24 * time.Hour, points: 12, labelKey: "week"}
	case "quarter":
		return seriesBucket{trunc: "month", span: 365 * 24 * time.Hour, step: 30 * 24 * time.Hour, points: 12, labelKey: "month"}
	default:
		return seriesBucket{trunc: "day", span: 12 * 24 * time.Hour, step: 24 * time.Hour, points: 12, labelKey: "day"}
	}
}

func (r *Repository) pipelineSeries(ctx context.Context, rangeKey string) ([]int, []string, error) {
	cfg := seriesConfig(rangeKey)
	if cfg.points <= 0 {
		cfg.points = 12
	}
	now := time.Now().UTC()
	start := now.Add(-cfg.span)

	rows, err := r.db(ctx).Query(ctx, `
		SELECT date_trunc($1, updated_at) AS bucket,
		       COALESCE(SUM(amount * probability / 100.0), 0) AS weighted
		FROM crm_deals
		WHERE stage NOT IN ('lost') AND updated_at >= $2
		GROUP BY 1
		ORDER BY 1
	`, cfg.trunc, start)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	byBucket := map[time.Time]float64{}
	for rows.Next() {
		var bucket time.Time
		var weighted float64
		if err := rows.Scan(&bucket, &weighted); err != nil {
			return nil, nil, err
		}
		byBucket[bucket.UTC()] = weighted
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	series := make([]int, 0, cfg.points)
	labels := make([]string, 0, cfg.points)
	cursor := truncateUTC(start, cfg.trunc)
	end := truncateUTC(now, cfg.trunc)
	for len(series) < cfg.points && !cursor.After(end) {
		weighted := byBucket[cursor]
		series = append(series, int(math.Round(weighted/1000)))
		labels = append(labels, seriesLabel(cursor, cfg.labelKey, len(series) == cfg.points))
		cursor = cursor.Add(cfg.step)
	}
	for len(series) < cfg.points {
		series = append(series, 0)
		labels = append(labels, fmt.Sprintf("-%d", cfg.points-len(series)))
	}
	if len(labels) > 0 {
		labels[len(labels)-1] = "now"
	}
	return series, labels, nil
}

func truncateUTC(t time.Time, unit string) time.Time {
	t = t.UTC()
	switch unit {
	case "hour":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
	case "day":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case "week":
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(weekday - 1))
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return t
	}
}

func seriesLabel(t time.Time, kind string, isLast bool) string {
	if isLast {
		return "now"
	}
	switch kind {
	case "hour":
		return t.Format("15")
	case "week":
		return fmt.Sprintf("w-%d", t.Day())
	case "month":
		return fmt.Sprintf("m-%d", int(t.Month()))
	default:
		return t.Format("02")
	}
}

func pipelineDelta(series []int) string {
	if len(series) < 2 || series[0] == 0 {
		return "live"
	}
	first, last := float64(series[0]), float64(series[len(series)-1])
	delta := (last - first) / first * 100
	if delta >= 0 {
		return fmt.Sprintf("▲ %.1f%%", delta)
	}
	return fmt.Sprintf("▼ %.1f%%", -delta)
}

func overviewRangeLabel(rangeKey string) string {
	switch rangeKey {
	case "today":
		return "Today"
	case "week":
		return "Week"
	case "month":
		return "Month"
	case "quarter":
		return "Quarter"
	default:
		return "Week"
	}
}

func mergeOverviewMetrics(base models.RangeMetrics, series []int, labels []string) models.RangeMetrics {
	if len(series) > 0 {
		base.Series = series
		base.PipelineDelta = pipelineDelta(series)
	}
	if len(labels) > 0 {
		base.XLabels = labels
	}
	return base
}
