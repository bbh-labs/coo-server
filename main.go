package main

import (
    "encoding/json"
    "errors"
    "flag"
    "io"
    "log"
    "mime/multipart"
    "net/http"
    "os"
    "os/exec"
    "os/signal"
    "strconv"
    "strings"
    "syscall"

    "github.com/codegangsta/negroni"
    "github.com/garyburd/redigo/redis"
    "github.com/gorilla/mux"
    "github.com/gorilla/pat"
    "github.com/gorilla/sessions"
    "github.com/markbates/goth"
    "github.com/markbates/goth/gothic"
    "github.com/markbates/goth/providers/facebook"
    "github.com/markbates/goth/providers/instagram"
    "github.com/markbates/goth/providers/twitter"
    "golang.org/x/crypto/bcrypt"
)

var db redis.Conn
var ss = sessions.NewCookieStore([]byte("SHuADRV4npfjU4stuN5dvcYaMmblSZlUyZbEl/mKyyw="))

// Command-line flags
var address = flag.String("address", "http://localhost:8080", "server address")
var port = flag.String("port", "8080", "server port")

// Errors
var (
    ErrEmailTooShort       = errors.New("Email too short")
    ErrPasswordTooShort    = errors.New("Password too short")
    ErrNotLoggedIn         = errors.New("User is not logged in")
    ErrPasswordMismatch    = errors.New("Password mismatch")
    ErrTypeAssertionFailed = errors.New("Type assertion failed")
    ErrKeyNotFound = errors.New("Key not found")
    ErrEmptyParameter = errors.New("Empty parameter")
)

func main() {
    var err error

    // Handle OS signals
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM, os.Kill)
    go func() {
        sig := <-c
        if db != nil {
            db.Close()
        }
        log.Println("Received signal:", sig)
        os.Exit(0)
    }()

    // Parse command-line flags
    flag.Parse()

    // Connect to database
    if db, err = redis.Dial("tcp", ":6379"); err != nil {
        log.Fatal(err)
    }

    // Setup social logins
    gothic.Store = sessions.NewFilesystemStore(os.TempDir(), []byte("coo"))
    goth.UseProviders(
        facebook.New(os.Getenv("FACEBOOK_KEY"), os.Getenv("FACEBOOK_SECRET"), *address+"/auth/facebook/callback"),
        instagram.New(os.Getenv("INSTAGRAM_KEY"), os.Getenv("INSTAGRAM_SECRET"), *address+"/auth/instagram/callback"),
        twitter.New(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), *address+"/auth/twitter/callback"),
    )

    // Prepare web server
    router := mux.NewRouter()
    apiRouter := router.PathPrefix("/api").Subrouter()
    apiRouter.HandleFunc("/login", loginHandler)
    apiRouter.HandleFunc("/signup", signupHandler)
    apiRouter.HandleFunc("/logout", logoutHandler)
    apiRouter.HandleFunc("/user", userHandler)
    apiRouter.HandleFunc("/users", usersHandler)
    apiRouter.HandleFunc("/longtable", longtableHandler)
    apiRouter.HandleFunc("/longtables", longtablesHandler)
    apiRouter.HandleFunc("/longtable/booking", longtableBookingHandler)

    // Prepare social login authenticators
    patHandler := pat.New()
    patHandler.Get("/auth/{provider}/callback", authHandler)
    patHandler.Get("/auth/{provider}", gothic.BeginAuthHandler)
    router.PathPrefix("/auth").Handler(patHandler)

    // Run web server
    n := negroni.Classic()
    n.UseHandler(router)
    n.Run(":" + *port)
}

func authHandler(w http.ResponseWriter, r *http.Request) {
    authuser, err := gothic.CompleteUserAuth(w, r)
    if err != nil {
        log.Println(err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if loggedIn, _ := loggedIn(w, r, true); loggedIn {
        switch authuser.Provider {
        case "facebook":
            //
        case "instagram":
            //
        case "twitter":
            //
        default:
            w.WriteHeader(http.StatusBadRequest)
            return
        }
        http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        return
    }

    user := User{}
    switch authuser.Provider {
    case "facebook":
        //
    case "instagram":
        //
    case "twitter":
        //
    default:
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    if exists, user := userExists(user, true); exists {
        if err := logIn(w, r, user); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        return
    }

    name := strings.Split(authuser.Name, " ")

    if len(name) > 1 {
        user["firstname"] = strings.Join(name[:len(name)-1], " ")
        user["lastname"] = name[len(name)-1]
    } else {
        user["firstname"] = name[0]
    }
    user["description"] = authuser.Description
    user["email"] = authuser.Email
    user["image_url"] = authuser.AvatarURL

    if user["id"], err = insertUser(user); err != nil {
        log.Println(err)
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    if err := logIn(w, r, user); err != nil {
        log.Println(err)
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        if ok, user := loggedIn(w, r, true); !ok {
            w.WriteHeader(http.StatusForbidden)
        } else {
            if data, err := json.Marshal(user); err != nil {
                log.Println(err)
                w.WriteHeader(http.StatusInternalServerError)
            } else {
                w.Write(data)
            }
        }
    case "POST":
        email := r.FormValue("email")
        if len(email) < 6 {
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte("Email is too short"))
            return
        }

        password := r.FormValue("password")
        if len(password) < 8 {
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte("Password is too short"))
            return
        }


        user := User{"email": email}
        if exists, user := userExists(user, true); exists {
            if err := bcrypt.CompareHashAndPassword(user["password"].([]byte), []byte(password)); err != nil {
                w.WriteHeader(http.StatusForbidden)
            } else {
                if err := logIn(w, r, user); err != nil {
                    log.Println(err)
                    w.WriteHeader(http.StatusInternalServerError)
                } else {
                    w.WriteHeader(http.StatusOK)
                }
            }
        } else {
            w.WriteHeader(http.StatusForbidden)
        }
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "POST":
        email := r.FormValue("email")
        if len(email) < 6 {
            http.Error(w, ErrEmailTooShort.Error(), http.StatusBadRequest)
            return
        }

        password := r.FormValue("password")
        if len(password) < 8 {
            http.Error(w, ErrPasswordTooShort.Error(), http.StatusBadRequest)
            return
        }

        firstname := r.FormValue("firstname")
        lastname := r.FormValue("lastname")

        imageURL := ""
        if destination, err := copyFile(r, "image", "content", randomFilename()); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        } else {
            imageURL = destination
        }

        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        user := User{
            "firstname": firstname,
            "lastname":  lastname,
            "email":     email,
            "password":  string(hashedPassword),
            "image_url":  imageURL,
        }

        if user["id"], err = insertUser(user); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        if err = logIn(w, r, user); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func userHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "PATCH":
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Set user firstname and lastname
        user["firstname"] = r.FormValue("firstname")
        user["lastname"] = r.FormValue("lastname")
        user["email"] = r.FormValue("email")

        // Check if user is updating password
        oldPassword := r.FormValue("old-password")
        newPassword := r.FormValue("new-password")
        if user["password"] != "" {
            // Process valid input (both passwords are at least the minimum length)
            if len(oldPassword) >= 8 && len(newPassword) >= 8 {
                // Check if old password matches
                if err := bcrypt.CompareHashAndPassword(user["password"].([]byte), []byte(oldPassword)); err != nil {
                    w.WriteHeader(http.StatusBadRequest)
                    return
                }

                // Create hashed password from new password
                hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
                if err != nil {
                    log.Println(err)
                    w.WriteHeader(http.StatusInternalServerError)
                    return
                }
                user["password"] = string(hashedPassword)

            // Invalid input (at least one of the password is less than minimum length
            } else if len(oldPassword) > 0 && len(newPassword) > 0 {
                w.WriteHeader(http.StatusBadRequest)
                return

            // Ignore input (at least one of the two passwords is empty)
            } else {
                user["password"] = ""
            }
        }

        // Update user avatar if necessary
        if destination, err := copyFile(r, "image", "content", randomFilename()); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        } else {
            if imageURL, ok := user["image_url"].(string); ok && imageURL != "" {
                if err := os.Remove(imageURL); err != nil {
                    log.Println(err)
                }
            } else {
                w.WriteHeader(http.StatusInternalServerError)
                return
            }
            user["image_url"] = destination
        }

        // Update user interests if exist
        if interests, ok := r.Form["interests"]; ok {
            user["interests"] = interests
        }

        if err := updateUser(user); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        var count uint64 = 100
        var err error

        if count, err = strconv.ParseUint(r.FormValue("count"), 10, 64); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        params := map[string]interface{}{"count": count}
        if interests, ok := r.Form["interests"]; ok {
            params["interests"] = interests
        }

        if users, err := getUsers(params); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            data, err := json.Marshal(users)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            w.Write(data)
        }
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
    if err := logOut(w, r); err != nil {
        log.Println(err)
        w.WriteHeader(http.StatusInternalServerError)
    }
    w.WriteHeader(http.StatusOK)
}

func longtableHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        longtable := LongTable{}
        if id, err := strconv.ParseUint(r.FormValue("id"), 10, 64); err != nil {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            longtable["id"] = id
        }

        if _, err := getLongTable(longtable); err != nil {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        }

        data, err := json.Marshal(longtable)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Write(data)

    case "POST":
        name := r.FormValue("name")
        if len(name) == 0 {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        }

        longtable := LongTable{"name": name}
        if numSeats, err := strconv.ParseUint(r.FormValue("num_seats"), 10, 64); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longtable["num_seats"] = numSeats
        }

        if longtableID, err := insertLongTable(longtable); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            w.Write([]byte(strconv.FormatUint(longtableID, 64)))
        }

    case "PATCH":
        longtable := LongTable{}
        if id, err := strconv.ParseUint(r.FormValue("id"), 10, 64); err != nil {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            longtable["id"] = id
        }

        name := r.FormValue("name")
        if len(name) == 0 {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            longtable["name"] = name
        }

        if numSeats, err := strconv.ParseUint(r.FormValue("num_seats"), 10, 64); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longtable["num_seats"] = numSeats
        }

        if err := updateLongTable(longtable); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            w.Write([]byte(strconv.FormatUint(longtable["id"].(uint64), 64)))
        }

    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func longtablesHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        var count uint64 = 100
        var err error

        if count, err = strconv.ParseUint(r.FormValue("count"), 10, 64); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        params := map[string]interface{}{"count": count}

        if longtables, err := getLongTables(params); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            data, err := json.Marshal(longtables)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            w.Write(data)
        }

    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func copyFile(r *http.Request, name string, folder, filename string) (destination string, err error) {
    var fileheader *multipart.FileHeader

    if _, fileheader, err = r.FormFile("image"); err != nil {
        if err == http.ErrMissingFile {
            err = nil
        }
        return
    } else {
        var infile multipart.File
        var outfile *os.File

        if err = os.MkdirAll(folder, os.ModeDir|0775); err != nil {
            return
        }

        destination = folder + "/" + filename

        // Open received file
        if infile, err = fileheader.Open(); err != nil {
            return
        }
        defer infile.Close()

        // Create destination file
        if outfile, err = os.OpenFile(destination, os.O_CREATE|os.O_WRONLY, 0664); err != nil {
            return
        }
        defer outfile.Close()

        // Copy file to destination
        if _, err = io.Copy(outfile, infile); err != nil {
            return
        }
    }

    return
}

func randomFilename() string {
    cmd := exec.Command("openssl", "rand", "-base64", "64")

    output, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }

    for i := range output {
        if output[i] == '/' || output[i] == '\n'{
            output[i] = '-'
        }
    }

    return string(output)
}
