package main

import (
    "fmt"
    "strconv"
    "time"

    "github.com/garyburd/redigo/redis"
)

type User map[string]interface{}

func userExists(user User, fetch bool) (bool, User) {
    // Check if the user exists and retrieve it
	if fetch {
        //
        return true, User{}

    // Just check if the user exists
	} else {
        //
		return true, nil
	}
}

func getUser(user User) (User, error) {
    if userID, ok := user["id"]; !ok {
        return user, ErrKeyNotFound
    } else {
        if reply, err := db.Do("HGETALL", fmt.Sprint("user:", userID)); err != nil {
            return user, err
        } else if retrievedUser, err := redis.StringMap(reply, err); err != nil {
            return user, err
        } else {
            for k, v := range retrievedUser {
                switch k {
                case "id":
                    userID, err := strconv.ParseUint(v, 10, 64)
                    if err != nil {
                        return user, err
                    }
                    user[k] = userID
                default:
                    user[k] = v
                }
            }
        }
    }

    return user, nil
}

func insertUser(user User) (uint64, error) {
    var userID uint64
    if reply, err := db.Do("INCR", "next_user_id"); err != nil {
        return 0, err
    } else if userID, err = redis.Uint64(reply, err); err != nil {
        return 0, err
    }

    var args []interface{}
    args = append(args, fmt.Sprint("user:", userID))

    now := time.Now().Unix()

    // Set user
    user["created_at"] = now
    for k, v := range user {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return 0, err
    }
    user["id"] = userID

    // Add user to users list
    if _, err := db.Do("ZADD", "users", now, userID); err != nil {
        return 0, err
    }

	return userID, nil
}

func deleteUser(user User) error {
    if _, err := db.Do("DECR", "next_user_id"); err != nil {
        return err
    }

    userID := user["id"]

    // Delete user
    if _, err := db.Do("DEL", fmt.Sprint("user:", userID)); err != nil {
        return err
    }

    // Remove user from users list
    if _, err := db.Do("ZREM", "users", userID); err != nil {
        return err
    }

    return nil
}

func updateUser(user User) (err error) {
    var args []interface{}

    if userID, ok := user["id"]; !ok {
        return ErrTypeAssertionFailed
    } else {
        args = append(args, fmt.Sprint("user:", userID))
    }

    user["updated_at"] = time.Now().Unix()

    // Update user
    for k, v := range user {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return err
    }

	return nil
}
