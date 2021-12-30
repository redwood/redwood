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
	defer fmt.Fprintln(w)

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
	logf("HEADS %v", heads)

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
	refsList = append(refsList, "@refs/heads/master HEAD")

	for _, ref := range refsList {
		fmt.Fprintln(w, ref)
	}

	return nil
}
