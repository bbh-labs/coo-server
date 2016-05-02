package main

import (
	"log"
	"net/http"
)

func loggedIn(w http.ResponseWriter, r *http.Request, fetchUser bool) (bool, User) {
	session, err := ss.Get(r, "session")
	if err != nil {
		log.Println(err)
		return false, nil
	}

	val := session.Values["userID"]
	if userID, ok := val.(int); !ok {
		return false, nil
	} else {
        user := User{"id": userID}
        if exists, user := user.exists(fetchUser); !exists {
            return false, nil
        } else {
            return true, user
        }
	}
}

func logIn(w http.ResponseWriter, r *http.Request, user User) error {
	session, err := ss.Get(r, "session")
	if err != nil {
		return err
	}

	session.Values["userID"] = user["id"].(int)
	session.Save(r, w)
	return nil
}

func logOut(w http.ResponseWriter, r *http.Request) error {
	session, err := ss.Get(r, "session")
	if err != nil {
		return err
	}

	session.Values["userID"] = 0
	session.Save(r, w)
	return nil
}
