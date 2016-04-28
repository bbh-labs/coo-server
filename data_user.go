package main

import (
    "fmt"
    "strconv"
    "time"

    "github.com/garyburd/redigo/redis"
)

type User map[string]interface{}

// Check if User exists, with an option to fetch the user
func userExists(user User, fetch bool) (bool, User) {
    // Check if the User exists and retrieve it
	if fetch {
        if user, err := getUser(user); err != nil {
            return false, nil
        } else {
            return true, user
        }

    // Just check if the User exists
	} else {
        if ok, err := hasUser(user); err != nil {
            return false, nil
        } else {
            return ok, nil
        }
	}
}

// Check if User exists
func hasUser(user User) (bool, error) {
    if reply, err := db.Do("EXISTS", fmt.Sprint("user:", user["id"])); err != nil {
        return false, err
    } else if count, err := redis.Int(reply, err); err != nil {
        return false, err
    } else {
        return count > 0, nil
    }
}

// Get User with specified parameters
func getUser(user User) (User, error) {
    var err error

    ok, key := hasKey(user, "id", "email")
    if !ok {
        return nil, ErrMissingKey
    }

    switch key {
    case "id":
        user, err = _getUser("HGETALL", fmt.Sprint("user:", user["id"]))
    case "email":
        var reply interface{}
        var userID int

        if reply, err = db.Do("GET", fmt.Sprint("user:email:", user["email"])); err != nil {
            return nil, err
        } else if userID, err = redis.Int(reply, err); err != nil {
            return nil, err
        } else {
            user, err = _getUser("HGETALL", fmt.Sprint("user:", userID))
        }
    }
    if err != nil {
        return nil, err
    }

    if interests, err := user.interests(); err != nil {
        return nil, err
    } else {
        user["interests"] = interests
    }

    return user, nil
}

// Insert User with specified parameters
func insertUser(user User) (int, error) {
    if ok, _ := hasKey(user, "email"); !ok {
        return 0, ErrMissingKey
    }

    var userID int
    if reply, err := db.Do("INCR", "nextUserID"); err != nil {
        return 0, err
    } else if userID, err = redis.Int(reply, err); err != nil {
        return 0, err
    }
    user["id"] = userID

    var args []interface{}
    args = append(args, fmt.Sprint("user:", userID))

    now := time.Now().Unix()

    // Set User
    user["createdAt"] = now
    for k, v := range user {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return 0, err
    }

    // Add User to users list
    if _, err := db.Do("ZADD", "users", now, userID); err != nil {
        return 0, err
    }

    // Add User email reference
    if err := user.setEmailReference(""); err != nil {
        return 0, err
    }

    // Update User interests if exist
    if interests, ok := user["interests"]; ok {
        if interests, ok := interests.([]string); ok {
            if err := user.setInterests(interests); err != nil {
                return 0, err
            }
        }
    }

	return userID, nil
}

// Delete User with specified parameters
func deleteUser(user User) error {
    if ok, _ := hasKey(user, "email"); !ok {
        return ErrMissingKey
    }

    if _, err := db.Do("DECR", "nextUserID"); err != nil {
        return err
    }

    userID := user["id"]

    // Delete User
    if _, err := db.Do("DEL", fmt.Sprint("user:", userID)); err != nil {
        return err
    }

    // Remove User from users list
    if _, err := db.Do("ZREM", "users", userID); err != nil {
        return err
    }

    // Remove User email reference
    if err := user.deleteEmailReference(); err != nil {
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

    // Delete interests
    if err := user.clearInterests(); err != nil {
        return err
    }

    return nil
}

// Update User with specified parameters
func updateUser(user User) (err error) {
    var args []interface{}

    if userID, ok := user["id"]; !ok {
        return ErrMissingKey
    } else {
        if userID, ok := userID.(int); !ok {
            return ErrTypeAssertionFailed
        } else {
            args = append(args, fmt.Sprint("user:", userID))
        }
    }

    user["updatedAt"] = time.Now().Unix()

    // Delete email reference
    if _, ok := user["email"]; ok {
        if err := user.deleteEmailReference(); err != nil {
            return err
        }
    }

    // Update User
    for k, v := range user {
        args = append(args, k, v)
    }
    if _, err := db.Do("HMSET", args...); err != nil {
        return err
    }

    // Delete email reference
    if email, ok := user["email"]; ok {
        if err := user.setEmailReference(email.(string)); err != nil {
            return err
        }
    }

    // Update User interests if exist
    if interests, ok := user["interests"]; ok {
        if interests, ok := interests.([]string); ok {
            if err := user.setInterests(interests); err != nil {
                return err
            }
        }
    }

	return nil
}

// Get Users matching specified parameters
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

// Get User with specified raw Redis command
func _getUser(command string, args ...interface{}) (User, error) {
    user := User{}

    if reply, err := db.Do(command, args...); err != nil {
        return nil, err
    } else if retrievedUser, err := redis.StringMap(reply, err); err != nil {
        return nil, err
    } else {
        for k, v := range retrievedUser {
            switch k {
            case "id":
                if userID, err := strconv.Atoi(v); err != nil {
                    return user, err
                } else {
                    user[k] = userID
                }
            default:
                user[k] = v
            }
        }
    }

    return user, nil
}

// Get Users with specified raw Redis command
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

// Get User IDs with specified raw Redis command
func _getUserIDs(command string, args ...interface{}) ([]int, error) {
    if reply, err := db.Do(command, args...); err != nil {
        return nil, err
    } else if userIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        return userIDs, nil
    }
}

// Add otherUser as current User's connection
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

// Remove otherUser from current User's connection
func (user User) removeUser(otherUser User) error {
    if _, err := db.Do("ZREM", fmt.Sprint("userConnections:", user["id"]), otherUser["id"]); err != nil {
        return err
    }
    if _, err := db.Do("ZREM", fmt.Sprint("userConnections:", otherUser["id"]), user["id"]); err != nil {
        return err
    }
    return nil
}

// Get current User's connected users' IDs
func (user User) otherUserIDs() ([]int, error) {
    if reply, err := db.Do("ZRANGE", fmt.Sprint("userConnections:", user["id"]), 0, -1); err != nil {
        return nil, err
    } else if userIDs, err := redis.Ints(reply, err); err != nil {
        return nil, err
    } else {
        return userIDs, nil
    }
}

// Get Users with similar interests with current User
func (user User) similarUsers() ([]User, error) {
    var allUsers []User

    if interests, err := user.interests(); err != nil {
        return nil, err
    } else {
        for _, interest := range interests {
            if users, err := _getUsers("ZRANGE", fmt.Sprint("interest:", interest), 0, -1); err != nil {
                return nil, err
            } else {
                for k, v := range users {
                    if v["id"] == user["id"] {
                        users = append(users[:k], users[k+1:]...)
                        break
                    }
                }
                allUsers = append(allUsers, users...)
            }
        }

        return allUsers, nil
    }
}

// Get current User's interests
func (user User) interests() ([]string, error) {
    if reply, err := db.Do("ZRANGE", fmt.Sprint("user:", user["id"], ":interests"), 0, -1); err != nil {
        if err == redis.ErrNil {
            return nil, nil
        }
        return nil, err
    } else if interests, err := redis.Strings(reply, err); err != nil {
        return nil, err
    } else {
        return interests, nil
    }
}

// Set current User's interests
func (user User) setInterests(interests []string) error {
    if err := user.clearInterests(); err != nil {
        return err
    }

    for _, interest := range interests {
        if _, err := db.Do("ZADD", fmt.Sprint("interest:", interest), time.Now().Unix(), user["id"]); err != nil {
            return err
        }
        if _, err := db.Do("ZADD", fmt.Sprint("user:", user["id"], ":interests"), time.Now().Unix(), interest); err != nil {
            return err
        }
    }

    return nil
}

// Clear current User's interests
func (user User) clearInterests() error {
    if interests, err := user.interests(); err == nil {
        // Delete interests
        if _, err := db.Do("DEL", fmt.Sprint("user:", user["id"], ":interests")); err != nil {
            return err
        }

        // Remove User from interests
        for _, interest := range interests {
            if _, err := db.Do("ZREM", fmt.Sprint("interest:", interest), user["id"]); err != nil {
                return err
            }
        }
    } else if err != redis.ErrNil {
        return err
    }

    return nil
}

func (user User) email() (string, error) {
    if reply, err := db.Do("HGET", fmt.Sprint("user:", user["id"]), "email"); err != nil {
        return "", err
    } else if email, err := redis.String(reply, err); err != nil {
        return "", err
    } else {
        return email, nil
    }
}

func (user User) setEmailReference(email string) error {
    if email == "" {
        email = user["email"].(string)
    }
    if _, err := db.Do("SET", fmt.Sprint("user:email:", user["email"]), user["id"]); err != nil {
        return err
    }
    return nil
}

func (user User) deleteEmailReference() error {
    if _, err := db.Do("DEL", fmt.Sprint("user:email:", user["email"])); err != nil {
        return err
    }
    return nil
}
