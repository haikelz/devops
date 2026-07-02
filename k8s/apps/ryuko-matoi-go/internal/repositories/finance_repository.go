package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type FinanceRecord struct {
	ID          int64
	Phone       string
	Type        string // "income" || "expense"
	Amount      float64
	Category    string
	Description string
	CreatedAt   time.Time
}

type FinanceRepository struct {
	db *sql.DB
}

func NewFinanceRepository(dbPath string) (*FinanceRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	repo := &FinanceRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *FinanceRepository) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS finance_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phone TEXT NOT NULL,
		type TEXT NOT NULL,
		amount REAL NOT NULL,
		category TEXT NOT NULL,
		description TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_finance_records_phone ON finance_records(phone);
	CREATE INDEX IF NOT EXISTS idx_finance_records_created_at ON finance_records(created_at);
	`
	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (r *FinanceRepository) InsertRecord(ctx context.Context, record *FinanceRecord) error {
	query := `
	INSERT INTO finance_records (
		phone, 
		type, 
		amount, 
		category, 
		description, 
		created_at
	) VALUES (?, ?, ?, ?, ?, ?)
	`
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	_, err = tx.ExecContext(ctx, query, record.Phone, record.Type, record.Amount, record.Category, record.Description, createdAt)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("insert finance record: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *FinanceRepository) GetMonthlyReport(ctx context.Context, year int, month time.Month) (map[string]map[string]float64, error) {
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, 0)

	query := `
	SELECT phone, type, SUM(amount)
	FROM finance_records
	WHERE created_at >= ? AND created_at < ?
	GROUP BY phone, type
	`
	rows, err := r.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query monthly report: %w", err)
	}
	defer rows.Close()

	report := make(map[string]map[string]float64)
	for rows.Next() {
		var phone, recordType string
		var total float64
		if err := rows.Scan(&phone, &recordType, &total); err != nil {
			return nil, fmt.Errorf("scan monthly report: %w", err)
		}

		if report[phone] == nil {
			report[phone] = make(map[string]float64)
		}
		report[phone][recordType] = total
	}
	return report, nil
}

func (r *FinanceRepository) GetDetailedMonthlyReport(ctx context.Context, phone string, year int, month time.Month) ([]FinanceRecord, error) {
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, 0)

	query := `
	SELECT id, phone, type, amount, category, description, created_at
	FROM finance_records
	WHERE phone = ? AND created_at >= ? AND created_at < ?
	ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, phone, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query detailed report: %w", err)
	}
	defer rows.Close()

	var records []FinanceRecord
	for rows.Next() {
		var rec FinanceRecord
		if err := rows.Scan(&rec.ID, &rec.Phone, &rec.Type, &rec.Amount, &rec.Category, &rec.Description, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan detailed report: %w", err)
		}
		records = append(records, rec)
	}
	return records, nil
}
