package main

import (
    "fmt"
    "strconv"
    "time"

    "github.com/garyburd/redigo/redis"
)

type LongTableBooking map[string]interface{}

func longTableBookingExists(longTableBooking LongTableBooking, fetch bool) (bool, LongTableBooking) {
    // Check if the longTableBooking exists and retrieve it
	if fetch {
        if longTableBooking, err := getLongTableBooking(longTableBooking); err != nil {
            return false, nil
        } else {
            return true, longTableBooking
        }

    // Just check if the longTableBooking exists
	} else {
        if ok, err := hasLongTableBooking(longTableBooking); err != nil {
            return false, nil
        } else {
            return ok, nil
        }
	}
}

func hasLongTableBooking(longTableBooking LongTableBooking) (bool, error) {
    if reply, err := db.Do("EXISTS", fmt.Sprint("longTableBooking:", longTableBooking["id"])); err != nil {
        return false, err
    } else if count, err := redis.Int(reply, err); err != nil {
        return false, err
    } else {
        return count > 0, nil
    }
}

func getLongTableBooking(longTableBooking LongTableBooking) (LongTableBooking, error) {
    if longTableBookingID, ok := longTableBooking["id"]; !ok {
        return longTableBooking, ErrMissingKey
    } else {
        if reply, err := db.Do("HGETALL", fmt.Sprint("longTableBooking:", longTableBookingID)); err != nil {
            return longTableBooking, err
        } else if retrievedLongTableBooking, err := redis.StringMap(reply, err); err != nil {
            return longTableBooking, err
        } else {
            for k, v := range retrievedLongTableBooking {
                switch k {
                case "id":
                    longTableBookingID, err := strconv.ParseUint(v, 10, 64)
                    if err != nil {
                        return longTableBooking, err
                    }
                    longTableBooking[k] = longTableBookingID
                default:
                    longTableBooking[k] = v
                }
            }
        }
    }

    return longTableBooking, nil
}

func insertLongTableBooking(longTableBooking LongTableBooking) (int, error) {
    if !checkKeys(longTableBooking, "longTableID", "userID") {
        return 0, ErrMissingKey
    }

    var longTableBookingID int
    if reply, err := db.Do("INCR", "nextLongTableBookingID"); err != nil {
        return 0, err
    } else if longTableBookingID, err = redis.Int(reply, err); err != nil {
        return 0, err
    }

    var args []interface{}
    args = append(args, fmt.Sprint("longTableBooking:", longTableBookingID))

    now := time.Now().Unix()

    // Set longTableBooking
    longTableBooking["createdAt"] = now
    for k, v := range longTableBooking {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return 0, err
    }
    longTableBooking["id"] = longTableBookingID

    // Add longTableBooking to longTableBookings list
    if _, err := db.Do("ZADD", fmt.Sprint("longTableBookings:", longTableBooking["longTableID"]), now, longTableBookingID); err != nil {
        return 0, err
    }

    // Add longTableBooking to userLongTableBookings list
    if _, err := db.Do("ZADD", fmt.Sprint("userLongTableBookings:", longTableBooking["userID"]), now, longTableBookingID); err != nil {
        return 0, err
    }

	return longTableBookingID, nil
}

func deleteLongTableBooking(longTableBooking LongTableBooking) error {
    if !checkKeys(longTableBooking, "longTableID", "userID") {
        return ErrMissingKey
    }

    if _, err := db.Do("DECR", "nextLongTableBookingID"); err != nil {
        return err
    }

    longTableBookingID := longTableBooking["id"]

    // Delete longTableBooking
    if _, err := db.Do("DEL", fmt.Sprint("longTableBooking:", longTableBookingID)); err != nil {
        return err
    }

    // Remove longTableBooking from longTableBookings list
    if _, err := db.Do("ZREM", fmt.Sprint("longTableBookings:", longTableBooking["longTableID"]), longTableBookingID); err != nil {
        return err
    }

    // Remove longTableBooking from userLongTableBookings list
    if _, err := db.Do("ZREM", fmt.Sprint("userLongTableBookings:", longTableBooking["userID"]), longTableBookingID); err != nil {
        return err
    }

    return nil
}

func updateLongTableBooking(longTableBooking LongTableBooking) (err error) {
    var args []interface{}

    if longTableBookingID, ok := longTableBooking["id"]; !ok {
        return ErrTypeAssertionFailed
    } else {
        args = append(args, fmt.Sprint("longTableBooking:", longTableBookingID))
    }

    longTableBooking["updatedAt"] = time.Now().Unix()

    // Update longTableBooking
    for k, v := range longTableBooking {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return err
    }

	return nil
}

func getLongTableBookings(params map[string]interface{}) ([]LongTableBooking, error) {
    count := params["count"].(int)

    if longTableID, ok := params["longTableID"]; ok {
        return _getLongTableBookings("ZRANGE", fmt.Sprint("longTableBookings:", longTableID), 0, count - 1)
    } else if userID, ok := params["userID"]; ok {
        return _getLongTableBookings("ZRANGE", fmt.Sprint("userLongTableBookings:", userID), 0, count - 1)
    }

    return nil, ErrMissingKey
}

func _getLongTableBookings(command string, args ...interface{}) ([]LongTableBooking, error) {
    var longTableBookings []LongTableBooking

    if reply, err := db.Do(command, args...); err != nil {
        return nil, err
    } else if longTableBookingIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        for _, longTableBookingID := range longTableBookingIDs {
            longTableBooking := LongTableBooking{"id": longTableBookingID}
            if _, err = getLongTableBooking(longTableBooking); err != nil {
                return nil, err
            }
            longTableBookings = append(longTableBookings, longTableBooking)
        }
    }

    return longTableBookings, nil
}