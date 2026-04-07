package cronjob

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Constructors ---

func TestNewCronBuilder(t *testing.T) {
	b := NewCronBuilder()
	require.NotNil(t, b)
	assert.False(t, b.withSeconds, "Default builder should not have seconds enabled")
	assert.Equal(t, "*", b.minute)
	assert.Equal(t, "*", b.hour)
	assert.Equal(t, "*", b.dayOfMonth)
	assert.Equal(t, "*", b.month)
	assert.Equal(t, "*", b.dayOfWeek)
	assert.Nil(t, b.err, "Initial error should be nil")

	expr, err := b.Build()
	require.NoError(t, err)
	assert.Equal(t, "* * * * *", expr)
}

func TestNewCronBuilderWithSeconds(t *testing.T) {
	b := NewCronBuilder().EnableSeconds()
	require.NotNil(t, b)
	assert.True(t, b.withSeconds, "Builder with seconds should have seconds enabled")
	assert.Equal(t, "*", b.second) // Default should be '*' when seconds enabled
	assert.Equal(t, "*", b.minute)
	assert.Equal(t, "*", b.hour)
	assert.Equal(t, "*", b.dayOfMonth)
	assert.Equal(t, "*", b.month)
	assert.Equal(t, "*", b.dayOfWeek)
	assert.Nil(t, b.err, "Initial error should be nil")

	expr, err := b.Build()
	require.NoError(t, err)
	assert.Equal(t, "* * * * * *", expr)
}

// --- Test Error Handling ---

func TestErrorPropagation(t *testing.T) {
	// Test that the first error is stored and subsequent calls are no-ops
	b := NewCronBuilder().Minute(-1) // Invalid minute
	require.Error(t, b.err, "Should have an error after invalid input")
	originalError := b.err

	// Try setting a valid value - should not overwrite error or change state
	b.Minute(30)
	assert.Equal(t, originalError, b.err, "Error should not be overwritten")
	assert.Equal(t, "*", b.minute, "Minute field should not change after an error") // It remains '*' because the invalid set failed

	// Try setting another invalid value - should not overwrite the *first* error
	b.Hour(25)
	assert.Equal(t, originalError, b.err, "First error should persist")

	// Build should return the first error
	expr, err := b.Build()
	assert.Equal(t, "", expr, "Expression should be empty on error")
	assert.Equal(t, originalError, err, "Build should return the stored error")
}

func TestSecondsDisabledError(t *testing.T) {
	b := NewCronBuilder() // Seconds disabled
	require.False(t, b.withSeconds)

	tests := []struct {
		name   string
		action func(*CronBuilder) *CronBuilder
	}{
		{"EverySecond", func(cb *CronBuilder) *CronBuilder { return cb.EverySecond() }},
		{"Second", func(cb *CronBuilder) *CronBuilder { return cb.Second(10) }},
		{"Seconds", func(cb *CronBuilder) *CronBuilder { return cb.Seconds(10, 20) }},
		{"SecondRange", func(cb *CronBuilder) *CronBuilder { return cb.SecondRange(5, 15) }},
		{"SecondStep", func(cb *CronBuilder) *CronBuilder { return cb.SecondStep(5) }},
		{"SecondRangeStep", func(cb *CronBuilder) *CronBuilder { return cb.SecondRangeStep(0, 30, 10) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewCronBuilder() // Fresh builder for each test
			builder = tt.action(builder)
			require.Error(t, builder.err)
			assert.Contains(t, builder.err.Error(), "seconds are not enabled")
			_, buildErr := builder.Build()
			require.Error(t, buildErr)
			assert.Contains(t, buildErr.Error(), "seconds are not enabled")
		})
	}
}

// --- Test Field Setters ---

// Helper for validation tests
type fieldTestCase struct {
	name        string
	action      func(b *CronBuilder) *CronBuilder
	expectField string // Expected value of the specific field being tested
	expectError string // Substring of expected error, empty if no error
}

// --- Test Second Field ---

func TestSecondField(t *testing.T) {
	testCases := []fieldTestCase{
		// Valid cases
		{"EverySecond", func(b *CronBuilder) *CronBuilder { return b.EverySecond() }, "*", ""},
		{"Single Second", func(b *CronBuilder) *CronBuilder { return b.Second(0) }, "0", ""},
		{"Mid Second", func(b *CronBuilder) *CronBuilder { return b.Second(30) }, "30", ""},
		{"Max Second", func(b *CronBuilder) *CronBuilder { return b.Second(59) }, "59", ""},
		{"Multiple Seconds", func(b *CronBuilder) *CronBuilder { return b.Seconds(5, 1, 10) }, "1,5,10", ""}, // Check sorting
		{"Second Range", func(b *CronBuilder) *CronBuilder { return b.SecondRange(5, 15) }, "5-15", ""},
		{"Second Step", func(b *CronBuilder) *CronBuilder { return b.SecondStep(10) }, "*/10", ""},
		{"Second Range Step", func(b *CronBuilder) *CronBuilder { return b.SecondRangeStep(10, 40, 5) }, "10-40/5", ""},
		{"Second Full Range Step", func(b *CronBuilder) *CronBuilder { return b.SecondRangeStep(0, 59, 15) }, "0-59/15", ""},

		// Invalid cases
		{"Invalid Second Low", func(b *CronBuilder) *CronBuilder { return b.Second(-1) }, "*", "invalid second: -1"},
		{"Invalid Second High", func(b *CronBuilder) *CronBuilder { return b.Second(60) }, "*", "invalid second: 60"},
		{"Invalid Seconds Low", func(b *CronBuilder) *CronBuilder { return b.Seconds(10, -5, 20) }, "*", "invalid second: -5"},
		{"Invalid Seconds High", func(b *CronBuilder) *CronBuilder { return b.Seconds(10, 65, 20) }, "*", "invalid second: 65"},
		{"Empty Seconds List", func(b *CronBuilder) *CronBuilder { return b.Seconds() }, "*", "seconds list cannot be empty"},
		{"Invalid Second Range Low Start", func(b *CronBuilder) *CronBuilder { return b.SecondRange(-1, 10) }, "*", "invalid second range start: -1"},
		{"Invalid Second Range Low End", func(b *CronBuilder) *CronBuilder { return b.SecondRange(10, -1) }, "*", "invalid second range end: -1"},
		{"Invalid Second Range High Start", func(b *CronBuilder) *CronBuilder { return b.SecondRange(60, 70) }, "*", "invalid second range start: 60"},
		{"Invalid Second Range High End", func(b *CronBuilder) *CronBuilder { return b.SecondRange(50, 60) }, "*", "invalid second range end: 60"},
		{"Invalid Second Range Order", func(b *CronBuilder) *CronBuilder { return b.SecondRange(15, 5) }, "*", "start (15) cannot be greater than end (5)"},
		{"Invalid Second Step Zero", func(b *CronBuilder) *CronBuilder { return b.SecondStep(0) }, "*", "invalid second step: 0"},
		{"Invalid Second Step High", func(b *CronBuilder) *CronBuilder { return b.SecondStep(60) }, "*", "invalid second step: 60"},
		{"Invalid Second Range Step Low", func(b *CronBuilder) *CronBuilder { return b.SecondRangeStep(10, 40, 0) }, "*", "invalid second step for range 10-40: 0"},
		{"Invalid Second Range Step High", func(b *CronBuilder) *CronBuilder { return b.SecondRangeStep(10, 20, 15) }, "*", "invalid second step for range 10-20: 15"}, // Step > range size
		{"Invalid Second Range Step Range", func(b *CronBuilder) *CronBuilder { return b.SecondRangeStep(40, 10, 5) }, "*", "start (40) cannot be greater than end (10)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewCronBuilder().EnableSeconds() // Use builder with seconds enabled
			b = tc.action(b)

			if tc.expectError == "" {
				require.NoError(t, b.err, "Expected no error for valid case")
				assert.Equal(t, tc.expectField, b.second, "Second field value mismatch")
				// Check build result for good measure
				expr, err := b.Build()
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(expr, tc.expectField+" "), "Build expression should start with correct second field")
			} else {
				require.Error(t, b.err, "Expected an error for invalid case")
				assert.Contains(t, b.err.Error(), tc.expectError, "Error message mismatch")
				// Field should remain default '*' if setting failed
				assert.Equal(t, "*", b.second, "Second field should remain default on error")
				// Build should also fail
				_, err := b.Build()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

// --- Test Minute Field ---

func TestMinuteField(t *testing.T) {
	testCases := []fieldTestCase{
		// Valid cases
		{"EveryMinute", func(b *CronBuilder) *CronBuilder { return b.EveryMinute() }, "*", ""},
		{"Single Minute", func(b *CronBuilder) *CronBuilder { return b.Minute(0) }, "0", ""},
		{"Mid Minute", func(b *CronBuilder) *CronBuilder { return b.Minute(30) }, "30", ""},
		{"Max Minute", func(b *CronBuilder) *CronBuilder { return b.Minute(59) }, "59", ""},
		{"Multiple Minutes", func(b *CronBuilder) *CronBuilder { return b.Minutes(5, 1, 10) }, "1,5,10", ""},
		{"Minute Range", func(b *CronBuilder) *CronBuilder { return b.MinuteRange(5, 15) }, "5-15", ""},
		{"Minute Step", func(b *CronBuilder) *CronBuilder { return b.MinuteStep(15) }, "*/15", ""},
		{"Minute Range Step", func(b *CronBuilder) *CronBuilder { return b.MinuteRangeStep(0, 30, 10) }, "0-30/10", ""},

		// Invalid cases
		{"Invalid Minute Low", func(b *CronBuilder) *CronBuilder { return b.Minute(-1) }, "*", "invalid minute: -1"},
		{"Invalid Minute High", func(b *CronBuilder) *CronBuilder { return b.Minute(60) }, "*", "invalid minute: 60"},
		{"Invalid Minutes List", func(b *CronBuilder) *CronBuilder { return b.Minutes(10, 60) }, "*", "invalid minute: 60"},
		{"Empty Minutes List", func(b *CronBuilder) *CronBuilder { return b.Minutes() }, "*", "minutes list cannot be empty"},
		{"Invalid Minute Range Order", func(b *CronBuilder) *CronBuilder { return b.MinuteRange(20, 10) }, "*", "start (20) cannot be greater than end (10)"},
		{"Invalid Minute Step Zero", func(b *CronBuilder) *CronBuilder { return b.MinuteStep(0) }, "*", "invalid minute step: 0"},
		{"Invalid Minute Step High", func(b *CronBuilder) *CronBuilder { return b.MinuteStep(60) }, "*", "invalid minute step: 60"},
		{"Invalid Minute Range Step Range", func(b *CronBuilder) *CronBuilder { return b.MinuteRangeStep(30, 0, 5) }, "*", "start (30) cannot be greater than end (0)"},
		{"Invalid Minute Range Step Step", func(b *CronBuilder) *CronBuilder { return b.MinuteRangeStep(0, 10, 11) }, "*", "invalid minute step for range 0-10: 11"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewCronBuilder() // Use standard builder
			b = tc.action(b)

			if tc.expectError == "" {
				require.NoError(t, b.err)
				assert.Equal(t, tc.expectField, b.minute)
				expr, err := b.Build()
				require.NoError(t, err)
				parts := strings.Fields(expr)
				require.Len(t, parts, 5)
				assert.Equal(t, tc.expectField, parts[0]) // Minute is the first field
			} else {
				require.Error(t, b.err)
				assert.Contains(t, b.err.Error(), tc.expectError)
				_, err := b.Build()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

// --- Test Hour Field ---

func TestHourField(t *testing.T) {
	testCases := []fieldTestCase{
		// Valid cases
		{"EveryHour", func(b *CronBuilder) *CronBuilder { return b.EveryHour() }, "*", ""},
		{"Single Hour", func(b *CronBuilder) *CronBuilder { return b.Hour(0) }, "0", ""},
		{"Mid Hour", func(b *CronBuilder) *CronBuilder { return b.Hour(12) }, "12", ""},
		{"Max Hour", func(b *CronBuilder) *CronBuilder { return b.Hour(23) }, "23", ""},
		{"Multiple Hours", func(b *CronBuilder) *CronBuilder { return b.Hours(8, 20, 0) }, "0,8,20", ""},
		{"Hour Range", func(b *CronBuilder) *CronBuilder { return b.HourRange(9, 17) }, "9-17", ""},
		{"Hour Step", func(b *CronBuilder) *CronBuilder { return b.HourStep(6) }, "*/6", ""},
		{"Hour Range Step", func(b *CronBuilder) *CronBuilder { return b.HourRangeStep(8, 18, 2) }, "8-18/2", ""},

		// Invalid cases
		{"Invalid Hour Low", func(b *CronBuilder) *CronBuilder { return b.Hour(-1) }, "*", "invalid hour: -1"},
		{"Invalid Hour High", func(b *CronBuilder) *CronBuilder { return b.Hour(24) }, "*", "invalid hour: 24"},
		{"Invalid Hours List", func(b *CronBuilder) *CronBuilder { return b.Hours(8, 24) }, "*", "invalid hour: 24"},
		{"Empty Hours List", func(b *CronBuilder) *CronBuilder { return b.Hours() }, "*", "hours list cannot be empty"},
		{"Invalid Hour Range Order", func(b *CronBuilder) *CronBuilder { return b.HourRange(17, 9) }, "*", "start (17) cannot be greater than end (9)"},
		{"Invalid Hour Step Zero", func(b *CronBuilder) *CronBuilder { return b.HourStep(0) }, "*", "invalid hour step: 0"},
		{"Invalid Hour Step High", func(b *CronBuilder) *CronBuilder { return b.HourStep(24) }, "*", "invalid hour step: 24"},
		{"Invalid Hour Range Step Range", func(b *CronBuilder) *CronBuilder { return b.HourRangeStep(18, 8, 2) }, "*", "start (18) cannot be greater than end (8)"},
		{"Invalid Hour Range Step Step", func(b *CronBuilder) *CronBuilder { return b.HourRangeStep(9, 12, 5) }, "*", "invalid hour step for range 9-12: 5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewCronBuilder()
			b = tc.action(b)

			if tc.expectError == "" {
				require.NoError(t, b.err)
				assert.Equal(t, tc.expectField, b.hour)
				expr, err := b.Build()
				require.NoError(t, err)
				parts := strings.Fields(expr)
				require.Len(t, parts, 5)
				assert.Equal(t, tc.expectField, parts[1]) // Hour is the second field
			} else {
				require.Error(t, b.err)
				assert.Contains(t, b.err.Error(), tc.expectError)
				_, err := b.Build()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

// --- Test DayOfMonth Field ---

func TestDayOfMonthField(t *testing.T) {
	testCases := []fieldTestCase{
		// Valid cases
		{"EveryDayOfMonth", func(b *CronBuilder) *CronBuilder { return b.EveryDayOfMonth() }, "*", ""},
		{"Min DayOfMonth", func(b *CronBuilder) *CronBuilder { return b.DayOfMonth(1) }, "1", ""},
		{"Mid DayOfMonth", func(b *CronBuilder) *CronBuilder { return b.DayOfMonth(15) }, "15", ""},
		{"Max DayOfMonth", func(b *CronBuilder) *CronBuilder { return b.DayOfMonth(31) }, "31", ""},
		{"Multiple DaysOfMonth", func(b *CronBuilder) *CronBuilder { return b.DaysOfMonth(1, 15, 31) }, "1,15,31", ""},
		{"DayOfMonth Range", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthRange(10, 20) }, "10-20", ""},
		{"DayOfMonth Step", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthStep(7) }, "*/7", ""},
		{"DayOfMonth Range Step", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthRangeStep(1, 15, 5) }, "1-15/5", ""},

		// Invalid cases
		{"Invalid DayOfMonth Low", func(b *CronBuilder) *CronBuilder { return b.DayOfMonth(0) }, "*", "invalid day of month: 0"},
		{"Invalid DayOfMonth High", func(b *CronBuilder) *CronBuilder { return b.DayOfMonth(32) }, "*", "invalid day of month: 32"},
		{"Invalid DaysOfMonth List", func(b *CronBuilder) *CronBuilder { return b.DaysOfMonth(1, 0) }, "*", "invalid day of month: 0"},
		{"Empty DaysOfMonth List", func(b *CronBuilder) *CronBuilder { return b.DaysOfMonth() }, "*", "days of month list cannot be empty"},
		{"Invalid DayOfMonth Range Order", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthRange(20, 10) }, "*", "start (20) cannot be greater than end (10)"},
		{"Invalid DayOfMonth Step Zero", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthStep(0) }, "*", "invalid day of month step: 0"},
		{"Invalid DayOfMonth Step High", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthStep(32) }, "*", "invalid day of month step: 32"},
		{"Invalid DayOfMonth Range Step Range", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthRangeStep(15, 1, 5) }, "*", "start (15) cannot be greater than end (1)"},
		{"Invalid DayOfMonth Range Step Step", func(b *CronBuilder) *CronBuilder { return b.DayOfMonthRangeStep(1, 10, 11) }, "*", "invalid day of month step for range 1-10: 11"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewCronBuilder()
			b = tc.action(b)

			if tc.expectError == "" {
				require.NoError(t, b.err)
				assert.Equal(t, tc.expectField, b.dayOfMonth)
				expr, err := b.Build()
				require.NoError(t, err)
				parts := strings.Fields(expr)
				require.Len(t, parts, 5)
				assert.Equal(t, tc.expectField, parts[2]) // DayOfMonth is the third field
			} else {
				require.Error(t, b.err)
				assert.Contains(t, b.err.Error(), tc.expectError)
				_, err := b.Build()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

// --- Test Month Field ---

func TestMonthField(t *testing.T) {
	testCases := []fieldTestCase{
		// Valid cases
		{"EveryMonth", func(b *CronBuilder) *CronBuilder { return b.EveryMonth() }, "*", ""},
		{"Min Month", func(b *CronBuilder) *CronBuilder { return b.Month(1) }, "1", ""},
		{"Mid Month", func(b *CronBuilder) *CronBuilder { return b.Month(6) }, "6", ""},
		{"Max Month", func(b *CronBuilder) *CronBuilder { return b.Month(12) }, "12", ""},
		{"Multiple Months", func(b *CronBuilder) *CronBuilder { return b.Months(1, 6, 12) }, "1,6,12", ""},
		{"Month Range", func(b *CronBuilder) *CronBuilder { return b.MonthRange(3, 5) }, "3-5", ""},
		{"Month Step", func(b *CronBuilder) *CronBuilder { return b.MonthStep(2) }, "*/2", ""},
		{"Month Range Step", func(b *CronBuilder) *CronBuilder { return b.MonthRangeStep(1, 6, 3) }, "1-6/3", ""},

		// Invalid cases
		{"Invalid Month Low", func(b *CronBuilder) *CronBuilder { return b.Month(0) }, "*", "invalid month: 0"},
		{"Invalid Month High", func(b *CronBuilder) *CronBuilder { return b.Month(13) }, "*", "invalid month: 13"},
		{"Invalid Months List", func(b *CronBuilder) *CronBuilder { return b.Months(1, 13) }, "*", "invalid month: 13"},
		{"Empty Months List", func(b *CronBuilder) *CronBuilder { return b.Months() }, "*", "months list cannot be empty"},
		{"Invalid Month Range Order", func(b *CronBuilder) *CronBuilder { return b.MonthRange(5, 3) }, "*", "start (5) cannot be greater than end (3)"},
		{"Invalid Month Step Zero", func(b *CronBuilder) *CronBuilder { return b.MonthStep(0) }, "*", "invalid month step: 0"},
		{"Invalid Month Step High", func(b *CronBuilder) *CronBuilder { return b.MonthStep(13) }, "*", "invalid month step: 13"},
		{"Invalid Month Range Step Range", func(b *CronBuilder) *CronBuilder { return b.MonthRangeStep(6, 1, 3) }, "*", "start (6) cannot be greater than end (1)"},
		{"Invalid Month Range Step Step", func(b *CronBuilder) *CronBuilder { return b.MonthRangeStep(1, 6, 7) }, "*", "invalid month step for range 1-6: 7"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewCronBuilder()
			b = tc.action(b)

			if tc.expectError == "" {
				require.NoError(t, b.err)
				assert.Equal(t, tc.expectField, b.month)
				expr, err := b.Build()
				require.NoError(t, err)
				parts := strings.Fields(expr)
				require.Len(t, parts, 5)
				assert.Equal(t, tc.expectField, parts[3]) // Month is the fourth field
			} else {
				require.Error(t, b.err)
				assert.Contains(t, b.err.Error(), tc.expectError)
				_, err := b.Build()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

// --- Test DayOfWeek Field ---

func TestDayOfWeekField(t *testing.T) {
	testCases := []fieldTestCase{
		// Valid cases
		{"EveryDayOfWeek", func(b *CronBuilder) *CronBuilder { return b.EveryDayOfWeek() }, "*", ""},
		{"Min DayOfWeek (Sun)", func(b *CronBuilder) *CronBuilder { return b.DayOfWeek(0) }, "0", ""},
		{"Mid DayOfWeek (Wed)", func(b *CronBuilder) *CronBuilder { return b.DayOfWeek(3) }, "3", ""},
		{"Max DayOfWeek (Sat)", func(b *CronBuilder) *CronBuilder { return b.DayOfWeek(6) }, "6", ""},
		{"Sunday as 7", func(b *CronBuilder) *CronBuilder { return b.DayOfWeek(7) }, "0", ""},                            // 7 maps to 0
		{"Multiple DaysOfWeek", func(b *CronBuilder) *CronBuilder { return b.DaysOfWeek(1, 5) }, "1,5", ""},              // Mon, Fri
		{"Multiple DaysOfWeek with 7", func(b *CronBuilder) *CronBuilder { return b.DaysOfWeek(7, 3, 0) }, "0,3", ""},    // Sun, Wed (duplicates removed, 7->0)
		{"DayOfWeek Range", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRange(1, 5) }, "1-5", ""},              // Mon-Fri
		{"DayOfWeek Range with 7 Start", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRange(7, 2) }, "0-2", ""}, // Sun-Tue
		{"DayOfWeek Range with 7 End", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRange(5, 7) }, "5-0", ""},   // Fri-Sun (Note: range is 5-0 not 5-6,0) - CRON standard behavior
		{"DayOfWeek Step", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekStep(2) }, "*/2", ""},
		{"DayOfWeek Range Step", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRangeStep(1, 5, 2) }, "1-5/2", ""},        // Mon, Wed, Fri
		{"DayOfWeek Range Step with 7", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRangeStep(7, 4, 2) }, "0-4/2", ""}, // Sun, Tue, Thu

		// Invalid cases
		{"Invalid DayOfWeek Low", func(b *CronBuilder) *CronBuilder { return b.DayOfWeek(-1) }, "*", "invalid day of week: -1"},
		{"Invalid DayOfWeek High", func(b *CronBuilder) *CronBuilder { return b.DayOfWeek(8) }, "*", "invalid day of week: 8"},
		{"Invalid DaysOfWeek List Low", func(b *CronBuilder) *CronBuilder { return b.DaysOfWeek(1, -1) }, "*", "invalid day of week in list: -1"},
		{"Invalid DaysOfWeek List High", func(b *CronBuilder) *CronBuilder { return b.DaysOfWeek(1, 8) }, "*", "invalid day of week in list: 8"},
		{"Empty DaysOfWeek List", func(b *CronBuilder) *CronBuilder { return b.DaysOfWeek() }, "*", "days of week list cannot be empty"},
		{"Invalid DayOfWeek Range Low Start", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRange(-1, 5) }, "*", "invalid day of week in range start: -1"},
		{"Invalid DayOfWeek Range High End", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRange(1, 8) }, "*", "invalid day of week in range end: 8"},
		// Note: start > end is allowed for DayOfWeek ranges in cron (e.g., 5-2 means Fri, Sat, Sun, Mon, Tue) - We follow the builder's validation which expects 0-6 range for start/end indices after mapping 7->0.
		// The builder currently formats it like "5-2" if you pass DayOfWeekRange(5, 2). This is standard cron. Let's test the validation part.
		{"Invalid DayOfWeek Step Zero", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekStep(0) }, "*", "invalid day of week step: 0"},
		{"Invalid DayOfWeek Step High", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekStep(8) }, "*", "invalid day of week step: 8"},
		{"Invalid DayOfWeek Range Step Range Low", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRangeStep(-1, 5, 2) }, "*", "invalid day of week in range start: -1"},
		{"Invalid DayOfWeek Range Step Range High", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRangeStep(1, 8, 2) }, "*", "invalid day of week in range end: 8"},
		{"Invalid DayOfWeek Range Step Step Zero", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRangeStep(1, 5, 0) }, "*", "invalid day of week step for range 1-5: 0"},
		{"Invalid DayOfWeek Range Step Step High", func(b *CronBuilder) *CronBuilder { return b.DayOfWeekRangeStep(1, 5, 7) }, "*", "invalid day of week step for range 1-5: 7"}, // Step > range size (5-1+1 = 5) -> error (step <= 7 is allowed by raw validation, but range step logic checks step <= range size) - Let's assume step must be <= 7 for general validity. This test uses 7 explicitly.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewCronBuilder()
			b = tc.action(b)

			if tc.expectError == "" {
				require.NoError(t, b.err)
				assert.Equal(t, tc.expectField, b.dayOfWeek)
				expr, err := b.Build()
				require.NoError(t, err)
				parts := strings.Fields(expr)
				require.Len(t, parts, 5)
				assert.Equal(t, tc.expectField, parts[4]) // DayOfWeek is the fifth field
			} else {
				require.Error(t, b.err)
				assert.Contains(t, b.err.Error(), tc.expectError, fmt.Sprintf("Full error: %v", b.err))
				_, err := b.Build()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

// --- Test Build Method ---

func TestBuild(t *testing.T) {
	t.Run("Standard 5-Field Build", func(t *testing.T) {
		b := NewCronBuilder().
			Minute(0).
			Hour(9).
			DayOfMonth(1).
			Month(1).    // January
			DayOfWeek(1) // Monday

		expr, err := b.Build()
		require.NoError(t, err)
		assert.Equal(t, "0 9 1 1 1", expr)
	})

	t.Run("With Seconds 6-Field Build", func(t *testing.T) {
		b := NewCronBuilder().
			EnableSeconds().
			Second(30).
			MinuteStep(15). // */15
			HourRange(9, 17).
			EveryDayOfMonth().   // *
			Months(1, 7).        // Jan, Jul
			DayOfWeekRange(1, 5) // Mon-Fri

		expr, err := b.Build()
		require.NoError(t, err)
		assert.Equal(t, "30 */15 9-17 * 1,7 1-5", expr)
	})

	t.Run("Build With Intermediate Error", func(t *testing.T) {
		b := NewCronBuilder().
			Minute(0).
			Hour(25).     // Invalid hour
			DayOfMonth(1) // This call should be ignored

		expr, err := b.Build()
		require.Error(t, err)
		assert.Equal(t, "", expr)
		assert.Contains(t, err.Error(), "invalid hour: 25")
	})

	t.Run("Build With Seconds With Intermediate Error", func(t *testing.T) {
		b := NewCronBuilder().
			EnableSeconds().
			Second(0).
			Minute(-5). // Invalid minute
			Hour(10)    // This call should be ignored

		expr, err := b.Build()
		require.Error(t, err)
		assert.Equal(t, "", expr)
		assert.Contains(t, err.Error(), "invalid minute: -5")
	})

	t.Run("Build Default Standard", func(t *testing.T) {
		expr, err := NewCronBuilder().Build()
		require.NoError(t, err)
		assert.Equal(t, "* * * * *", expr)
	})

	t.Run("Build Default With Seconds", func(t *testing.T) {
		expr, err := NewCronBuilder().EnableSeconds().Build()
		require.NoError(t, err)
		assert.Equal(t, "* * * * * *", expr) // Seconds field defaults to *
	})

	t.Run("Complex Chain Standard", func(t *testing.T) {
		b := NewCronBuilder().
			Minutes(0, 15, 30, 45).
			HourStep(2).
			EveryDayOfMonth().
			MonthRange(3, 6). // Mar-Jun
			DayOfWeek(0)      // Sunday

		expr, err := b.Build()
		require.NoError(t, err)
		assert.Equal(t, "0,15,30,45 */2 * 3-6 0", expr)
	})

	t.Run("Complex Chain With Seconds", func(t *testing.T) {
		b := NewCronBuilder().
			EnableSeconds().
			SecondRangeStep(0, 59, 5). // 0-59/5
			EveryMinute().
			Hour(23).
			DaysOfMonth(1, 15).
			EveryMonth().
			DayOfWeekRange(6, 7) // Sat, Sun (becomes 6-0)

		expr, err := b.Build()
		require.NoError(t, err)
		assert.Equal(t, "0-59/5 * 23 1,15 * 6-0", expr)
	})
}

// --- Test joinInts helper (though implicitly tested above) ---
// It's good practice to test helpers if they have non-trivial logic.
func TestJoinInts(t *testing.T) {
	assert.Equal(t, "1,5,10", joinInts([]int{10, 1, 5}))
	assert.Equal(t, "0", joinInts([]int{0}))
	assert.Equal(t, "1,2,3", joinInts([]int{1, 2, 3}))
	assert.Equal(t, "7,42", joinInts([]int{42, 7}))
	assert.Equal(t, "", joinInts([]int{})) // Test empty slice
}

// --- Test String() and Validate() methods ---

func TestCronBuilder_String(t *testing.T) {
	t.Run("String method returns expression", func(t *testing.T) {
		b := NewCronBuilder().Minute(0).Hour(9)
		assert.Equal(t, "0 9 * * *", b.String())
	})

	t.Run("String method with error", func(t *testing.T) {
		b := NewCronBuilder().Minute(-1)
		assert.Contains(t, b.String(), "error:")
	})
}

func TestCronBuilder_Validate(t *testing.T) {
	t.Run("Validate returns nil for valid config", func(t *testing.T) {
		b := NewCronBuilder().Minute(30)
		assert.NoError(t, b.Validate())
	})

	t.Run("Validate returns error for invalid config", func(t *testing.T) {
		b := NewCronBuilder().Minute(-1)
		assert.Error(t, b.Validate())
	})
}

// --- Test MonthByName ---

func TestCronBuilder_MonthByName(t *testing.T) {
	tests := []struct {
		name      string
		monthName string
		expected  string
		wantErr   bool
	}{
		{"January full", "January", "* * * 1 *", false},
		{"January short", "Jan", "* * * 1 *", false},
		{"February full", "February", "* * * 2 *", false},
		{"Feb short", "Feb", "* * * 2 *", false},
		{"December full", "December", "* * * 12 *", false},
		{"Dec short", "Dec", "* * * 12 *", false},
		{"case insensitive uppercase", "JANUARY", "* * * 1 *", false},
		{"case insensitive mixed", "JaNuArY", "* * * 1 *", false},
		{"invalid name", "NotAMonth", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewCronBuilder().MonthByName(tt.monthName)
			if tt.wantErr {
				assert.Error(t, b.Validate())
			} else {
				expr, err := b.Build()
				require.NoError(t, err)
				assert.Equal(t, tt.expected, expr)
			}
		})
	}

	t.Run("Multiple months using Months", func(t *testing.T) {
		// MonthByName sets a single value, use Months() for multiple values
		b := NewCronBuilder().Months(1, 12)
		expr, err := b.Build()
		require.NoError(t, err)
		assert.Equal(t, "* * * 1,12 *", expr)
	})
}

// --- Test DayOfWeekByName ---

func TestCronBuilder_DayOfWeekByName(t *testing.T) {
	tests := []struct {
		name     string
		dayName  string
		expected string
		wantErr  bool
	}{
		{"Sunday full", "Sunday", "* * * * 0", false},
		{"Sun short", "Sun", "* * * * 0", false},
		{"Monday full", "Monday", "* * * * 1", false},
		{"Mon short", "Mon", "* * * * 1", false},
		{"Saturday full", "Saturday", "* * * * 6", false},
		{"Sat short", "Sat", "* * * * 6", false},
		{"case insensitive uppercase", "MONDAY", "* * * * 1", false},
		{"case insensitive mixed", "MoNdAy", "* * * * 1", false},
		{"invalid name", "NotADay", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewCronBuilder().DayOfWeekByName(tt.dayName)
			if tt.wantErr {
				assert.Error(t, b.Validate())
			} else {
				expr, err := b.Build()
				require.NoError(t, err)
				assert.Equal(t, tt.expected, expr)
			}
		})
	}

	t.Run("Multiple days using DaysOfWeek", func(t *testing.T) {
		// DayOfWeekByName sets a single value, use DaysOfWeek() for multiple values
		b := NewCronBuilder().DaysOfWeek(1, 5)
		expr, err := b.Build()
		require.NoError(t, err)
		assert.Equal(t, "* * * * 1,5", expr)
	})
}
