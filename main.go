package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
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
var ss = sessions.NewCookieStore([]byte("HbeA9vqJ7Wk+rLGmYzyp9SAdHxmK4EIVtylo/aXZ/ZA="))
var templates *template.Template

// Command-line flags
var address = flag.String("address", "http://localhost:8080", "server address")
var port = flag.String("port", "8080", "server port")
var test = flag.Bool("test", false, "serve front-end test sample")

// Errors
var (
    ErrEmailTooShort       = errors.New("Email too short")
    ErrPasswordTooShort    = errors.New("Password too short")
    ErrFirstnameTooShort   = errors.New("Firstname too short")
    ErrLastnameTooShort    = errors.New("Lastname too short")

    ErrNotLoggedIn         = errors.New("User is not logged in")
    ErrPasswordMismatch    = errors.New("Password mismatch")
    ErrWrongDateFormat     = errors.New("Wrong date format")
    ErrTypeAssertionFailed = errors.New("Type assertion failed")
    ErrEntityNotFound      = errors.New("Entity not found")
    ErrEmptyParameter      = errors.New("Empty parameter")
    ErrMissingKey          = errors.New("Missing key")
    ErrIDMismach           = errors.New("ID mismatch")
    ErrPermissionDenied    = errors.New("Permission denied")
)

// Constants
const (
    DateFormat = "2-1-2006"
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
    apiRouter.HandleFunc("/users/similar", usersSimilarHandler)
    apiRouter.HandleFunc("/longtable", longTableHandler)
    apiRouter.HandleFunc("/longtables", longTablesHandler)
    apiRouter.HandleFunc("/longtable/booking", longTableBookingHandler)

    // Extra
    apiRouter.HandleFunc("/longtable/booking/delete", longTableBookingDeleteHandlerFunc)
    apiRouter.HandleFunc("/user/connect/delete", userConnectionDeleteHandlerFunc)

    // Prepare social login authenticators
    patHandler := pat.New()
    patHandler.Get("/auth/{provider}/callback", authHandler)
    patHandler.Get("/auth/{provider}", gothic.BeginAuthHandler)
    router.PathPrefix("/auth").Handler(patHandler)

    // If running test server, set up template handlers
    if *test {
        funcMap := template.FuncMap{
            "longtables": func(count int) []LongTable {
                if longTables, err := getLongTables(map[string]interface{}{"count": count}); err != nil {
                    return nil
                } else {
                    return longTables
                }
            },
            "minus": func(a, b int) int {
                return a - b
            },
        }
        templates = template.Must(template.New("main").Funcs(funcMap).ParseGlob("test/*.html"))
        setupTemplateHandlers(router)
    }

    // Run web server
    var n *negroni.Negroni
    if *test {
        n = negroni.New(negroni.NewRecovery(), negroni.NewLogger(), negroni.NewStatic(http.Dir("test/public")))
    } else {
        n = negroni.Classic()
    }
    n.UseHandler(router)
    n.Run(":" + *port)
}

func setupTemplateHandlers(router *mux.Router) {
    // Index
    router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if loggedIn, _ := loggedIn(w, r, false); loggedIn {
            http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
        } else {
            templates.ExecuteTemplate(w, "index", nil)
        }
    })

    // Dashboard
    router.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
        if loggedIn, user := loggedIn(w, r, true); loggedIn {
            templates.ExecuteTemplate(w, "dashboard", user)
        } else {
            http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        }
    })

    // Profile
    router.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
        if loggedIn, user := loggedIn(w, r, true); loggedIn {
            templates.ExecuteTemplate(w, "profile", user)
        } else {
            http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        }
    })

    // Profile (with ID)
    router.HandleFunc("/profile/{id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
        if loggedIn, user := loggedIn(w, r, true); loggedIn {
            vars := mux.Vars(r)
            if otherUserID, err := strconv.Atoi(vars["id"]); err != nil {
                templates.ExecuteTemplate(w, "profile", user)
            } else if user["id"].(int) == otherUserID {
                templates.ExecuteTemplate(w, "profile", user)
            } else {
                user = User{"id": otherUserID}
                if otherUser, err := user.fetch(); err != nil {
                    http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
                } else {
                    templates.ExecuteTemplate(w, "profile", map[string]interface{}{"user": user, "otherUser": otherUser})
                }
            }
        } else {
            http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        }
    })

    // LongTables
    router.HandleFunc("/longtables", func(w http.ResponseWriter, r *http.Request) {
        if loggedIn, user := loggedIn(w, r, true); loggedIn {
            templates.ExecuteTemplate(w, "longtables", user)
        } else {
            http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        }
    })

    // LongTable
    router.HandleFunc("/longtable/{id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
        if loggedIn, user := loggedIn(w, r, true); loggedIn {
            vars := mux.Vars(r)
            id, _ := strconv.Atoi(vars["id"])
            longTable := LongTable{"id": id}
            if longTable, err := longTable.fetch(); err != nil {
                w.WriteHeader(http.StatusNotFound)
            } else {
                templates.ExecuteTemplate(w, "longtable", map[string]interface{}{"user":user,"longtable":longTable})
            }
        } else {
            http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
        }
    })
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
    if exists, user := user.exists(true); exists {
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
    if user["id"], err = user.insert(); err != nil {
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
            if *test {
                http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
            } else {
                if data, err := json.Marshal(user); err != nil {
                    log.Println(err)
                    w.WriteHeader(http.StatusInternalServerError)
                } else {
                    w.Write(data)
                }
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
        if exists, user := user.exists(true); exists {
            if err := bcrypt.CompareHashAndPassword([]byte(user["password"].(string)), []byte(password)); err != nil {
                w.WriteHeader(http.StatusForbidden)
            } else {
                if err := logIn(w, r, user); err != nil {
                    log.Println(err)
                    w.WriteHeader(http.StatusInternalServerError)
                } else {
                    if *test {
                        http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
                    } else {
                        w.WriteHeader(http.StatusOK)
                    }
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
        if len(firstname) < 2 {
            http.Error(w, ErrFirstnameTooShort.Error(), http.StatusBadRequest)
            return
        }

        lastname := r.FormValue("lastname")
        if len(lastname) < 2 {
            http.Error(w, ErrLastnameTooShort.Error(), http.StatusBadRequest)
            return
        }

        birthdate := r.FormValue("birthdate")
        if _, err := parseDate(birthdate); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        imageURL := ""

        // Copy uploaded image to 'content' folder
        if strings.HasPrefix(r.Header["Content-Type"][0], "multipart/form-data") {
            if destination, err := copyFile(r, "image", "content", randomFilename()); err != nil {
                log.Println(err)
                w.WriteHeader(http.StatusInternalServerError)
                return
            } else if destination != "" {
                imageURL = destination
            }
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
            "birthdate": birthdate,
        }

        // Set User interests if exist
        if interests, ok := r.Form["interests"]; ok {
            user["interests"] = interests
        }

        // Insert User
        if user["id"], err = user.insert(); err != nil {
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

        if *test {
            http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
        } else {
            w.WriteHeader(http.StatusOK)
        }
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

    if *test {
        http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
    } else {
        w.WriteHeader(http.StatusOK)
    }
}

func userHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "POST": fallthrough
    case "PATCH":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        birthdate := r.FormValue("birthdate")

        // Set User info
        s := Setter{}
        if birthdate != "" {
            s.setDate(user, "birthdate", birthdate)
        }
        s.set(user, "firstname", r.FormValue("firstname"))
        s.set(user, "lastname", r.FormValue("lastname"))
        s.set(user, "email", r.FormValue("email"))
        s.set(user, "travellingAs", r.FormValue("travellingAs"))
        s.set(user, "wechatNumber", r.FormValue("wechatNumber"))
        s.set(user, "lineNumber", r.FormValue("lineNumber"))
        s.set(user, "facebookNumber", r.FormValue("facebookNumber"))
        s.set(user, "skypeNumber", r.FormValue("skypeNumber"))
        s.set(user, "whatsappNumber", r.FormValue("whatsappNumber"))
        if s.err != nil {
            http.Error(w, s.err.Error(), http.StatusBadRequest)
            return
        }

        // Check if User is updating password
        oldPassword := r.FormValue("old-password")
        newPassword := r.FormValue("new-password")
        // Process valid input (both passwords are at least the minimum length)
        if len(oldPassword) >= 8 && len(newPassword) >= 8 {
            // Check if old password matches
            if err := bcrypt.CompareHashAndPassword([]byte(user["password"].(string)), []byte(oldPassword)); err != nil {
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
        }

        if strings.HasPrefix(r.Header["Content-Type"][0], "multipart/form-data") {
            // Try to copy uploaded image to 'content' folder
            if destination, err := copyFile(r, "image", "content", randomFilename()); err != nil {
                log.Println(err)
                w.WriteHeader(http.StatusInternalServerError)
                return
            } else if destination != "" {
                // Check if User previously has an image, if so remove it
                if imageURL, ok := user["imageURL"]; ok {
                    if imageURL, ok := imageURL.(string); ok && imageURL != "" {
                        if err := os.Remove(imageURL); err != nil {
                            log.Println(err)
                        }
                    }

                    // Successfully copied so set the destination path as the image URL
                    user["imageURL"] = destination
                }
            }
        }

        // Set User interests if exist
        if interests, ok := r.Form["interests"]; ok {
            user["interests"] = interests
        }

        // Update User
        if err := user.update(); err != nil {
            log.Println(err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        if *test {
            http.Redirect(w, r, "/profile", http.StatusTemporaryRedirect)
        } else {
            w.WriteHeader(http.StatusOK)
        }
    case "DELETE":
        // Check if User is logged in
        loggedIn, user := loggedIn(w, r, true)
        if !loggedIn {
            http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
            return
        }

        // Delete User
        if err := user.delete(); err != nil {
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
        } else {
            user := User{"id": otherUserID}
            if _, err := user.exists(false); err != nil {
                http.Error(w, ErrEntityNotFound.Error(), http.StatusBadRequest)
                return
            }
        }

        // Add the other User as new connection of current User
        if err = user.addUser(User{"id": otherUserID}); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        if *test {
            http.Redirect(w, r, fmt.Sprint("/profile/", otherUserID), http.StatusTemporaryRedirect)
        } else {
            w.WriteHeader(http.StatusOK)
        }

    case "DELETE":
        userConnectionDeleteHandlerFunc(w, r)

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
        if users, err := fetchUsers(params); err != nil {
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
        if users, err := user.SimilarUsers(); err != nil {
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
        if _, err := longTable.fetch(); err != nil {
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
        if longTableID, err := longTable.insert(); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            if *test {
                http.Redirect(w, r, "/longtable", http.StatusTemporaryRedirect)
            } else {
                w.Write([]byte(strconv.Itoa(longTableID)))
            }
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
        if err := longTable.update(); err != nil {
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
        var date string
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

        // Check if 'date' query parameter is valid
        date = r.FormValue("date")
        if _, err = time.Parse(DateFormat, date); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            longTableBooking["date"] = date
        }

        // Check if 'longTableID' query parameter is valid
        if longTableID, err = strconv.Atoi(r.FormValue("longTableID")); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        } else {
            // Get LongTable with set 'longTableID'
            longTable := LongTable{"id": longTableID}
            if longTable, err := longTable.fetch(); err != nil {
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
        if longTableBookingID, err := longTableBooking.insert(); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        } else {
            if *test {
                http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
            } else {
                w.Write([]byte(strconv.Itoa(longTableBookingID)))
            }
        }

    case "DELETE":
        longTableBookingDeleteHandlerFunc(w, r)

    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}


func longTableBookingDeleteHandlerFunc(w http.ResponseWriter, r *http.Request) {
    // Check if User is logged in
    loggedIn, user := loggedIn(w, r, true)
    if !loggedIn {
        http.Error(w, ErrNotLoggedIn.Error(), http.StatusForbidden)
        return
    }

    var longTableBookingID int
    var err error

    if longTableBookingID, err = strconv.Atoi(r.FormValue("longTableBookingID")); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    longTableBooking := LongTableBooking{"id":longTableBookingID, "userID": user["id"]}
    if err := longTableBooking.delete(); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if *test {
        http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
    } else {
        w.WriteHeader(http.StatusOK)
    }
}

func userConnectionDeleteHandlerFunc(w http.ResponseWriter, r *http.Request) {
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
    } else {
        user := User{"id": otherUserID}
        if ok, err := user.exists(false); !ok || err != nil {
            http.Error(w, ErrEntityNotFound.Error(), http.StatusBadRequest)
            return
        }
    }

    // Remove other User from current User's connection
    if err = user.removeUser(User{"id": otherUserID}); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if *test {
        http.Redirect(w, r, fmt.Sprint("/profile/", otherUserID), http.StatusTemporaryRedirect)
    } else {
        w.WriteHeader(http.StatusOK)
    }
}
