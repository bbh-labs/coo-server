package main

import (
    "testing"
    "time"

    "github.com/garyburd/redigo/redis"
)

func TestLongTableBooking(t *testing.T) {
    var err error

    if db, err = redis.Dial("tcp", ":6379"); err != nil {
        t.Error(err)
    }
    defer db.Close()

    date := time.Now().Format(DateFormat)

    // Insert longTableBooking
    longTableBooking := LongTableBooking{
        "longTableID": 1000,
        "userID": 2000,
        "seatPosition": 20,
        "date": date,
    }

    var longTableBookingID int
    if longTableBookingID, err = longTableBooking.insert(); err != nil {
        t.Error("LongTableBooking.insert:", err)
    }
    longTableBooking["id"] = longTableBookingID

    // Update longTableBooking
    longTableBooking["seatPosition"] = 25
    if err := longTableBooking.update(); err != nil {
        t.Error("LongTableBooking.update:", err)
    }

    // Fetch longTableBooking
    if _, err := longTableBooking.fetch(); err != nil {
        t.Error("LongTableBooking.fetch:", err)
    }

    // Has longTableBooking
    if ok, _ := longTableBooking._exists(); !ok {
        t.Error("LongTableBooking._exists")
    }

    // Get longTableBookings by longTableID
    if longTableBookings, err := getLongTableBookings(map[string]interface{}{"longTableID": 1000, "count": 5}); err != nil || len(longTableBookings) < 1 {
        t.Error("getLongTableBookings:", err)
    }

    // Get longTableBookings by userID
    if longTableBookings, err := getLongTableBookings(map[string]interface{}{"userID": 2000, "count": 5}); err != nil || len(longTableBookings) < 1 {
        t.Error("getLongTableBookings:", err)
    }

    // Delete longTableBooking
    if err = longTableBooking.delete(); err != nil {
        t.Error("LongTableBooking.delete:", err)
    }
}
