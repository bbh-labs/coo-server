package main

import (
    "testing"

    "github.com/garyburd/redigo/redis"
)

func TestLongTable(t *testing.T) {
    var err error

    if db, err = redis.Dial("tcp", ":6379"); err != nil {
        t.Error(err)
    }
    defer db.Close()

    // Insert longTable
    longTable := LongTable{
        "name": "Some longTable",
        "numSeats": 40,
    }

    var longTableID int
    if longTableID, err = insertLongTable(longTable); err != nil {
        t.Error("insertLongTable:", err)
    }
    longTable["id"] = longTableID

    // Update longTable
    longTable["name"] = "Some longer longTable"
    longTable["numSeats"] = 50
    if err := updateLongTable(longTable); err != nil {
        t.Error("updateLongTable:", err)
    }

    // Get longTable
    if _, err := getLongTable(longTable); err != nil {
        t.Error("getLongTable:", err)
    }

    // Has longTable
    if ok, _ := hasLongTable(longTable); !ok {
        t.Error("hasLongTable")
    }

    // Get longTables
    if longTables, err := getLongTables(map[string]interface{}{"count": 5}); err != nil || len(longTables) < 1 {
        t.Error("getLongTables")
    }

    // Delete longTable
    if err = deleteLongTable(longTable); err != nil {
        t.Error("deleteLongTable:", err)
    }
}
