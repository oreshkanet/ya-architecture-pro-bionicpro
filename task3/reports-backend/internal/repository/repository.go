package repository

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

const repoLogPrefix = "[reports-backend][repository]"

type ReportRow struct {
	UserID       string     `json:"user_id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	RegisteredAt *time.Time `json:"registered_at,omitempty"`
	PeriodStart  string     `json:"period_start"`
	PeriodEnd    string     `json:"period_end"`
	UsageHours   float64    `json:"usage_hours"`
	SessionCount int        `json:"session_count"`
	AvgDailyUse  float64    `json:"avg_daily_use"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ReportRepository struct {
	db *sql.DB
}

func New(dsn string) (*ReportRepository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &ReportRepository{db: db}, nil
}

func (r *ReportRepository) GetPeriodByUserID(ctx context.Context, userID string) (periodStart, periodEnd string, err error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT period_start, period_end
		FROM report_mart
		WHERE user_id = $1
	`, userID)
	var start, end time.Time
	if err := row.Scan(&start, &end); err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", err
	}
	return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}

func (r *ReportRepository) GetByUserID(ctx context.Context, userID string) (*ReportRow, error) {
	log.Printf("%s GetByUserID user=%s", repoLogPrefix, userID)
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, full_name, email, registered_at,
		       period_start, period_end,
		       usage_hours, session_count, avg_daily_use, updated_at
		FROM report_mart
		WHERE user_id = $1
	`, userID)
	var rep ReportRow
	var regAt sql.NullTime
	var updAt time.Time
	var periodStartT, periodEndT time.Time
	err := row.Scan(
		&rep.UserID,
		&rep.FullName,
		&rep.Email,
		&regAt,
		&periodStartT,
		&periodEndT,
		&rep.UsageHours,
		&rep.SessionCount,
		&rep.AvgDailyUse,
		&updAt,
	)
	if err == sql.ErrNoRows {
		log.Printf("%s GetByUserID user=%s no rows", repoLogPrefix, userID)
		return nil, nil
	}
	if err != nil {
		log.Printf("%s GetByUserID user=%s error: %v", repoLogPrefix, userID, err)
		return nil, err
	}
	log.Printf("%s GetByUserID user=%s ok", repoLogPrefix, userID)
	if regAt.Valid {
		rep.RegisteredAt = &regAt.Time
	}
	rep.UpdatedAt = updAt
	rep.PeriodStart = periodStartT.Format("2006-01-02")
	rep.PeriodEnd = periodEndT.Format("2006-01-02")
	return &rep, nil
}
