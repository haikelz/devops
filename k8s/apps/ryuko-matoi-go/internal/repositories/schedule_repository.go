package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Schedule struct {
	ID           int64
	Phone        string
	ScheduleName string
	ScheduleTime time.Time
	Reminded30   bool
	Reminded15   bool
	Reminded5    bool
	CreatedAt    time.Time
}

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(dbPath string) (*ScheduleRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	repo := &ScheduleRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *ScheduleRepository) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS schedules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phone TEXT NOT NULL,
		schedule_name TEXT NOT NULL,
		schedule_time DATETIME NOT NULL,
		reminded_30 BOOLEAN DEFAULT FALSE,
		reminded_15 BOOLEAN DEFAULT FALSE,
		reminded_5 BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_schedules_phone ON schedules(phone);
	CREATE INDEX IF NOT EXISTS idx_schedules_schedule_time ON schedules(schedule_time);
	`
	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) InsertSchedule(ctx context.Context, schedule *Schedule) error {
	query := `
	INSERT INTO schedules (
		phone, 
		schedule_name, 
		schedule_time, 
		created_at
	) VALUES (?, ?, ?, ?)
	`
	createdAt := schedule.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	_, err = tx.ExecContext(ctx, query, schedule.Phone, schedule.ScheduleName, schedule.ScheduleTime, createdAt)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("insert schedule: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *ScheduleRepository) GetUpcomingSchedules(ctx context.Context) ([]Schedule, error) {
	// We want schedules that haven't been fully reminded (not reminded_5 means it hasn't reached 5 mins before)
	// Or maybe just fetch all that are in the future + some buffer.
	query := `
	SELECT id, phone, schedule_name, schedule_time, reminded_30, reminded_15, reminded_5, created_at
	FROM schedules
	WHERE schedule_time > ? AND (reminded_30 = FALSE OR reminded_15 = FALSE OR reminded_5 = FALSE)
	`
	rows, err := r.db.QueryContext(ctx, query, time.Now().Add(-10 * time.Minute)) // little buffer
	if err != nil {
		return nil, fmt.Errorf("query upcoming schedules: %w", err)
	}
	defer rows.Close()

	var records []Schedule
	for rows.Next() {
		var rec Schedule
		if err := rows.Scan(&rec.ID, &rec.Phone, &rec.ScheduleName, &rec.ScheduleTime, &rec.Reminded30, &rec.Reminded15, &rec.Reminded5, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan upcoming schedule: %w", err)
		}
		records = append(records, rec)
	}
	return records, nil
}

func (r *ScheduleRepository) UpdateRemindedStatus(ctx context.Context, id int64, min int) error {
	var field string
	if min == 30 {
		field = "reminded_30"
	} else if min == 15 {
		field = "reminded_15"
	} else if min == 5 {
		field = "reminded_5"
	} else {
		return fmt.Errorf("invalid reminder minute: %d", min)
	}

	query := fmt.Sprintf("UPDATE schedules SET %s = TRUE WHERE id = ?", field)
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
