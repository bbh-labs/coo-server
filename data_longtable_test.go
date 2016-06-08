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
		"name":     "Some longTable",
		"numSeats": 40,
	}

	var longTableID int
	if longTableID, err = longTable.insert(); err != nil {
		t.Error("LongTable.insert:", err)
	}
	longTable["id"] = longTableID

	// Update longTable
	longTable["name"] = "Some longer longTable"
	longTable["numSeats"] = 50
	if err := longTable.update(); err != nil {
		t.Error("LongTable.update:", err)
	}

	// Get longTable
	if _, err := longTable.fetch(); err != nil {
		t.Error("LongTable.fetch:", err)
	}

	// Has longTable
	if ok, _ := longTable._exists(); !ok {
		t.Error("LongTable._exists")
	}

	// Get longTables
	if longTables, err := getLongTables(map[string]interface{}{"count": 5}); err != nil || len(longTables) < 1 {
		t.Error("getLongTables")
	}

	// Delete longTable
	if err = longTable.delete(); err != nil {
		t.Error("LongTable.delete:", err)
	}
}
