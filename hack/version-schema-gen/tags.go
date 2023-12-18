package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

// TalosVersion is a struct that holds the Talos version and list of available Talos System Extensions.
type TalosVersion struct {
	Version          string   `json:"version"`
	SystemExtensions []string `json:"systemExtensions"`
}

// TalosVersionTags is a struct that holds the list of TalosVersionTags for each Talos version returned by the registry.
type TalosVersionTags struct {
	Versions []TalosVersion `json:"versions"`
}

// Implement Contains on TalosVersionsTags.Versions
func (v TalosVersionTags) Contains(s string) bool {
	for _, a := range v.Versions {
		if a.Version == s {
			return true
		}
	}

	return false
}

// Implement Sort interface on TalosVersionsTags.Versions
func (v TalosVersionTags) Len() int {
	return len(v.Versions)
}
func (v TalosVersionTags) Less(i, j int) bool {
	return semver.Compare(v.Versions[i].Version, v.Versions[j].Version) < 0
}
func (v TalosVersionTags) Swap(i, j int) {
	v.Versions[i], v.Versions[j] = v.Versions[j], v.Versions[i]
}

// getMissingTags fetches all tags for a given repository and return a TalosVersionsTags struct of all tags not already in the cache or an error.
func getMissingTags(cachedTags TalosVersionTags) (TalosVersionTags, error) {
	tagsToAppend := TalosVersionTags{}

	// Fetch the tags
	log.Debugf("calling registry docker://%s...", TSEHelperTalosExtensionsRepository)
	upstreamTags, err := crane.ListTags(TSEHelperTalosExtensionsRepository)
	if err != nil {
		return cachedTags, err
	}

	// Loop through the tags
	for _, tag := range upstreamTags {
		// Skip anything that doesn't start with v
		if !strings.HasPrefix(tag, "v") {
			log.Tracef("skipping tag %s", tag)
			continue
		}
		// Skip any tag that's already present
		if cachedTags.Contains(tag) {
			continue
		}

		// Add any new tags to the list
		log.Debugf("adding new tag %s", tag)
		tagsToAppend.Versions = append(tagsToAppend.Versions, TalosVersion{Version: tag})
	}

	// Sort the list
	log.Debugf("finalizing list of tags to append: %s", tagsToAppend)
	sort.Sort(tagsToAppend)

	return tagsToAppend, nil
}

// getMissingVersions checks if a newer version is available and returns a TalosVersionTags struct of all missing versions.
func getMissingVersions(versionsTags *TalosVersionTags) TalosVersionTags {
	// Check if the cache file exists
	if !checkCache() {
		// Load the cache file
		loadCache(versionsTags)
	}

	// Fetch the missing tags
	tags, err := getMissingTags(*versionsTags)
	if err != nil {
		fmt.Printf("Error fetching tags: %s\n", err)
		os.Exit(1)
	}

	return tags
}

// cleanString takes a string and returns a cleaned string based on the flags passed.
func cleanString(line string) string {
	// Log all flags
	log.Tracef("minimal: %t", minimal)

	log.Tracef("trimRegistry: %t", trimRegistry)
	log.Tracef("trimSha256: %t", trimSha256)
	log.Tracef("trimTag: %t", trimTag)

	// Create a new regexp from the TalHelperTalosExtensionsRegex
	regexp := regexp.MustCompile(TSEHelperTalosExtensionsRegex)

	// Find the sub-matches
	matches := regexp.FindStringSubmatch(line)
	log.Tracef("regexp matches: %s", matches)

	// Map results to capture group names
	result := make(map[string]string)
	for i, name := range regexp.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}

	// If all flags are set or minimal is set, return the minimal json
	if (trimRegistry && trimSha256 && trimTag) || minimal {
		log.Tracef("returning minimal json")
		return fmt.Sprintf(result["org"] + "/" + result["repo"])
	}

	if trimRegistry && trimSha256 {
		log.Tracef("returning trimmed registry and sha256")
		return fmt.Sprintf(result["org"] + "/" + result["repo"] + ":" + result["tag"])
	} else if trimRegistry && !trimSha256 {
		if trimTag {
			log.Tracef("returning trimmed registry and tag")
			return fmt.Sprintf(result["org"] + "/" + result["repo"] + "@sha256:" + result["shasum"])
		}
		log.Tracef("returning trimmed registry")
		return fmt.Sprintf(result["org"] + "/" + result["repo"] + ":" + result["tag"] + "@sha256:" + result["shasum"])
	} else if !trimRegistry && trimSha256 {
		if trimTag {
			log.Tracef("returning trimmed sha256 and tag")
			return fmt.Sprintf(result["registry"] + "/" + result["org"] + "/" + result["repo"])
		}
		log.Tracef("returning trimmed sha256")
		return fmt.Sprintf(result["registry"] + "/" + result["org"] + "/" + result["repo"] + ":" + result["tag"])
	} else {
		if trimTag {
			log.Tracef("returning trimmed tag")
			return fmt.Sprintf(result["registry"] + "/" + result["org"] + "/" + result["repo"] + "@sha256:" + result["shasum"])
		}
		log.Tracef("returning full string")
		return fmt.Sprintf(result["registry"] + "/" + result["org"] + "/" + result["repo"] + ":" + result["tag"] + "@sha256:" + result["shasum"])
	}
}
