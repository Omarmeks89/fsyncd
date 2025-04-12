// package contains type for handle synchronization time

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultTimePartsSeparator to parse h, m, s
const (
	// OSLocalTimeLink is used for setup local time from OS
	OSLocalTimeLink           = "/etc/localtime"
	OSLinkDefaultSeparator    = "/"
	DefaultTimePartsSeparator = ":"

	// numeric const section

	// RequiredTimePartsCount is 3 for hours, minutes and seconds
	RequiredTimePartsCount = 3

	MinPossibleTimeValue   = 0
	MaxPossibleHoursValue  = 23
	MaxPossibleMinSecValue = 59
)

// SyncTimeGenerator for handle sync time
type SyncTimeGenerator struct {
	H        int
	M        int
	S        int
	location *time.Location
}

// SetupSyncTime convert time string (like 12:45:15) into numeric values
// for hours, minutes and seconds
func (stp *SyncTimeGenerator) SetupSyncTime(tmFmt string) (err error) {

	parts := strings.Split(tmFmt, DefaultTimePartsSeparator)
	if len(parts) != RequiredTimePartsCount {
		return fmt.Errorf("invalid time parts count")
	}

	if err = stp.SetHours(parts[0]); err != nil {
		return err
	}

	if err = stp.SetMinutes(parts[1]); err != nil {
		return err
	}

	if err = stp.SetSeconds(parts[2]); err != nil {
		return err
	}

	return err
}

// SetHours (int) from time string
func (stp *SyncTimeGenerator) SetHours(h string) (err error) {
	if stp.H, err = strconv.Atoi(h); err != nil {
		return err
	}

	if stp.H < MinPossibleTimeValue || stp.H > MaxPossibleHoursValue {
		return fmt.Errorf("hours have to be in between of 0 and 23")
	}

	return err
}

// SetMinutes (int) from time string
func (stp *SyncTimeGenerator) SetMinutes(m string) (err error) {
	if stp.M, err = strconv.Atoi(m); err != nil {
		return err
	}

	if stp.M < MinPossibleTimeValue || stp.M > MaxPossibleMinSecValue {
		return fmt.Errorf("minutes have to be in between of 0 and 59")
	}

	return err
}

func (stp *SyncTimeGenerator) SetSeconds(s string) (err error) {
	if stp.S, err = strconv.Atoi(s); err != nil {
		return err
	}

	if stp.S < MinPossibleTimeValue || stp.S > MaxPossibleMinSecValue {
		return fmt.Errorf("seconds have to be in between of 0 and 59")
	}
	return err
}

// SetSyncTime from current time
func (stp *SyncTimeGenerator) SetSyncTime(origin time.Time) (
	t time.Duration,
	err error,
) {
	// set basic time
	y, m, day := origin.Date()
	tSync := time.Date(y, m, day, stp.H, stp.M, stp.S, 0, stp.location)

	if origin.After(tSync) {
		// add 24 hours for truncated because it before current time
		tSync = tSync.Add(24 * time.Hour)
	}

	return tSync.Sub(origin), err
}

// GetLocalTime get time for current location (timezone)
func (stp *SyncTimeGenerator) GetLocalTime() (t time.Time, err error) {
	if stp.location == nil {
		return t, fmt.Errorf("location not set")
	}

	return time.Now().Local(), err
}

// GenerateInterval for repeat wished operation (or set timer object)
func (stp *SyncTimeGenerator) GenerateInterval() (d time.Duration, err error) {
	var tm time.Time

	if tm, err = stp.GetLocalTime(); err != nil {
		return d, err
	}

	return stp.SetSyncTime(tm)
}

// SetLocalTime from OS settings (/etc/localtime)
func (stp *SyncTimeGenerator) SetLocalTime(location string) (err error) {
	if stp.location, err = time.LoadLocation(location); err != nil {
		return err
	}

	return err
}
