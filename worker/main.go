package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

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

	// Connect to Postgres and make sure our table exists.
	db, err := pgx.Connect(ctx, "postgres://postgres:postgres@localhost:5434/telemetry?sslmode=disable")
	if err != nil {
		fmt.Println("could not connect to postgres:", err)
		return
	}
	defer db.Close(ctx)

	if _, err := db.Exec(ctx, createTable); err != nil {
		fmt.Println("could not create table:", err)
		return
	}

	// Connect to Redis.
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	fmt.Println("worker started: reading from Redis, saving to Postgres...")

	saved := 0
	lastID := "0" // "0" = start from the beginning of the stream
	for {
		streams, err := rdb.XRead(ctx, &redis.XReadArgs{
			Streams: []string{"events", lastID},
			Count:   10,
			Block:   0, // block until new events arrive
		}).Result()
		if err != nil {
			fmt.Println("error reading from redis:", err)
			return
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := save(ctx, db, msg); err != nil {
					fmt.Println("could not save event:", err)
					continue
				}
				saved++
				lastID = msg.ID
				fmt.Printf("saved event %s (total saved: %d)\n", msg.ID, saved)
			}
		}
	}
}

// save parses one stream entry's JSON and inserts it as a row in Postgres.
func save(ctx context.Context, db *pgx.Conn, msg redis.XMessage) error {
	raw, ok := msg.Values["data"].(string) // the "data" field holds our JSON
	if !ok {
		return fmt.Errorf("entry %s has no string data field", msg.ID)
	}

	var e TelemetryEvent
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return fmt.Errorf("bad json in %s: %w", msg.ID, err)
	}

	_, err := db.Exec(ctx,
		`INSERT INTO telemetry_events
		 (stream_id, frame, lat, lon, alt, speed_mps, accel_mps2, yaw_rate_rps, num_satellites)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		msg.ID, e.Frame, e.Lat, e.Lon, e.Alt, e.SpeedMPS, e.AccelMPS2, e.YawRateRPS, e.NumSatellites)
	return err
}
