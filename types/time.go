package types

import (
	"time"
)

type Time time.Time

func (t *Time) UnmarshalStateBytes(bs []byte) error {
	t2, err := time.Parse("2006-01-02T15:04:05Z", string(bs))
	if err != nil {
		return err
	}
	*t = Time(t2.In(time.UTC))
	return nil
}

func (t Time) MarshalStateBytes() ([]byte, error) {
	return []byte(time.Time(t).In(time.UTC).Format("2006-01-02T15:04:05Z")), nil
}
