// +build !headless

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/markbates/pkger"
	"github.com/tyler-smith/go-bip39"

	"redwood.dev"
	"redwood.dev/crypto"
)

var chLoggedOut = make(chan struct{}, 1)

func startAPI(port uint) {
	server := http.NewServeMux()
	handler := redwood.UnrestrictedCors(server)

	server.HandleFunc("/api/login", loginUser)
	server.HandleFunc("/api/logout", logoutUser)
	server.HandleFunc("/api/profile-names", getProfileNames)
	server.HandleFunc("/api/confirm-profile", confirmProfile)
	server.HandleFunc("/api/check-login", checkLogin)
	server.HandleFunc("/", serveHome)

	err := http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func loginUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var loginRequest struct {
		Password    string `json:"password"`
		Mnemonic    string `json:"mnemonic"`
		ProfileName string `json:"profileName"`
	}
	err := json.NewDecoder(r.Body).Decode(&loginRequest)
	if err != nil {
		panic(err)
	}

	// Check if profileName exists
	if len(loginRequest.ProfileName) == 0 {
		http.Error(w, "Profile name is required.", http.StatusInternalServerError)
		return
	}

	// Check if mnemonic exists
	if len(loginRequest.Mnemonic) > 0 {
		if !bip39.IsMnemonicValid(loginRequest.Mnemonic) {
			http.Error(w, "Mnemonic isn't valid.", http.StatusInternalServerError)
			return
		}

		files, err := ioutil.ReadDir(app.profileRoot)
		if err == nil {
			// Check if profilName is unique
			for _, profileNames := range files {
				if profileNames.IsDir() {
					if strings.ToLower(profileNames.Name()) == strings.ToLower(loginRequest.ProfileName) {
						http.Error(w, "Profile name already exists.", http.StatusInternalServerError)
						return
					}
				}
			}
		}
	}

	// Else just unlock profile
	app.password = loginRequest.Password
	app.mnemonic = loginRequest.Mnemonic
	app.profileName = loginRequest.ProfileName
	app.devMode = true

	err = app.Start()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "WORKED!")
}

func getProfileNames(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var reqResponse struct {
		ProfileNames []string `json:"profileNames"`
	}

	files, err := ioutil.ReadDir(app.profileRoot)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	for _, profileNames := range files {
		if profileNames.IsDir() {
			profileName := profileNames.Name()
			reqResponse.ProfileNames = append(reqResponse.ProfileNames, profileName)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(reqResponse)
}

func confirmProfile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var reqResponse struct {
		ProfileName string `json:"profileName"`
	}

	err := json.NewDecoder(r.Body).Decode(&reqResponse)
	if err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(app.profileRoot)
	if err == nil {
		// Check if profilName is unique
		for _, profileName := range files {
			if profileName.IsDir() {
				if strings.ToLower(profileName.Name()) == strings.ToLower(reqResponse.ProfileName) {
					http.Error(w, "Profile name already exists.", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	mnemonic, err := crypto.GenerateMnemonic()
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(w, mnemonic)
}

func checkLogin(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	fmt.Fprintf(w, strconv.FormatBool(app.started))
}

func logoutUser(w http.ResponseWriter, r *http.Request) {
	select {
	case chLoggedOut <- struct{}{}:
	default:
	}
	app.Close()
	fmt.Fprintf(w, "WORKED!")
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join("/frontend/build", r.URL.Path)
	fmt.Println("PATH: ", path)
	switch filepath.Ext(path) {
	case ".html":
		w.Header().Add("Content-Type", "text/html")
	case ".js":
		w.Header().Add("Content-Type", "application/javascript")
	case ".css":
		w.Header().Add("Content-Type", "text/css")
	case ".svg":
		w.Header().Add("Content-Type", "image/svg+xml")
	case ".map":
		w.Header().Add("Content-Type", "text/plain")
	}

	indexHTML, err := pkger.Open(path)
	if err != nil {
		panic(err) // @@TODO
	}

	_, err = io.Copy(w, indexHTML)
	if err != nil {
		panic(err) // @@TODO
	}
}
