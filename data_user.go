package main

import (
    //"github.com/garyburd/redigo/redis"
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

func getUserByID(userID int64) (User, error) {
    var user User

    return user, nil
}

func insertUser(user User) (int64, error) {
    var userID int64

    //db.Send("HSET", fmt.Sprintf("user:%d", use

	return userID, nil
}

func updateUser(user User) (err error) {

	return
}
