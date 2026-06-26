package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ReportStore struct {
	db *sql.DB
}

func NewReportStore(db *sql.DB) *ReportStore {
	return &ReportStore{db: db}
}

type DeviceInfo struct {
	Name           string `json:"name"`
	Model          string `json:"model"`
	Manufacturer   string `json:"manufacturer"`
	AndroidVersion string `json:"android_version"`
	DeviceID       string `json:"device_id"`
}

type UploadRequest struct {
	SessionID    string          `json:"session_id"`
	Device       DeviceInfo      `json:"device"`
	Network      json.RawMessage `json:"network"`
	TestResults  json.RawMessage `json:"test_results"`
	XlatSummary  json.RawMessage `json:"xlat_summary"`
	PassCount    int             `json:"pass_count"`
	TotalCount   int             `json:"total_count"`
	RunTimestamp int64           `json:"run_timestamp"`
}

type ReportSummary struct {
	ID           string `json:"id"`
	DeviceName   string `json:"device_name"`
	DeviceModel  string `json:"device_model"`
	PassCount    int    `json:"pass_count"`
	TotalCount   int    `json:"total_count"`
	RunTimestamp int64  `json:"run_timestamp"`
	UploadedAt   int64  `json:"uploaded_at"`
}

type ReportDetail struct {
	ReportSummary
	DeviceManufacturer string          `json:"device_manufacturer"`
	AndroidVersion     string          `json:"android_version"`
	DeviceID           string          `json:"device_id"`
	Network            json.RawMessage `json:"network"`
	TestResults        json.RawMessage `json:"test_results"`
	XlatSummary        json.RawMessage `json:"xlat_summary"`
}

type ListFilter struct {
	From   *time.Time
	To     *time.Time
	Device string
	Limit  int
	Offset int
}

func (s *ReportStore) Upsert(req *UploadRequest) error {
	xlatJSON := []byte("null")
	if len(req.XlatSummary) > 0 && string(req.XlatSummary) != "null" {
		xlatJSON = req.XlatSummary
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO reports
			(id, device_name, device_model, device_manufacturer, android_version, device_id,
			 network_json, test_results_json, xlat_summary_json,
			 pass_count, total_count, run_timestamp, uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.SessionID,
		req.Device.Name,
		req.Device.Model,
		req.Device.Manufacturer,
		req.Device.AndroidVersion,
		req.Device.DeviceID,
		string(req.Network),
		string(req.TestResults),
		string(xlatJSON),
		req.PassCount,
		req.TotalCount,
		req.RunTimestamp,
		time.Now().UnixMilli(),
	)
	return err
}

func (s *ReportStore) List(f ListFilter) ([]ReportSummary, int, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	query := `SELECT id, device_name, device_model, pass_count, total_count, run_timestamp, uploaded_at
	          FROM reports WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM reports WHERE 1=1`
	args := []any{}

	if f.From != nil {
		query += " AND uploaded_at >= ?"
		countQuery += " AND uploaded_at >= ?"
		args = append(args, f.From.UnixMilli())
	}
	if f.To != nil {
		end := f.To.Add(24*time.Hour - time.Millisecond)
		query += " AND uploaded_at <= ?"
		countQuery += " AND uploaded_at <= ?"
		args = append(args, end.UnixMilli())
	}
	if f.Device != "" {
		query += " AND device_name LIKE ?"
		countQuery += " AND device_name LIKE ?"
		args = append(args, "%"+f.Device+"%")
	}

	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query += " ORDER BY uploaded_at DESC LIMIT ? OFFSET ?"
	rows, err := s.db.Query(query, append(args, limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var summaries []ReportSummary
	for rows.Next() {
		var r ReportSummary
		if err := rows.Scan(&r.ID, &r.DeviceName, &r.DeviceModel,
			&r.PassCount, &r.TotalCount, &r.RunTimestamp, &r.UploadedAt); err != nil {
			return nil, 0, err
		}
		summaries = append(summaries, r)
	}
	if summaries == nil {
		summaries = []ReportSummary{}
	}
	return summaries, total, rows.Err()
}

func (s *ReportStore) GetByID(id string) (*ReportDetail, error) {
	row := s.db.QueryRow(`
		SELECT id, device_name, device_model, device_manufacturer, android_version, device_id,
		       pass_count, total_count, run_timestamp, uploaded_at,
		       network_json, test_results_json, xlat_summary_json
		FROM reports WHERE id = ?`, id)

	var r ReportDetail
	var networkJSON, testResultsJSON, xlatJSON string
	err := row.Scan(
		&r.ID, &r.DeviceName, &r.DeviceModel, &r.DeviceManufacturer,
		&r.AndroidVersion, &r.DeviceID,
		&r.PassCount, &r.TotalCount, &r.RunTimestamp, &r.UploadedAt,
		&networkJSON, &testResultsJSON, &xlatJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Network = json.RawMessage(networkJSON)
	r.TestResults = json.RawMessage(testResultsJSON)
	r.XlatSummary = json.RawMessage(xlatJSON)
	return &r, nil
}
