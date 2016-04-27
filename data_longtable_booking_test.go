package main

import (
    "testing"

    "github.com/garyburd/redigo/redis"
)

func TestLongTableBooking(t *testing.T) {
    var err error

    if db, err = redis.Dial("tcp", ":6379"); err != nil {
        t.Error(err)
    }
    defer db.Close()

    // Insert longTableBooking
    longTableBooking := LongTableBooking{
        "longTableID": 1000,
        "userID": 2000,
        "seatPosition": 20,
    }

    var longTableBookingID int
    if longTableBookingID, err = insertLongTableBooking(longTableBooking); err != nil {
        t.Error("insertLongTableBooking:", err)
    }
    longTableBooking["id"] = longTableBookingID

    // Update longTableBooking
    longTableBooking["seatPosition"] = 25
    if err := updateLongTableBooking(longTableBooking); err != nil {
        t.Error("updateLongTableBooking:", err)
    }

    // Get longTableBooking
    if _, err := getLongTableBooking(longTableBooking); err != nil {
        t.Error("getLongTableBooking:", err)
    }

    // Has longTableBooking
    if ok, _ := hasLongTableBooking(longTableBooking); !ok {
        t.Error("hasLongTableBooking")
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
    if err = deleteLongTableBooking(longTableBooking); err != nil {
        t.Error("deleteLongTableBooking:", err)
    }
}
