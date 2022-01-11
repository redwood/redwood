package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	git "github.com/brynbellomy/git2go"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/braidhttp"
)

var sigkeys = func() *crypto.SigKeypair {
	sigkeys, err := crypto.GenerateSigKeypair()
	if err != nil {
		panic(err)
	}
	return sigkeys
}()

var StateURI string
var RootKeypath state.Keypath
var repo *git.Repository

type M = map[string]interface{}

func main() {
	var GIT_DIR = os.Getenv("GIT_DIR")

	if GIT_DIR == "" {
		die(errors.New("empty GIT_DIR"))
	}

	re := regexp.MustCompile(`redwood://(?:([^@]*)@)?(.*)`)
	matches := re.FindStringSubmatch(os.Args[2])
	host := matches[1]
	stateURIAndPath := strings.Split(matches[2], "/")

	StateURI = strings.Join(stateURIAndPath[:2], "/")
	RootKeypath = state.Keypath(strings.Join(stateURIAndPath[2:], string(state.KeypathSeparator)))

	gitDir, err := filepath.Abs(filepath.Dir(GIT_DIR))
	if err != nil {
		die(err)
	}

	repo, err = git.OpenRepository(gitDir)
	if err != nil {
		die(err)
	}

	client, err := braidhttp.NewLightClient("http://"+host, sigkeys, nil, false)
	if err != nil {
		die(err)
	}

	err = client.Authorize()
	if err != nil {
		die(err)
	}
	logf("redwood: authorized with " + host)

	err = speakGit(os.Stdin, os.Stdout, client)
	if err != nil {
		die(err)
	}
}

func speakGit(r io.Reader, w io.Writer, client *braidhttp.LightClient) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		text = strings.TrimSpace(text)
		logf("[git] %v", text)

		switch {

		case strings.HasPrefix(text, "capabilities"):
			sayln(w, "list")
			sayln(w, "fetch")
			sayln(w, "push")
			sayln(w)

		case strings.HasPrefix(text, "list"):
			//forPush := strings.Contains(text, "for-push")
			err := listRefs(w, client)
			if err != nil {
				return err
			}

		case strings.HasPrefix(text, "fetch"):
			fetchArgs := strings.Split(text, " ")
			commitHash := fetchArgs[1]
			err := fetch(client, commitHash)
			if err != nil {
				return err
			}

			sayln(w)

		case strings.HasPrefix(text, "push"):
			for scanner.Scan() {
				pushSplit := strings.Split(text, " ")
				if len(pushSplit) < 2 {
					return errors.Errorf("malformed 'push' command. %q", text)
				}
				srcDstSplit := strings.Split(pushSplit[1], ":")
				if len(srcDstSplit) < 2 {
					return errors.Errorf("malformed 'push' command. %q", text)
				}
				src, dst := srcDstSplit[0], srcDstSplit[1]
				err := push(src, dst, client)
				if err != nil {
					return err
				}
				text = scanner.Text()
				if text == "" {
					break
				}
			}
			sayln(w)

		case text == "":
			// The blank line is the stream terminator.  We return when we see this.
			if runtime.GOOS == "windows" {
				return nil
			}

		default:
			return fmt.Errorf("unknown git speak: %v", text)
		}
	}
	return scanner.Err()
}
