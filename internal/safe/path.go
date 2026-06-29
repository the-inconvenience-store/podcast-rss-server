package safe

import (
	"fmt"
	"regexp"
)

var pathPartPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func PathPart(value string) (string, error) {
	if !pathPartPattern.MatchString(value) || value == "." || value == ".." {
		return "", fmt.Errorf("unsafe path value")
	}
	return value, nil
}

func MediaObjectKey(showID, episodeID, filename string) (string, error) {
	showID, err := PathPart(showID)
	if err != nil {
		return "", err
	}
	episodeID, err = PathPart(episodeID)
	if err != nil {
		return "", err
	}
	filename, err = PathPart(filename)
	if err != nil {
		return "", err
	}
	return "media/" + showID + "/" + episodeID + "/" + filename, nil
}
