package main

import (
    "fmt"
    "strconv"
    "time"

    "github.com/garyburd/redigo/redis"
)

type LongTable map[string]interface{}

// Check if LongTable exists, with an option to fetch the user
func longTableExists(longTable LongTable, fetch bool) (bool, LongTable) {
    // Check if the LongTable exists and retrieve it
	if fetch {
        if longTable, err := getLongTable(longTable); err != nil {
            return false, nil
        } else {
            return true, longTable
        }

    // Just check if the LongTable exists
	} else {
        if ok, err := hasLongTable(longTable); err != nil {
            return false, nil
        } else {
            return ok, nil
        }
	}
}

// Check if LongTable exists
func hasLongTable(longTable LongTable) (bool, error) {
    if reply, err := db.Do("EXISTS", fmt.Sprint("longTable:", longTable["id"])); err != nil {
        return false, err
    } else if count, err := redis.Int(reply, err); err != nil {
        return false, err
    } else {
        return count > 0, nil
    }
}

// Get LongTable with specified parameters
func getLongTable(longTable LongTable) (LongTable, error) {
    if longTableID, ok := longTable["id"]; !ok {
        return longTable, ErrMissingKey
    } else {
        if reply, err := db.Do("HGETALL", fmt.Sprint("longTable:", longTableID)); err != nil {
            return longTable, err
        } else if retrievedLongTable, err := redis.StringMap(reply, err); err != nil {
            return longTable, err
        } else {
            for k, v := range retrievedLongTable {
                switch k {
                case "id":
                    longTableID, err := strconv.ParseUint(v, 10, 64)
                    if err != nil {
                        return longTable, err
                    }
                    longTable[k] = longTableID
                default:
                    longTable[k] = v
                }
            }
        }
    }

    return longTable, nil
}

// Insert LongTable with specified parameters
func insertLongTable(longTable LongTable) (int, error) {
    var longTableID int
    if reply, err := db.Do("INCR", "nextLongTableID"); err != nil {
        return 0, err
    } else if longTableID, err = redis.Int(reply, err); err != nil {
        return 0, err
    }

    var args []interface{}
    args = append(args, fmt.Sprint("longTable:", longTableID))

    now := time.Now().Unix()

    // Set longTable
    longTable["created_at"] = now
    for k, v := range longTable {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return 0, err
    }
    longTable["id"] = longTableID

    // Add longTable to longTables list
    if _, err := db.Do("ZADD", "longTables", now, longTableID); err != nil {
        return 0, err
    }

	return longTableID, nil
}

// Delete LongTable with specified parameters
func deleteLongTable(longTable LongTable) error {
    if _, err := db.Do("DECR", "nextLongTableID"); err != nil {
        return err
    }

    longTableID := longTable["id"]

    // Delete longTable
    if _, err := db.Do("DEL", fmt.Sprint("longTable:", longTableID)); err != nil {
        return err
    }

    // Remove longTable from longTables list
    if _, err := db.Do("ZREM", "longTables", longTableID); err != nil {
        return err
    }

    // Delete longTableBookings from userLongTableBookings
    if _, err := db.Do("DEL", fmt.Sprint("longTableBookings:", longTableID)); err != nil {
        return err
    }

    return nil
}

// Update LongTable with specified parameters
func updateLongTable(longTable LongTable) (err error) {
    var args []interface{}

    if longTableID, ok := longTable["id"]; !ok {
        return ErrTypeAssertionFailed
    } else {
        args = append(args, fmt.Sprint("longTable:", longTableID))
    }

    longTable["updatedAt"] = time.Now().Unix()

    // Update longTable
    for k, v := range longTable {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return err
    }

	return nil
}

// Get LongTables matching specified parameters
func getLongTables(params map[string]interface{}) ([]LongTable, error) {
    count := params["count"].(int)

    return _getLongTables("ZRANGE", "longTables", 0, count - 1)
}

// Get LongTables with specified raw Redis command
func _getLongTables(command string, args ...interface{}) ([]LongTable, error) {
    var longTables []LongTable

    if reply, err := db.Do(command, args...); err != nil {
        return nil, err
    } else if longTableIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        for _, longTableID := range longTableIDs {
            longTable := LongTable{"id": longTableID}
            if _, err = getLongTable(longTable); err != nil {
                return nil, err
            }
            longTables = append(longTables, longTable)
        }
    }

    return longTables, nil
}
