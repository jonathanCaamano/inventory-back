package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// DateOnly accepts JSON dates in "2006-01-02" format and stores them as time.Time.
type DateOnly struct {
	time.Time
}

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		return nil
	}
	// strip quotes
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	formats := []string{"2006-01-02", time.RFC3339}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			d.Time = t
			return nil
		}
	}
	return fmt.Errorf("cannot parse %q as date (expected YYYY-MM-DD)", s)
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Format("2006-01-02") + `"`), nil
}

// Value implements driver.Valuer so GORM stores it as time.Time.
func (d DateOnly) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.Time, nil
}

// Scan implements sql.Scanner so GORM can read it back.
func (d *DateOnly) Scan(value interface{}) error {
	if value == nil {
		d.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		d.Time = v
		return nil
	}
	return fmt.Errorf("cannot scan type %T into DateOnly", value)
}
