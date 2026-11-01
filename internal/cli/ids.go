package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func parsePositiveID(flagName, raw string) (uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, usageErr("%s is required", flagName)
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, usageErr("%s must be a positive base-10 integer", flagName)
	}
	if n == 0 {
		return 0, usageErr("%s must be greater than 0", flagName)
	}
	return n, nil
}

func apiPathSegment(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func projectPathFromID(projectID uint64, suffix string) (string, error) {
	p := fmt.Sprintf("/api/projects/%s%s", apiPathSegment(projectID), suffix)
	return normalizeAPIPath(p)
}

func boardPathFromID(boardID uint64, suffix string) (string, error) {
	p := fmt.Sprintf("/api/boards/%s%s", apiPathSegment(boardID), suffix)
	return normalizeAPIPath(p)
}

func cardPathFromID(cardID uint64, suffix string) (string, error) {
	p := fmt.Sprintf("/api/cards/%s%s", apiPathSegment(cardID), suffix)
	return normalizeAPIPath(p)
}

func listCardsPath(listID uint64) (string, error) {
	p := fmt.Sprintf("/api/lists/%s/cards", apiPathSegment(listID))
	return normalizeAPIPath(p)
}
