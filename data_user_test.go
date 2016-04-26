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
        "image_url": "content/jane_doe.jpg",
    }

    var userID uint64
    if userID, err = insertUser(user); err != nil {
        t.Error("insertUser:", err)
    }
    user["id"] = userID

    // Update user
    user["firstname"] = "John"
    user["lastname"] = "Cook"
    user["email"] = "john.cook@example.com"
    user["password"] = "1234abcd"
    user["image_url"] = "content/john_cook.jpg"
    if err := updateUser(user); err != nil {
        t.Error("updateUser:", err)
    }

    // Get user
    if _, err := getUser(user); err != nil {
        t.Error("getUser:", err)
    }

    // Delete user
    if err = deleteUser(user); err != nil {
        t.Error("deleteUser:", err)
    }
}
