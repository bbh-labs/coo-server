package main

import (
    "fmt"
    "strconv"
    "time"

    "github.com/garyburd/redigo/redis"
)

type LongTable map[string]interface{}

func longtableExists(longtable LongTable, fetch bool) (bool, LongTable) {
    // Check if the longtable exists and retrieve it
	if fetch {
        if longtable, err := getLongTable(longtable); err != nil {
            return false, nil
        } else {
            return true, longtable
        }

    // Just check if the longtable exists
	} else {
        if ok, err := hasLongTable(longtable); err != nil {
            return false, nil
        } else {
            return ok, nil
        }
	}
}

func hasLongTable(longtable LongTable) (bool, error) {
    if reply, err := db.Do("EXISTS", fmt.Sprint("longtable:", longtable["id"])); err != nil {
        return false, err
    } else if count, err := redis.Int(reply, err); err != nil {
        return false, err
    } else {
        return count > 0, nil
    }
}

func getLongTable(longtable LongTable) (LongTable, error) {
    if longtableID, ok := longtable["id"]; !ok {
        return longtable, ErrKeyNotFound
    } else {
        if reply, err := db.Do("HGETALL", fmt.Sprint("longtable:", longtableID)); err != nil {
            return longtable, err
        } else if retrievedLongTable, err := redis.StringMap(reply, err); err != nil {
            return longtable, err
        } else {
            for k, v := range retrievedLongTable {
                switch k {
                case "id":
                    longtableID, err := strconv.ParseUint(v, 10, 64)
                    if err != nil {
                        return longtable, err
                    }
                    longtable[k] = longtableID
                default:
                    longtable[k] = v
                }
            }
        }
    }

    return longtable, nil
}

func insertLongTable(longtable LongTable) (uint64, error) {
    var longtableID uint64
    if reply, err := db.Do("INCR", "next_longtable_id"); err != nil {
        return 0, err
    } else if longtableID, err = redis.Uint64(reply, err); err != nil {
        return 0, err
    }

    var args []interface{}
    args = append(args, fmt.Sprint("longtable:", longtableID))

    now := time.Now().Unix()

    // Set longtable
    longtable["created_at"] = now
    for k, v := range longtable {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return 0, err
    }
    longtable["id"] = longtableID

    // Add longtable to longtables list
    if _, err := db.Do("ZADD", "longtables", now, longtableID); err != nil {
        return 0, err
    }

	return longtableID, nil
}

func deleteLongTable(longtable LongTable) error {
    if _, err := db.Do("DECR", "next_longtable_id"); err != nil {
        return err
    }

    longtableID := longtable["id"]

    // Delete longtable
    if _, err := db.Do("DEL", fmt.Sprint("longtable:", longtableID)); err != nil {
        return err
    }

    // Remove longtable from longtables list
    if _, err := db.Do("ZREM", "longtables", longtableID); err != nil {
        return err
    }

    return nil
}

func updateLongTable(longtable LongTable) (err error) {
    var args []interface{}

    if longtableID, ok := longtable["id"]; !ok {
        return ErrTypeAssertionFailed
    } else {
        args = append(args, fmt.Sprint("longtable:", longtableID))
    }

    longtable["updated_at"] = time.Now().Unix()

    // Update longtable
    for k, v := range longtable {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return err
    }

	return nil
}

func getLongTables(params map[string]interface{}) ([]LongTable, error) {
    var count uint64 = uint64(params["count"].(int))

    return _getLongTables("ZRANGE", "longtables", 0, count - 1)
}

func _getLongTables(command string, args ...interface{}) ([]LongTable, error) {
    var longtables []LongTable

    if reply, err := db.Do(command, args...); err != nil {
        return nil, err
    } else if longtableIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        for _, longtableID := range longtableIDs {
            longtable := LongTable{"id": longtableID}
            if _, err = getLongTable(longtable); err != nil {
                return nil, err
            }
            longtables = append(longtables, longtable)
        }
    }

    return longtables, nil
}
