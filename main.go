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
    "time"

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
    ErrEntityNotFound      = errors.New("Entity not found")
    ErrEmptyParameter      = errors.New("Empty parameter")
    ErrMissingKey          = errors.New("Missing key")
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
    apiRouter.HandleFunc("/user/connect", userConnectHandler)
    apiRouter.HandleFunc("/users", usersHandler)
    apiRouter.HandleFunc("/users/simular", usersSimilarHandler)
    apiRouter.HandleFunc("/longtable", longTableHandler)
    apiRouter.HandleFunc("/longtables", longTablesHandler)
    apiRouter.HandleFunc("/longtable/booking", longTableBookingHandler)

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

    // Check if User is logged in
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

    // Check if User already exists
    // If so, log her in
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
    user["imageURL"] = authuser.AvatarURL

    // Insert User
    if user["id"], err = insertUser(user); err != nil {
        log.Println(err)
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    // Log User in
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
        // Check if User is logged in
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

        // Check email length
        if len(email) < 6 {
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte("Email is too short"))
            return
        }

        password := r.FormValue("password")

        // Check password length
        if len(password) < 8 {
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte("Password is too short"))
            return
        }


        user := User{"email": email}

        // Check if User exists
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

        // Check email length
        if len(email) < 6 {
            http.Error(w, ErrEmailTooShort.Error(), http.StatusBadRequest)
            return
        }

        password := r.FormValue("password")

        // Check password length
        if len(password) < 8 {
            http.Error(w, ErrPasswordTooShort.Error(), http.StatusBadRequest)
            return
        }

        firstname := r.FormValue("firstname")
        lastname := r.FormValue("lastname")

        imageURL := ""

        // Copy uploaded image to 'content' folder
        if destination, err := copyFile(r, "image", "content", randomFilename()); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        } else {
            imageURL = destination
        }

        // Generate hashed password
        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        // Initialize User data
        user := User{
            "firstname": firstname,
            "lastname":  lastname,
            "email":     email,
            "password":  string(hashedPassword),
            "imageURL":  imageURL,
        }

        // Set User interests if exist
        if interests, ok := r.Form["interests"]; ok {
            user["interests"] = interests
        }

        // Insert User
        if user["id"], err = insertUser(user); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        // Log User in
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

func logoutHandler(w http.ResponseWriter, r *http.Request) {
    // Log User out
    if err := logOut(w, r); err != nil {
        log.Println(err)
        w.WriteHeader(http.StatusInternalServerError)
    }
    w.WriteHeader(http.StatusOK)
}

func userHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "PATCH":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Set User firstname and lastname
        user["firstname"] = r.FormValue("firstname")
        user["lastname"] = r.FormValue("lastname")
        user["email"] = r.FormValue("email")

        // Check if User is updating password
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

        // Try to copy uploaded image to 'content' folder
        if destination, err := copyFile(r, "image", "content", randomFilename()); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        } else {
            // Check if User previously has an image, if so remove it
            if imageURL, ok := user["imageURL"].(string); ok && imageURL != "" {
                if err := os.Remove(imageURL); err != nil {
                    log.Println(err)
                }
            } else {
                w.WriteHeader(http.StatusInternalServerError)
                return
            }

            // Successfully copied so set the destination path as the image URL
            user["imageURL"] = destination
        }

        // Set User interests if exist
        if interests, ok := r.Form["interests"]; ok {
            user["interests"] = interests
        }

        // Update User
        if err := updateUser(user); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
    case "DELETE":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Delete User
        if err := deleteUser(user); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func userConnectHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "POST":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        var otherUserID int
        var err error

        // Check if otherUserID query parameter is valid
        if otherUserID, err = strconv.Atoi(r.FormValue("otherUserID")); err != nil {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        } else if ok, err := hasUser(User{"id": otherUserID}); !ok || err != nil {
            http.Error(w, ErrEntityNotFound.Error(), http.StatusBadRequest)
            return
        }

        // Add the other User as new connection of current User
        if err = user.addUser(User{"id": otherUserID}); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)

    case "DELETE":
        var otherUserID int
        var err error

        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Check if otherUserID query parameter is valid
        if otherUserID, err = strconv.Atoi(r.FormValue("otherUserID")); err != nil {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        } else if ok, err := hasUser(User{"id": otherUserID}); !ok || err != nil {
            http.Error(w, ErrEntityNotFound.Error(), http.StatusBadRequest)
            return
        }

        // Remove other User from current User's connection
        if err = user.removeUser(User{"id": otherUserID}); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
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
        var count int
        var err error

        // Set default 'count' if not set by the query
        if count, err = strconv.Atoi(r.FormValue("count")); err != nil {
            count = 100
        }

        // Prepare parameters
        params := map[string]interface{}{"count": count}

        // Set 'interests' parameter if exists
        if interests, ok := r.Form["interests"]; ok {
            params["interests"] = interests
        }

        // Get Users that match the parameters
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

func usersSimilarHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Get similar Users
        if users, err := user.similarUsers(); err != nil {
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

func longTableHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        longTable := LongTable{}

        // Check if 'id' query parameter is valid
        if id, err := strconv.Atoi(r.FormValue("id")); err != nil {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["id"] = id
        }

        // Get LongTable with set 'id'
        if _, err := getLongTable(longTable); err != nil {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            data, err := json.Marshal(longTable)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            w.Write(data)
        }

    case "POST":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        name := r.FormValue("name")

        // Check if 'name' query parameter is valid
        if name == "" {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        }

        // Initialize LongTable
        longTable := LongTable{"userID": user["id"], "name": name}

        // Check if 'numSeats' query parameter is valid
        if numSeats, err := strconv.Atoi(r.FormValue("numSeats")); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["numSeats"] = numSeats
        }

        // Check if 'openingTime' query parameter is valid
        openingTime := r.FormValue("openingTime")
        if _, err := time.Parse("15:04", openingTime); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["openingTime"] = openingTime
        }

        // Check if 'closingTime' query parameter is valid
        closingTime := r.FormValue("closingTime")
        if _, err := time.Parse("15:04", closingTime); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["closingTime"] = closingTime
        }

        // Insert LongTable
        if longTableID, err := insertLongTable(longTable); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            w.Write([]byte(strconv.Itoa(longTableID)))
        }

    case "PATCH":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Initialize LongTable
        longTable := LongTable{"userID": user["id"]}
        if id, err := strconv.Atoi(r.FormValue("id")); err != nil {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["id"] = id
        }

        name := r.FormValue("name")

        // Check if 'name' query parameter is valid
        if name == "" {
            http.Error(w, ErrEmptyParameter.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["name"] = name
        }

        // Check if 'numSeats' query parameter is valid
        if numSeats, err := strconv.Atoi(r.FormValue("numSeats")); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["numSeats"] = numSeats
        }

        // Check if 'openingTime' query parameter is valid
        openingTime := r.FormValue("openingTime")
        if _, err := time.Parse("15:04", openingTime); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["openingTime"] = openingTime
        }

        // Check if 'closingTime' query parameter is valid
        closingTime := r.FormValue("closingTime")
        if _, err := time.Parse("15:04", closingTime); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTable["closingTime"] = closingTime
        }

        // Update LongTable
        if err := updateLongTable(longTable); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            w.Write([]byte(strconv.Itoa(longTable["id"].(int))))
        }

    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func longTablesHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        var count int
        var err error

        // Set default 'count' if not set by the query
        if count, err = strconv.Atoi(r.FormValue("count")); err != nil {
            count = 100
        }

        // Prepare parameters
        params := map[string]interface{}{"count": count}

        // Get longtables that match the parameters
        if longTables, err := getLongTables(params); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            data, err := json.Marshal(longTables)
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

func longTableBookingHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "POST":
        var longTableID, seatPosition int
        var err error

        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Initialize LongTableBooking
        longTableBooking := LongTableBooking{"userID": user["id"]}

        // Check if 'seatPosition' query parameter is valid
        if seatPosition, err = strconv.Atoi(r.FormValue("seatPosition")); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTableBooking["seatPosition"] = seatPosition
        }

        // Check if 'longTableID' query parameter is valid
        if longTableID, err = strconv.Atoi(r.FormValue("longTableID")); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            // Get LongTable with set 'longTableID'
            if longTable, err := getLongTable(LongTable{"id": longTableID}); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            } else {
                // Check if seatPosition is equal to or higher than numSeats
                if numSeats, ok := longTable["numSeats"].(int); !ok {
                    http.Error(w, ErrTypeAssertionFailed.Error(), http.StatusInternalServerError)
                    return
                } else if seatPosition >= numSeats {
                    w.WriteHeader(http.StatusBadRequest)
                    return
                }
            }
            longTableBooking["longTableID"] = longTableID
        }

        // Insert LongTableBooking
        if longTableBookingID, err := insertLongTableBooking(longTableBooking); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            w.Write([]byte(strconv.Itoa(longTableBookingID)))
        }

    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

// Copy file from request to local destination
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

// Generates randomized filename
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

// Check if map has specified keys
func checkKeys(m map[string]interface{}, args ...string) bool {
    for _, key := range args {
        if _, ok := m[key]; !ok {
            return false
        }
    }
    return true
}
