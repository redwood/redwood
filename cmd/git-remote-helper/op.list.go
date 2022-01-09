package main

import (
	"encoding/json"
	"fmt"
	"io"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/braidhttp"
)

func listRefs(w io.Writer, client *braidhttp.LightClient) error {
	defer sayln(w)

	stateReader, _, _, err := client.Get(StateURI, nil, RootKeypath.Push(state.Keypath("refs/heads")), nil, true)
	if errors.Cause(err) == errors.Err404 {
		logf("ERR: %v", err)
		return nil
	} else if err != nil {
		logf("ERR: %v", err)
		return err
	}
	defer stateReader.Close()

	var heads map[string]map[string]interface{}
	err = json.NewDecoder(stateReader).Decode(&heads)
	if err != nil {
		return err
	}

	var refsList []string
	for refName, refObject := range heads {
		maybeCommitHash, exists := refObject["HEAD"]
		if !exists {
			logf("no commit for ref %v", refName)
			continue
		}
		commitHash, is := maybeCommitHash.(string)
		if !is {
			logf("no commit for ref %v", refName)
			continue
		}
		refsList = append(refsList, fmt.Sprintf("%s %s", commitHash, "refs/heads/"+refName))
	}

	var headSymlink string
	if _, exists := heads["master"]; exists {
		headSymlink = "master"
	} else if _, exists = heads["main"]; exists {
		headSymlink = "main"
	} else {
		for k := range heads {
			headSymlink = k
			break
		}
	}

	refsList = append(refsList, fmt.Sprintf("@refs/heads/%v HEAD", headSymlink))

	for _, ref := range refsList {
		sayln(w, ref)
	}

	return nil
}
