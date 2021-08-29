// +build !headless

package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tyler-smith/go-bip39"

	"redwood.dev/crypto"
	"redwood.dev/process"
	"redwood.dev/utils"
)

type API struct {
	process.Process
	closeOnce sync.Once

	app   *App
	appMu sync.Mutex

	port        uint
	configPath  string
	profileRoot string

	server        *http.Server
	masterProcess process.ProcessTreer
}

//go:embed frontend/build
var staticAssets embed.FS

func newAPI(port uint, configPath, profileRoot string, masterProcess process.ProcessTreer) *API {
	api := &API{
		Process:       *process.New("api"),
		port:          port,
		configPath:    configPath,
		profileRoot:   profileRoot,
		masterProcess: masterProcess,
	}

	router := http.NewServeMux()
	router.HandleFunc("/api/login", api.loginUser)
	router.HandleFunc("/api/logout", api.logoutUser)
	router.HandleFunc("/api/profile-names", api.getProfileNames)
	router.HandleFunc("/api/confirm-profile", api.confirmProfile)
	router.HandleFunc("/api/check-login", api.checkLogin)
	router.HandleFunc("/api/process-tree", api.processTree)
	router.HandleFunc("/", api.serveHome)

	api.server = &http.Server{
		Addr:           fmt.Sprintf(":%v", port),
		Handler:        utils.UnrestrictedCors(router),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return api
}

func (api *API) Start() error {
	err := api.Process.Start()
	if err != nil {
		return err
	}
	defer api.Process.Autoclose()

	api.Process.Go(nil, "http.ListenAndServe", func(ctx context.Context) {
		err := api.server.ListenAndServe()
		if err == http.ErrServerClosed {
			return
		} else if err != nil {
			panic(err)
		}
	})
	return nil
}

func (api *API) Close() (err error) {
	api.closeOnce.Do(func() {
		api.Debugf("api close")

		_ = api.server.Shutdown(context.TODO())

		err = api.Process.Close()

		api.app = nil
	})
	return err
}

func (api *API) loginUser(w http.ResponseWriter, r *http.Request) {
	api.appMu.Lock()
	defer api.appMu.Unlock()

	defer r.Body.Close()

	var loginRequest struct {
		Password    string `json:"password"`
		Mnemonic    string `json:"mnemonic"`
		ProfileName string `json:"profileName"`
	}
	err := json.NewDecoder(r.Body).Decode(&loginRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err.Error()), http.StatusInternalServerError)
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

		files, err := ioutil.ReadDir(api.profileRoot)
		if err == nil {
			// Check if profileName is unique
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
	api.app, err = newApp(
		loginRequest.Password,
		loginRequest.Mnemonic,
		api.profileRoot,
		loginRequest.ProfileName,
		api.configPath,
		true,
		api.masterProcess,
	)
	if err != nil {
		api.app = nil
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = api.Process.SpawnChild(context.Background(), api.app)
	if err != nil {
		api.app = nil
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "WORKED!")
}

func (api *API) getProfileNames(w http.ResponseWriter, r *http.Request) {
	api.appMu.Lock()
	defer api.appMu.Unlock()

	defer r.Body.Close()

	var reqResponse struct {
		ProfileNames []string `json:"profileNames"`
	}

	files, err := ioutil.ReadDir(api.profileRoot)
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

func (api *API) confirmProfile(w http.ResponseWriter, r *http.Request) {
	api.appMu.Lock()
	defer api.appMu.Unlock()

	defer r.Body.Close()

	var reqResponse struct {
		ProfileName string `json:"profileName"`
	}

	err := json.NewDecoder(r.Body).Decode(&reqResponse)
	if err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(api.profileRoot)
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

func (api *API) checkLogin(w http.ResponseWriter, r *http.Request) {
	api.appMu.Lock()
	defer api.appMu.Unlock()

	defer r.Body.Close()

	if api.app != nil {
		fmt.Fprintf(w, strconv.FormatBool(api.app.started))
	} else {
		fmt.Fprintf(w, strconv.FormatBool(false))
	}
}

func (api *API) logoutUser(w http.ResponseWriter, r *http.Request) {
	api.app.Close()
	api.app = nil
	w.WriteHeader(200)
}

func (api *API) serveHome(w http.ResponseWriter, r *http.Request) {
	api.appMu.Lock()
	defer api.appMu.Unlock()

	if r.URL.Path == "" || r.URL.Path == "/" {
		w.Header().Add("Content-Type", "text/html")
	} else {
		switch filepath.Ext(r.URL.Path) {
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
	}

	path := r.URL.Path
	if path == "" || path == "/" {
		path = "index.html"
	}

	file, err := staticAssets.Open(filepath.Join("frontend", "build", path))
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("http://localhost:%v/index.html", api.port), http.StatusFound)
		return
	}

	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (api *API) processTree(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(utils.PrettyJSON(api.masterProcess.ProcessTree())))
}
