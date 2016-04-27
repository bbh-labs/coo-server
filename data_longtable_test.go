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

    // Insert longtable
    longtable := LongTable{
        "name": "Some longtable",
        "num_seats": 40,
    }

    var longtableID uint64
    if longtableID, err = insertLongTable(longtable); err != nil {
        t.Error("insertLongTable:", err)
    }
    longtable["id"] = longtableID

    // Update longtable
    longtable["name"] = "Some longer longtable"
    longtable["num_seats"] = 50
    if err := updateLongTable(longtable); err != nil {
        t.Error("updateLongTable:", err)
    }

    // Get longtable
    if _, err := getLongTable(longtable); err != nil {
        t.Error("getLongTable:", err)
    }

    // Has longtable
    if ok, _ := hasLongTable(longtable); !ok {
        t.Error("hasLongTable")
    }

    // Get longtables
    if longtables, err := getLongTables(map[string]interface{}{"count": 5}); err != nil || len(longtables) < 1 {
        t.Error("getLongTables")
    }

    // Delete longtable
    if err = deleteLongTable(longtable); err != nil {
        t.Error("deleteLongTable:", err)
    }
}
