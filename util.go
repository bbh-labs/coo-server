package main

import (
    "io"
    "log"
    "mime/multipart"
    "net/http"
    "os"
    "os/exec"
    "time"
)

type Setter struct {
    err error
}

func (setter *Setter) set(m map[string]interface{}, key, value string) {
    if setter.err == nil {
        m[key] = value
    }
}

func (setter *Setter) setDate(m map[string]interface{}, key, value string) {
    if setter.err == nil {
        if _, err := parseDate(value); err != nil {
            setter.err = err
        } else {
            m[key] = value
        }
    }
}

// Parse date with the format "02-01-2006"
func parseDate(date string) (time.Time, error) {
    if t, err := time.Parse(DateFormat, date); err != nil {
        return time.Time{}, err
    } else {
        return t, nil
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

// Check if map has all the specified keys
func hasKeys(m map[string]interface{}, args ...string) bool {
    for _, key := range args {
        if _, ok := m[key]; !ok {
            return false
        }
    }
    return true
}

// Check if map has one of the specified keys
func hasKey(m map[string]interface{}, args ...string) (bool, string) {
    for _, key := range args {
        if _, ok := m[key]; ok {
            return true, key
        }
    }
    return false, ""
}
