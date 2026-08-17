package model

import "time"

type TimeSlot struct {
	ID     TimeSlotID
	Day    time.Weekday
	Period int
	Label  string
}

// SlotKey identifies a recurring weekly slot by day and period.
type SlotKey struct {
	Day    time.Weekday
	Period int
}

func (ts TimeSlot) Key() SlotKey {
	return SlotKey{Day: ts.Day, Period: ts.Period}
}
