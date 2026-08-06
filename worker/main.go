package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

const bucket = "telemetry-raw"

// getenv returns the environment variable `key`, or `fallback` if it's unset/empty.
// Lets the same binary run locally (defaults) or in Docker Compose (env overrides).
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TelemetryEvent mirrors the JSON stored in each stream entry.
type TelemetryEvent struct {
	Frame         int     `json:"frame"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Alt           float64 `json:"alt"`
	SpeedMPS      float64 `json:"speed_mps"`
	AccelMPS2     float64 `json:"accel_mps2"`
	YawRateRPS    float64 `json:"yaw_rate_rps"`
	NumSatellites int     `json:"num_satellites"`
}

const createTable = `
CREATE TABLE IF NOT EXISTS telemetry_events (
    id             BIGSERIAL PRIMARY KEY,
    stream_id      TEXT,
    frame          INTEGER,
    lat            DOUBLE PRECISION,
    lon            DOUBLE PRECISION,
    alt            DOUBLE PRECISION,
    speed_mps      DOUBLE PRECISION,
    accel_mps2     DOUBLE PRECISION,
    yaw_rate_rps   DOUBLE PRECISION,
    num_satellites INTEGER,
    received_at    TIMESTAMPTZ DEFAULT now()
)`

func main() {
	ctx := context.Background()

	// --- Postgres (structured data) ---
	pgURL := getenv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5434/telemetry?sslmode=disable")
	db, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		fmt.Println("could not connect to postgres:", err)
		return
	}
	defer db.Close(ctx)

	if _, err := db.Exec(ctx, createTable); err != nil {
		fmt.Println("could not create table:", err)
		return
	}

	// --- MinIO (object storage: raw payloads) ---
	mc, err := minio.New(getenv("MINIO_ENDPOINT", "localhost:9100"), &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false, // local, no TLS
	})
	if err != nil {
		fmt.Println("could not connect to minio:", err)
		return
	}
	if err := ensureBucket(ctx, mc); err != nil {
		fmt.Println("could not ensure bucket:", err)
		return
	}

	// --- Redis (the queue we consume) ---
	rdb := redis.NewClient(&redis.Options{Addr: getenv("REDIS_ADDR", "localhost:6379")})

	// Create the consumer group "workers" on the "events" stream (idempotent).
	// MkStream also creates the stream if it doesn't exist yet. "0" means the
	// group starts consuming from the very beginning of the stream.
	const group = "workers"
	consumer := getenv("CONSUMER_NAME", "worker-1")
	if err := rdb.XGroupCreateMkStream(ctx, "events", group, "0").Err(); err != nil {
		// "BUSYGROUP" just means the group already exists — expected on restart.
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			fmt.Println("could not create consumer group:", err)
			return
		}
	}

	fmt.Printf("worker %q started in group %q: Redis → Postgres + object storage...\n", consumer, group)

	saved := 0
	// Start by replaying our OWN delivered-but-unacked events (crash recovery),
	// then switch to ">" for brand-new events. Redis keeps the score now, not us.
	readID := "0"
	for {
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{"events", readID},
			Count:    10,
			Block:    0, // block waiting for new events (only matters once readID == ">")
		}).Result()
		if err != nil && err != redis.Nil {
			fmt.Println("error reading from redis:", err)
			return
		}

		// Messages returned by this read (may be empty).
		var msgs []redis.XMessage
		if len(streams) > 0 {
			msgs = streams[0].Messages
		}

		// Finished replaying our old pending events? Switch to brand-new ones.
		if readID != ">" && len(msgs) == 0 {
			readID = ">"
			continue
		}

		for _, msg := range msgs {
			if err := save(ctx, db, mc, msg); err != nil {
				fmt.Println("could not save event:", err)
				continue // do NOT ack — it stays pending and gets retried later
			}
			// Saved to Postgres + MinIO → tell Redis "done" so it won't be redelivered.
			if err := rdb.XAck(ctx, "events", group, msg.ID).Err(); err != nil {
				fmt.Println("could not ack event:", err)
			}
			saved++
			fmt.Printf("saved + acked %s (total: %d)\n", msg.ID, saved)
		}
	}
}

// ensureBucket creates the object-storage bucket if it doesn't already exist.
func ensureBucket(ctx context.Context, mc *minio.Client) error {
	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// save writes one event to BOTH Postgres (a structured row) and MinIO (the raw JSON blob).
func save(ctx context.Context, db *pgx.Conn, mc *minio.Client, msg redis.XMessage) error {
	raw, ok := msg.Values["data"].(string) // the "data" field holds our JSON
	if !ok {
		return fmt.Errorf("entry %s has no string data field", msg.ID)
	}

	var e TelemetryEvent
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return fmt.Errorf("bad json in %s: %w", msg.ID, err)
	}

	// 1) structured row → Postgres
	if _, err := db.Exec(ctx,
		`INSERT INTO telemetry_events
		 (stream_id, frame, lat, lon, alt, speed_mps, accel_mps2, yaw_rate_rps, num_satellites)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		msg.ID, e.Frame, e.Lat, e.Lon, e.Alt, e.SpeedMPS, e.AccelMPS2, e.YawRateRPS, e.NumSatellites,
	); err != nil {
		return fmt.Errorf("postgres insert: %w", err)
	}

	// 2) raw payload → object storage
	key := fmt.Sprintf("events/%s.json", msg.ID)
	_, err := mc.PutObject(ctx, bucket, key, strings.NewReader(raw), int64(len(raw)),
		minio.PutObjectOptions{ContentType: "application/json"})
	if err != nil {
		return fmt.Errorf("minio put: %w", err)
	}

	return nil
}
