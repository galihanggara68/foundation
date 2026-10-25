package gohex

func emitGoDecimalDef(domainPkg string) string {
	return `package ` + domainPkg + `

import (
	"database/sql/driver"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Decimal represents a fixed-precision 4-decimal-place number for currency and financial domain calculations.
type Decimal struct {
	units int64
}

func NewDecimal(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Decimal{}, nil
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return Decimal{}, fmt.Errorf("invalid decimal %q", s)
	}
	var whole int64
	var err error
	if parts[0] != "" {
		whole, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return Decimal{}, fmt.Errorf("invalid decimal %q: %w", s, err)
		}
	}
	var frac int64
	if len(parts) == 2 {
		fStr := parts[1]
		if len(fStr) > 4 {
			fStr = fStr[:4]
		}
		for len(fStr) < 4 {
			fStr += "0"
		}
		frac, err = strconv.ParseInt(fStr, 10, 64)
		if err != nil {
			return Decimal{}, fmt.Errorf("invalid decimal %q: %w", s, err)
		}
	}
	units := whole*10000 + frac
	if neg {
		units = -units
	}
	return Decimal{units: units}, nil
}

func MustNewDecimal(s string) Decimal {
	d, err := NewDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

func NewDecimalFromInt(n int64) Decimal {
	return Decimal{units: n * 10000}
}

func NewDecimalFromFloat(f float64) Decimal {
	return Decimal{units: int64(math.Round(f * 10000))}
}

func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{units: d.units + other.units}
}

func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{units: d.units - other.units}
}

func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{units: (d.units * other.units) / 10000}
}

func (d Decimal) Div(other Decimal) Decimal {
	if other.units == 0 {
		return Decimal{}
	}
	return Decimal{units: (d.units * 10000) / other.units}
}

func (d Decimal) Equal(other Decimal) bool {
	return d.units == other.units
}

func (d Decimal) GreaterThan(other Decimal) bool {
	return d.units > other.units
}

func (d Decimal) LessThan(other Decimal) bool {
	return d.units < other.units
}

func (d Decimal) GreaterThanOrEqual(other Decimal) bool {
	return d.units >= other.units
}

func (d Decimal) LessThanOrEqual(other Decimal) bool {
	return d.units <= other.units
}

func (d Decimal) Float64() float64 {
	return float64(d.units) / 10000.0
}

func (d Decimal) String() string {
	sign := ""
	u := d.units
	if u < 0 {
		sign = "-"
		u = -u
	}
	whole := u / 10000
	frac := u % 10000
	return fmt.Sprintf("%s%d.%04d", sign, whole, frac)
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Decimal) UnmarshalJSON(b []byte) error {
	raw := strings.Trim(string(b), "\"")
	dec, err := NewDecimal(raw)
	if err != nil {
		return err
	}
	*d = dec
	return nil
}

func (d *Decimal) Scan(src any) error {
	if src == nil {
		*d = Decimal{}
		return nil
	}
	switch v := src.(type) {
	case string:
		dec, err := NewDecimal(v)
		if err != nil {
			return err
		}
		*d = dec
		return nil
	case []byte:
		dec, err := NewDecimal(string(v))
		if err != nil {
			return err
		}
		*d = dec
		return nil
	case int64:
		*d = NewDecimalFromInt(v)
		return nil
	case float64:
		*d = NewDecimalFromFloat(v)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Decimal", src)
	}
}

func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}
`
}
