package cronjob

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// Shared ranges
	minSecond = 0
	maxSecond = 59
	minMinute = 0
	maxMinute = 59
	minHour   = 0
	maxHour   = 23

	// Day/Month/Week ranges
	minDayOfMonth = 1
	maxDayOfMonth = 31
	minMonth      = 1
	maxMonth      = 12
	minDayOfWeek  = 0 // Sunday
	maxDayOfWeek  = 6 // Saturday
)

// CronBuilder facilitates building a cron expression programmatically.
type CronBuilder struct {
	second     string // Optional: 0-59
	minute     string // 0-59
	hour       string // 0-23
	dayOfMonth string // 1-31
	month      string // 1-12
	dayOfWeek  string // 0-6 (or 7)

	withSeconds bool  // Flag indicating if the seconds field is used
	err         error // Stores the first error encountered
}

// New creates a new CronBuilder for a standard 5-field cron expression (minute, hour, dom, month, dow).
// Seconds field is disabled by default.
func NewCronBuilder() *CronBuilder {
	return &CronBuilder{
		minute:      "*",
		hour:        "*",
		dayOfMonth:  "*",
		month:       "*",
		dayOfWeek:   "*",
		withSeconds: false, // Explicitly false for standard cron
	}
}

// setError stores the first error encountered. Subsequent calls become no-ops if an error exists.
func (b *CronBuilder) setError(err error) *CronBuilder {
	if b.err == nil && err != nil {
		b.err = err
	}
	return b
}

// checkSecondsEnabled returns an error if seconds methods are called when disabled.
func (b *CronBuilder) checkSecondsEnabled() error {
	if !b.withSeconds {
		return errors.New("cannot set second field: seconds are not enabled (use NewWithSeconds)")
	}
	return nil
}

// validate validates a value against min/max for a given field name.
func validate(val, min, max int, fieldName string) error {
	if val < min || val > max {
		return fmt.Errorf("invalid %s: %d (must be %d-%d)", fieldName, val, min, max)
	}
	return nil
}

// joinInts converts a slice of integers to a sorted, comma-separated string.
func joinInts(vals []int) string {
	if len(vals) == 0 {
		return "" // Should be handled by caller validation
	}

	// Remove duplicates and sort
	seen := make(map[int]bool)
	distinct := []int{}
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			distinct = append(distinct, v)
		}
	}

	sort.Ints(distinct)
	sVals := make([]string, len(distinct))
	for i, v := range distinct {
		sVals[i] = strconv.Itoa(v)
	}
	return strings.Join(sVals, ",")
}

func (b *CronBuilder) EnableSeconds() *CronBuilder {
	if b.second == "" {
		b.second = "*"
	}
	b.withSeconds = true
	return b
}

// EverySecond sets the second field to '*' (every second).
// Returns an error if the builder was not created with NewWithSeconds().
func (b *CronBuilder) EverySecond() *CronBuilder {
	if err := b.checkSecondsEnabled(); err != nil {
		return b.setError(err)
	}
	if b.err != nil {
		return b
	}
	b.second = "*"
	return b
}

// Second sets the second field to a specific value.
// Returns an error if the builder was not created with NewWithSeconds().
func (b *CronBuilder) Second(second int) *CronBuilder {
	if err := b.checkSecondsEnabled(); err != nil {
		return b.setError(err)
	}
	if b.err != nil {
		return b
	}
	if err := validate(second, minSecond, maxSecond, "second"); err != nil {
		return b.setError(err)
	}
	b.second = strconv.Itoa(second)
	return b
}

// Seconds sets the second field to a list of specific values.
// Returns an error if the builder was not created with NewWithSeconds().
func (b *CronBuilder) Seconds(seconds ...int) *CronBuilder {
	if err := b.checkSecondsEnabled(); err != nil {
		return b.setError(err)
	}
	if b.err != nil {
		return b
	}
	if len(seconds) == 0 {
		return b.setError(errors.New("seconds list cannot be empty"))
	}
	for _, s := range seconds {
		if err := validate(s, minSecond, maxSecond, "second"); err != nil {
			return b.setError(err)
		}
	}
	b.second = joinInts(seconds)
	return b
}

// SecondRange sets the second field to a range (e.g., 5-10).
// Returns an error if the builder was not created with NewWithSeconds().
func (b *CronBuilder) SecondRange(start, end int) *CronBuilder {
	if err := b.checkSecondsEnabled(); err != nil {
		return b.setError(err)
	}
	if b.err != nil {
		return b
	}
	if err := validate(start, minSecond, maxSecond, "second range start"); err != nil {
		return b.setError(err)
	}
	if err := validate(end, minSecond, maxSecond, "second range end"); err != nil {
		return b.setError(err)
	}
	if start > end {
		return b.setError(fmt.Errorf("invalid second range: start (%d) cannot be greater than end (%d)", start, end))
	}
	b.second = fmt.Sprintf("%d-%d", start, end)
	return b
}

// SecondStep sets the second field to a step value (e.g., */15).
// Returns an error if the builder was not created with NewWithSeconds().
func (b *CronBuilder) SecondStep(step int) *CronBuilder {
	if err := b.checkSecondsEnabled(); err != nil {
		return b.setError(err)
	}
	if b.err != nil {
		return b
	}
	if step <= 0 || step > maxSecond {
		return b.setError(fmt.Errorf("invalid second step: %d (must be > 0 and <= %d)", step, maxSecond))
	}
	b.second = fmt.Sprintf("*/%d", step)
	return b
}

// SecondRangeStep sets the second field to a stepped range (e.g., 0-30/5).
// Returns an error if the builder was not created with NewWithSeconds().
func (b *CronBuilder) SecondRangeStep(start, end, step int) *CronBuilder {
	if err := b.checkSecondsEnabled(); err != nil {
		return b.setError(err)
	}
	if b.err != nil {
		return b
	}
	// Reuse range validation
	b.SecondRange(start, end)
	if b.err != nil {
		return b // Return if SecondRange failed
	}
	// Validate step
	if step <= 0 || step > (end-start+1) { // Step must be positive and reasonable for the range
		return b.setError(fmt.Errorf("invalid second step for range %d-%d: %d", start, end, step))
	}
	b.second = fmt.Sprintf("%d-%d/%d", start, end, step)
	return b
}

// --- Minute Field ---

// EveryMinute sets the minute field to '*' (every minute).
func (b *CronBuilder) EveryMinute() *CronBuilder {
	if b.err != nil {
		return b
	}
	b.minute = "*"
	return b
}

// Minute sets the minute field to a specific value.
func (b *CronBuilder) Minute(minute int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(minute, minMinute, maxMinute, "minute"); err != nil {
		return b.setError(err)
	}
	b.minute = strconv.Itoa(minute)
	return b
}

// Minutes sets the minute field to a list of specific values.
func (b *CronBuilder) Minutes(minutes ...int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if len(minutes) == 0 {
		return b.setError(errors.New("minutes list cannot be empty"))
	}
	for _, m := range minutes {
		if err := validate(m, minMinute, maxMinute, "minute"); err != nil {
			return b.setError(err)
		}
	}
	b.minute = joinInts(minutes)
	return b
}

// MinuteRange sets the minute field to a range (e.g., 5-10).
func (b *CronBuilder) MinuteRange(start, end int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(start, minMinute, maxMinute, "minute range start"); err != nil {
		return b.setError(err)
	}
	if err := validate(end, minMinute, maxMinute, "minute range end"); err != nil {
		return b.setError(err)
	}
	if start > end {
		return b.setError(fmt.Errorf("invalid minute range: start (%d) cannot be greater than end (%d)", start, end))
	}
	b.minute = fmt.Sprintf("%d-%d", start, end)
	return b
}

// MinuteStep sets the minute field to a step value (e.g., */15).
func (b *CronBuilder) MinuteStep(step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if step <= 0 || step > maxMinute {
		return b.setError(fmt.Errorf("invalid minute step: %d (must be > 0 and <= %d)", step, maxMinute))
	}
	b.minute = fmt.Sprintf("*/%d", step)
	return b
}

// MinuteRangeStep sets the minute field to a stepped range (e.g., 0-30/5).
func (b *CronBuilder) MinuteRangeStep(start, end, step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	b.MinuteRange(start, end)
	if b.err != nil {
		return b
	}
	if step <= 0 || step > (end-start+1) {
		return b.setError(fmt.Errorf("invalid minute step for range %d-%d: %d", start, end, step))
	}
	b.minute = fmt.Sprintf("%d-%d/%d", start, end, step)
	return b
}

// --- Hour Field ---

// EveryHour sets the hour field to '*'.
func (b *CronBuilder) EveryHour() *CronBuilder {
	if b.err != nil {
		return b
	}
	b.hour = "*"
	return b
}

// Hour sets the hour field to a specific value.
func (b *CronBuilder) Hour(hour int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(hour, minHour, maxHour, "hour"); err != nil {
		return b.setError(err)
	}
	b.hour = strconv.Itoa(hour)
	return b
}

// Hours sets the hour field to a list of specific values.
func (b *CronBuilder) Hours(hours ...int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if len(hours) == 0 {
		return b.setError(errors.New("hours list cannot be empty"))
	}
	for _, h := range hours {
		if err := validate(h, minHour, maxHour, "hour"); err != nil {
			return b.setError(err)
		}
	}
	b.hour = joinInts(hours)
	return b
}

// HourRange sets the hour field to a range.
func (b *CronBuilder) HourRange(start, end int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(start, minHour, maxHour, "hour range start"); err != nil {
		return b.setError(err)
	}
	if err := validate(end, minHour, maxHour, "hour range end"); err != nil {
		return b.setError(err)
	}
	if start > end {
		return b.setError(fmt.Errorf("invalid hour range: start (%d) cannot be greater than end (%d)", start, end))
	}
	b.hour = fmt.Sprintf("%d-%d", start, end)
	return b
}

// HourStep sets the hour field to a step value.
func (b *CronBuilder) HourStep(step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if step <= 0 || step > maxHour {
		return b.setError(fmt.Errorf("invalid hour step: %d (must be > 0 and <= %d)", step, maxHour))
	}
	b.hour = fmt.Sprintf("*/%d", step)
	return b
}

// HourRangeStep sets the hour field to a stepped range.
func (b *CronBuilder) HourRangeStep(start, end, step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	b.HourRange(start, end)
	if b.err != nil {
		return b
	}
	if step <= 0 || step > (end-start+1) {
		return b.setError(fmt.Errorf("invalid hour step for range %d-%d: %d", start, end, step))
	}
	b.hour = fmt.Sprintf("%d-%d/%d", start, end, step)
	return b
}

// --- DayOfMonth Field ---

// EveryDayOfMonth sets the dayOfMonth field to '*'.
func (b *CronBuilder) EveryDayOfMonth() *CronBuilder {
	if b.err != nil {
		return b
	}
	b.dayOfMonth = "*"
	return b
}

// DayOfMonth sets the dayOfMonth field to a specific value.
func (b *CronBuilder) DayOfMonth(day int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(day, minDayOfMonth, maxDayOfMonth, "day of month"); err != nil {
		return b.setError(err)
	}
	b.dayOfMonth = strconv.Itoa(day)
	return b
}

// DaysOfMonth sets the dayOfMonth field to a list of specific values.
func (b *CronBuilder) DaysOfMonth(days ...int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if len(days) == 0 {
		return b.setError(errors.New("days of month list cannot be empty"))
	}
	for _, d := range days {
		if err := validate(d, minDayOfMonth, maxDayOfMonth, "day of month"); err != nil {
			return b.setError(err)
		}
	}
	b.dayOfMonth = joinInts(days)
	return b
}

// DayOfMonthRange sets the dayOfMonth field to a range.
func (b *CronBuilder) DayOfMonthRange(start, end int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(start, minDayOfMonth, maxDayOfMonth, "day of month range start"); err != nil {
		return b.setError(err)
	}
	if err := validate(end, minDayOfMonth, maxDayOfMonth, "day of month range end"); err != nil {
		return b.setError(err)
	}
	if start > end {
		return b.setError(fmt.Errorf("invalid day of month range: start (%d) cannot be greater than end (%d)", start, end))
	}
	b.dayOfMonth = fmt.Sprintf("%d-%d", start, end)
	return b
}

// DayOfMonthStep sets the dayOfMonth field to a step value.
func (b *CronBuilder) DayOfMonthStep(step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if step <= 0 || step > maxDayOfMonth {
		return b.setError(fmt.Errorf("invalid day of month step: %d (must be > 0 and <= %d)", step, maxDayOfMonth))
	}
	b.dayOfMonth = fmt.Sprintf("*/%d", step)
	return b
}

// DayOfMonthRangeStep sets the dayOfMonth field to a stepped range.
func (b *CronBuilder) DayOfMonthRangeStep(start, end, step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	b.DayOfMonthRange(start, end)
	if b.err != nil {
		return b
	}
	if step <= 0 || step > (end-start+1) {
		return b.setError(fmt.Errorf("invalid day of month step for range %d-%d: %d", start, end, step))
	}
	b.dayOfMonth = fmt.Sprintf("%d-%d/%d", start, end, step)
	return b
}

// --- Month Field ---

// EveryMonth sets the month field to '*'.
func (b *CronBuilder) EveryMonth() *CronBuilder {
	if b.err != nil {
		return b
	}
	b.month = "*"
	return b
}

// Month sets the month field to a specific value.
func (b *CronBuilder) Month(month int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(month, minMonth, maxMonth, "month"); err != nil {
		return b.setError(err)
	}
	b.month = strconv.Itoa(month)
	return b
}

// Months sets the month field to a list of specific values.
func (b *CronBuilder) Months(months ...int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if len(months) == 0 {
		return b.setError(errors.New("months list cannot be empty"))
	}
	for _, m := range months {
		if err := validate(m, minMonth, maxMonth, "month"); err != nil {
			return b.setError(err)
		}
	}
	b.month = joinInts(months)
	return b
}

// MonthRange sets the month field to a range.
func (b *CronBuilder) MonthRange(start, end int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if err := validate(start, minMonth, maxMonth, "month range start"); err != nil {
		return b.setError(err)
	}
	if err := validate(end, minMonth, maxMonth, "month range end"); err != nil {
		return b.setError(err)
	}
	if start > end {
		return b.setError(fmt.Errorf("invalid month range: start (%d) cannot be greater than end (%d)", start, end))
	}
	b.month = fmt.Sprintf("%d-%d", start, end)
	return b
}

// MonthStep sets the month field to a step value.
func (b *CronBuilder) MonthStep(step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if step <= 0 || step > maxMonth {
		return b.setError(fmt.Errorf("invalid month step: %d (must be > 0 and <= %d)", step, maxMonth))
	}
	b.month = fmt.Sprintf("*/%d", step)
	return b
}

// MonthRangeStep sets the month field to a stepped range.
func (b *CronBuilder) MonthRangeStep(start, end, step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	b.MonthRange(start, end)
	if b.err != nil {
		return b
	}
	if step <= 0 || step > (end-start+1) {
		return b.setError(fmt.Errorf("invalid month step for range %d-%d: %d", start, end, step))
	}
	b.month = fmt.Sprintf("%d-%d/%d", start, end, step)
	return b
}

// --- DayOfWeek Field ---

// EveryDayOfWeek sets the dayOfWeek field to '*'.
func (b *CronBuilder) EveryDayOfWeek() *CronBuilder {
	if b.err != nil {
		return b
	}
	b.dayOfWeek = "*"
	return b
}

// DayOfWeek sets the dayOfWeek field to a specific value (0=Sun, 6=Sat).
func (b *CronBuilder) DayOfWeek(day int) *CronBuilder {
	if b.err != nil {
		return b
	}
	actualDay := day
	if day == 7 {
		actualDay = 0
	}
	if err := validate(actualDay, minDayOfWeek, maxDayOfWeek, "day of week"); err != nil {
		return b.setError(fmt.Errorf("invalid day of week: %d (must be 0-6 or 7, where 0 and 7 are Sunday)", day))
	}
	b.dayOfWeek = strconv.Itoa(actualDay)
	return b
}

// DaysOfWeek sets the dayOfWeek field to a list of specific values (0=Sun, 6=Sat).
func (b *CronBuilder) DaysOfWeek(days ...int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if len(days) == 0 {
		return b.setError(errors.New("days of week list cannot be empty"))
	}
	actualDays := make([]int, len(days))
	for i, d := range days {
		actualDay := d
		if d == 7 {
			actualDay = 0
		}
		if err := validate(actualDay, minDayOfWeek, maxDayOfWeek, "day of week"); err != nil {
			return b.setError(fmt.Errorf("invalid day of week in list: %d (must be 0-6 or 7)", d))
		}
		actualDays[i] = actualDay
	}
	b.dayOfWeek = joinInts(actualDays)
	return b
}

// DayOfWeekRange sets the dayOfWeek field to a range (0=Sun, 6=Sat).
func (b *CronBuilder) DayOfWeekRange(start, end int) *CronBuilder {
	if b.err != nil {
		return b
	}
	actualStart := start
	if start == 7 {
		actualStart = 0
	}
	actualEnd := end
	if end == 7 {
		actualEnd = 0
	}
	if err := validate(actualStart, minDayOfWeek, maxDayOfWeek, "day of week range start"); err != nil {
		return b.setError(fmt.Errorf("invalid day of week in range start: %d (must be 0-6 or 7)", start))
	}
	if err := validate(actualEnd, minDayOfWeek, maxDayOfWeek, "day of week range end"); err != nil {
		return b.setError(fmt.Errorf("invalid day of week in range end: %d (must be 0-6 or 7)", end))
	}
	b.dayOfWeek = fmt.Sprintf("%d-%d", actualStart, actualEnd)
	return b
}

// DayOfWeekStep sets the dayOfWeek field to a step value.
func (b *CronBuilder) DayOfWeekStep(step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	if step <= 0 || step > maxDayOfWeek { // Step can technically be 7, but usually smaller
		return b.setError(fmt.Errorf("invalid day of week step: %d (must be > 0 and <= %d)", step, maxDayOfWeek+1))
	}
	b.dayOfWeek = fmt.Sprintf("*/%d", step)
	return b
}

// DayOfWeekRangeStep sets the dayOfWeek field to a stepped range.
func (b *CronBuilder) DayOfWeekRangeStep(start, end, step int) *CronBuilder {
	if b.err != nil {
		return b
	}
	b.DayOfWeekRange(start, end)
	if b.err != nil {
		return b
	}
	if step <= 0 || step > (maxDayOfWeek+1) {
		return b.setError(fmt.Errorf("invalid day of week step for range %d-%d: %d", start, end, step))
	}
	b.dayOfWeek = fmt.Sprintf("%s/%d", b.dayOfWeek, step)
	return b
}

// Build finalizes the cron expression.
// It returns the built expression string (either 5 or 6 fields depending on initialization)
// or the first error encountered during building.
func (b *CronBuilder) Build() (string, error) {
	if b.err != nil {
		return "", b.err
	}

	// Check for internal consistency (should always be set by constructors/methods)
	fields := []string{b.minute, b.hour, b.dayOfMonth, b.month, b.dayOfWeek}
	if b.withSeconds {
		fields = append([]string{b.second}, fields...)
	}
	for _, field := range fields {
		if field == "" {
			return "", errors.New("internal builder error: one or more required fields are empty")
		}
	}

	// Format output based on whether seconds are enabled
	if b.withSeconds {
		return fmt.Sprintf("%s %s %s %s %s %s",
			b.second, b.minute, b.hour, b.dayOfMonth, b.month, b.dayOfWeek), nil
	} else {
		return fmt.Sprintf("%s %s %s %s %s",
			b.minute, b.hour, b.dayOfMonth, b.month, b.dayOfWeek), nil
	}
}
