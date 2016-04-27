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
        if user, err := getUser(user); err != nil {
            return false, nil
        } else {
            return true, user
        }

    // Just check if the user exists
	} else {
        if ok, err := hasUser(user); err != nil {
            return false, nil
        } else {
            return ok, nil
        }
	}
}

func hasUser(user User) (bool, error) {
    if reply, err := db.Do("EXISTS", fmt.Sprint("user:", user["id"])); err != nil {
        return false, err
    } else if count, err := redis.Int(reply, err); err != nil {
        return false, err
    } else {
        return count > 0, nil
    }
}

func getUser(user User) (User, error) {
    if userID, ok := user["id"]; !ok {
        return user, ErrMissingKey
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

func insertUser(user User) (int, error) {
    var userID int
    if reply, err := db.Do("INCR", "nextUserID"); err != nil {
        return 0, err
    } else if userID, err = redis.Int(reply, err); err != nil {
        return 0, err
    }

    var args []interface{}
    args = append(args, fmt.Sprint("user:", userID))

    now := time.Now().Unix()

    // Set user
    user["createdAt"] = now
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
    if _, err := db.Do("DECR", "nextUserID"); err != nil {
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

    // Delete longTableBookings from userLongTableBookings
    if _, err := db.Do("DEL", fmt.Sprint("userLongTableBookings:", userID)); err != nil {
        return err
    }

    // Delete userConnections
    if otherUserIDs, err := user.otherUserIDs(); err != nil {
        return err
    } else {
        for _, otherUserID := range otherUserIDs {
            if err = user.removeUser(User{"id" : otherUserID}); err != nil {
                return err
            }
        }
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

    user["updatedAt"] = time.Now().Unix()

    // Update user
    for k, v := range user {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return err
    }

	return nil
}

func getUsers(params map[string]interface{}) ([]User, error) {
    count := int(params["count"].(int))

    if interests, ok := params["interests"].([]string); ok {
        var allUsers []User

        for _, interest := range interests {
            if users, err := _getUsers("ZRANGE", fmt.Sprint("interest:", interest), 0, count - 1); err != nil {
                return nil, err
            } else {
                allUsers = append(allUsers, users...)
            }
        }

        return allUsers, nil
    }

    return _getUsers("ZRANGE", "users", 0, count - 1)
}

func _getUsers(command string, args ...interface{}) ([]User, error) {
    var users []User

    if userIDs, err := _getUserIDs(command, args...); err != nil {
        return nil, err
    } else {
        for _, userID := range userIDs {
            user := User{"id": userID}
            if _, err = getUser(user); err != nil {
                return nil, err
            }
            users = append(users, user)
        }
    }

    return users, nil
}

func _getUserIDs(command string, args ...interface{}) ([]int, error) {
    if reply, err := db.Do(command, args...); err != nil {
        return nil, err
    } else if userIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        return userIDs, nil
    }
}

func (user User) addUser(otherUser User) error {
    now := time.Now().Unix()
    if _, err := db.Do("ZADD", fmt.Sprint("userConnections:", user["id"]), now, otherUser["id"]); err != nil {
        return err
    }
    if _, err := db.Do("ZADD", fmt.Sprint("userConnections:", otherUser["id"]), now, user["id"]); err != nil {
        return err
    }
    return nil
}

func (user User) removeUser(otherUser User) error {
    if _, err := db.Do("ZREM", fmt.Sprint("userConnections:", user["id"]), otherUser["id"]); err != nil {
        return err
    }
    if _, err := db.Do("ZREM", fmt.Sprint("userConnections:", otherUser["id"]), user["id"]); err != nil {
        return err
    }
    return nil
}

func (user User) otherUserIDs() ([]int, error) {
    if reply, err := db.Do("ZRANGE", fmt.Sprint("userConnections:", user["id"]), 0, -1); err != nil {
        return nil, err
    } else if userIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        return userIDs, nil
    }
}
