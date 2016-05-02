package main

import (
    "testing"

    "github.com/garyburd/redigo/redis"
)

func TestUser(t *testing.T) {
    var err error

    if db, err = redis.Dial("tcp", ":6379"); err != nil {
        t.Error(err)
    }
    defer db.Close()

    // Insert user
    user := User{
        "firstname": "Jane",
        "lastname": "Doe",
        "email": "jane.doe@example.com",
        "password": "abcd1234",
        "imageURL": "content/jane_doe.jpg",
    }

    var userID int
    if userID, err = user.insert(); err != nil {
        t.Error("user.insert:", err)
    }
    user["id"] = userID

    // Update user
    user["firstname"] = "John"
    user["lastname"] = "Cook"
    user["email"] = "john.cook@example.com"
    user["password"] = "1234abcd"
    user["imageURL"] = "content/john_cook.jpg"
    if err := user.update(); err != nil {
        t.Error("updateUser:", err)
    }

    // Get user
    if _, err = user.fetch(); err != nil {
        t.Error("user.fetch:", err)
    }

    // Has user
    if ok, _ := user._exists(); !ok {
        t.Error("user._exists")
    }

    // Get users
    if users, err := fetchUsers(map[string]interface{}{"count": 5}); err != nil || len(users) < 1 {
        t.Error("fetchUsers:", err)
    }

    // Delete user
    if err = user.delete(); err != nil {
        t.Error("user.delete:", err)
    }
}
